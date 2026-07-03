package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/export"
	"github.com/dbh/photo-management/internal/hash"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/media"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report derivatives without writing files or rows")
	since := fs.String("since", "", "limit to frames captured on/after this date (YYYY-MM-DD)")
	fs.Usage = func() { fmt.Print(usage) }
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}

	var sinceDate time.Time
	if *since != "" {
		t, err := time.ParseInLocation("2006-01-02", *since, time.Local)
		if err != nil {
			return fmt.Errorf("--since must be YYYY-MM-DD, got %q", *since)
		}
		sinceDate = t
	}

	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	longEdge, quality := cfg.ExportLongEdge, cfg.ExportQuality
	if longEdge == 0 {
		longEdge = export.DefaultLongEdge
	}
	if quality == 0 {
		quality = export.DefaultQuality
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	logf := debugLogger(*debug)
	start := time.Now()

	paths, err := collectArchive(cfg.Library)
	if err != nil {
		return err
	}
	frames := export.Group(paths)
	logf("found %d frame(s) in %d archive file(s)", len(frames), len(paths))

	type work struct {
		frame export.Frame
		src   export.Source
	}
	var jobs []work
	var malformed int
	for _, f := range frames {
		d, ok := f.CaptureDate()
		if !ok {
			malformed++
			logf("skip %s: stem is not a canonical capture-date name", f.Stem)
			continue
		}
		if !sinceDate.IsZero() && d.Before(sinceDate) {
			continue
		}
		for _, s := range f.Sources() {
			jobs = append(jobs, work{f, s})
		}
	}
	if malformed > 0 {
		fmt.Printf("skipped %d frame(s) with a non-canonical name (see --debug)\n", malformed)
	}

	gen := &export.Generator{LongEdge: longEdge, Quality: quality}
	defer gen.Close()

	// Scan phase: resolve each job to its source content hash and drop the ones
	// whose derivative already exists. Hashing is parallel and reuses the shared
	// files index (idx.Cached), so an unchanged file is never re-read — whether
	// it was hashed by a prior `index`, a prior `export`, or earlier this run.
	// Only DB reads happen here; the memoizing writes are deferred until after
	// the workers finish, since index.Open caps the db at one connection and an
	// open write transaction would block the concurrent reads.
	type genJob struct {
		w   work
		h   string
		dst string
	}
	type scanResult struct {
		w           work
		hash        string
		size, mtime int64
		fresh       bool // hashed this run (not from cache) -> memoize
		err         error
	}

	showScan := !*debug && isatty.IsTerminal(os.Stderr.Fd())
	var scanBar *progressbar.ProgressBar
	if showScan && len(jobs) > 0 {
		scanBar = progressbar.Default(int64(len(jobs)), "scanning")
	}

	scanCh := make(chan work)
	scanResCh := make(chan scanResult)
	var scanWG sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		scanWG.Add(1)
		go func() {
			defer scanWG.Done()
			for w := range scanCh {
				r := scanResult{w: w}
				info, err := os.Stat(w.src.Path)
				if err != nil {
					r.err = err
					scanResCh <- r
					continue
				}
				r.size, r.mtime = info.Size(), info.ModTime().Unix()
				if h, ok := idx.Cached(w.src.Path, r.size, r.mtime); ok {
					r.hash = h
				} else if h, err := hash.File(w.src.Path); err != nil {
					r.err = err
				} else {
					r.hash, r.fresh = h, true
				}
				scanResCh <- r
			}
		}()
	}
	go func() {
		for _, w := range jobs {
			scanCh <- w
		}
		close(scanCh)
		scanWG.Wait()
		close(scanResCh)
	}()

	type freshFile struct {
		path        string
		size, mtime int64
		hash        string
	}
	var toGenerate []genJob
	var fresh []freshFile
	var generated, skipped, failed int
	for r := range scanResCh {
		if scanBar != nil {
			scanBar.Add(1)
		}
		if r.err != nil {
			failed++
			logf("hash %s: %v", r.w.src.Path, r.err)
			continue
		}
		if r.fresh {
			fresh = append(fresh, freshFile{r.w.src.Path, r.size, r.mtime, r.hash})
		}
		has, err := idx.HasDerivative(r.hash)
		if err != nil {
			return err
		}
		if has {
			skipped++
			logf("skip %s (already generated)", r.w.src.Path)
			continue
		}
		dst := export.DestPath(cfg.Library, r.w.frame, r.w.src)
		if *dryRun {
			generated++
			logf("would export %s -> %s", r.w.src.Path, dst)
			continue
		}
		toGenerate = append(toGenerate, genJob{r.w, r.hash, dst})
	}
	if scanBar != nil {
		scanBar.Finish()
		fmt.Fprintln(os.Stderr)
	}

	// Memoize freshly hashed files into the shared index so a later run skips
	// the read. Safe now: the scan workers have finished, so this write
	// transaction holds the single connection uncontended. Skipped under
	// --dry-run, which writes nothing.
	if !*dryRun && len(fresh) > 0 {
		fb, err := idx.Begin()
		if err != nil {
			return err
		}
		for _, f := range fresh {
			if err := fb.Put(f.path, f.size, f.mtime, f.hash); err != nil {
				fb.Rollback()
				return err
			}
		}
		if err := fb.Commit(); err != nil {
			return err
		}
	}

	// Generate phase: sips+exiftool per job, parallelized since each is a
	// subprocess spawn dominated by wait time. Writes funnel through the
	// batch on the main goroutine only; workers touch no shared state besides
	// the Generator (safe for concurrent Generate calls).
	var exportBar *progressbar.ProgressBar
	if !*dryRun && !*debug && isatty.IsTerminal(os.Stderr.Fd()) && len(toGenerate) > 0 {
		exportBar = progressbar.Default(int64(len(toGenerate)), "exporting")
	}
	if len(toGenerate) > 0 {
		batch, err := idx.BeginDerivatives()
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				batch.Rollback()
			}
		}()

		type genResult struct {
			gj  genJob
			err error
		}
		jobCh := make(chan genJob)
		resCh := make(chan genResult)
		var wg sync.WaitGroup
		for i := 0; i < runtime.NumCPU(); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for gj := range jobCh {
					err := gen.Generate(gj.w.src, gj.w.frame.Stem, gj.h, gj.dst)
					resCh <- genResult{gj, err}
				}
			}()
		}
		go func() {
			for _, gj := range toGenerate {
				jobCh <- gj
			}
			close(jobCh)
			wg.Wait()
			close(resCh)
		}()

		for r := range resCh {
			if exportBar != nil {
				exportBar.Add(1)
			}
			if r.err != nil {
				failed++
				logf("export %s: %v", r.gj.w.src.Path, r.err)
				continue
			}
			if err := batch.Put(r.gj.h, r.gj.w.frame.Stem, r.gj.w.src.Kind, r.gj.dst); err != nil {
				return err
			}
			generated++
			logf("exported %s -> %s", r.gj.w.src.Path, r.gj.dst)
		}
		if err := batch.Commit(); err != nil {
			return err
		}
		committed = true
	}
	if exportBar != nil {
		exportBar.Finish()
		fmt.Fprintln(os.Stderr)
	}

	verb := "Exported"
	if *dryRun {
		verb = "Would export"
	}
	fmt.Printf("%s %d HEIC(s) in %s; skipped %d already generated", verb, generated, time.Since(start).Round(time.Millisecond), skipped)
	if failed > 0 {
		fmt.Printf("; %d error(s)", failed)
	}
	fmt.Println(".")
	return nil
}

var yearDir = regexp.MustCompile(`^\d{4}$`)

// collectArchive returns the media files in the library's YYYY/MM tree,
// leaving Export/, Unsorted/, and other non-archive directories alone.
func collectArchive(library string) ([]string, error) {
	var paths []string
	years, err := os.ReadDir(library)
	if err != nil {
		return nil, err
	}
	for _, y := range years {
		if !y.IsDir() || !yearDir.MatchString(y.Name()) {
			continue
		}
		root := filepath.Join(library, y.Name())
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Unsorted/ holds files import couldn't derive a canonical
				// stem for (see main.go's "manual review" bucket); export's
				// frame grouping assumes every archive name is canonical, so
				// these must stay out of the walk even nested under a year.
				if d.Name() == "Unsorted" || media.IsExcludedDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if media.IsMedia(d.Name()) {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}
