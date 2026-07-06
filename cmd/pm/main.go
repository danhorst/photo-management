// Command pm organizes photos into a YYYY/MM library, skipping
// content duplicates using a BLAKE3 hash index, so Capture One only ever has to
// synchronize genuinely-new files.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/exif"
	"github.com/dbh/photo-management/internal/hash"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/media"
	"github.com/dbh/photo-management/internal/organize"
	"github.com/dbh/photo-management/internal/volume"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `pm — fast, deduplicating photo importer

Usage:
  pm <source> [flags]   Import media from a card or queue directory
  pm export [flags]     Generate presentation HEICs into Export/
  pm publish [flags]    Import exported HEICs into Apple Photos
  pm sync [flags]       Export new derivatives, then publish them, using a stored watermark
  pm reconcile [flags]  Clear published state for assets no longer in Photos
  pm link [flags]       Link natively-imported Photos assets back into the index by filename
  pm pull [flags]       Pull iPhone photos from Apple Photos into the archive
  pm recanon [flags]    Rename non-canonical archive files to a reduced-precision name
  pm index [flags]      Build/refresh the content-hash index
  pm stats [flags]      Show index location and size
  pm config <cmd>       Read/write the config file (see below)
  pm media <cmd>        Manage the skip cache (see below)
  pm version            Print the version

Config commands:
  config path                     Print the config file location
  config show                     Print the effective library and database
  config init                     Write a default config file
  config get <library|database>   Print one effective value
  config set <library|database> <value>
                                  Set a value, creating the file if needed

Media commands:
  media list                      List cached volumes in the skip cache
  media clear [<id>…]             Clear volumes by id or prefix; no args
                                  on a terminal opens a multiselect

Flags:
  -L, --library DIR   Photo library root (overrides config and default)
      --db FILE       Index database path (overrides config and default)
      --debug         Print a detailed activity log
      --dry-run       Import/export/publish/sync/pull/recanon/reconcile/link: report actions without writing anything
      --since DATE    Export/publish/pull: limit to frames captured on/after YYYY-MM-DD
                      Sync: override the stored watermark for this run only
      --set-since DATE
                      Sync: set the stored watermark directly (YYYY-MM-DD)
                      without running export or publish, then exit
      --batch-size N  Publish/sync: files per osxphotos import call (0 = one batch)
      --settle DUR    Publish/sync: pause between batches so Photos drains (e.g. 2s)
      --stage DIR     Publish: hardlink derivatives into DIR/YYYY/MM for native import instead of importing
      --match SUBSTR  Recanon: limit to frames whose stem contains SUBSTR
      --date DATE     Recanon: stamp day precision YYYY-MM-DD instead of month-from-folder
      --photos-library PATH
                      Publish/sync/pull: target this Photos library instead of
                      whatever's open (see README for the import caveat)
`

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(2)
	}

	var err error
	switch args[0] {
	case "export":
		err = cmdExport(args[1:])
	case "publish":
		err = cmdPublish(args[1:])
	case "sync":
		err = cmdSync(args[1:])
	case "reconcile":
		err = cmdReconcile(args[1:])
	case "link":
		err = cmdLink(args[1:])
	case "pull":
		err = cmdPull(args[1:])
	case "recanon":
		err = cmdRecanon(args[1:])
	case "index":
		err = cmdIndex(args[1:])
	case "stats":
		err = cmdStats(args[1:])
	case "config":
		err = cmdConfig(args[1:])
	case "media":
		err = cmdMedia(args[1:])
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		err = cmdImport(args)
	}
	if err != nil {
		log.Fatalf("pm: %v", err)
	}
}

// commonFlags registers the flags shared by every subcommand.
func commonFlags(fs *flag.FlagSet) (lib, db *string, debug *bool) {
	lib = fs.String("library", "", "photo library root")
	fs.StringVar(lib, "L", "", "photo library root (shorthand)")
	db = fs.String("db", "", "index database path")
	debug = fs.Bool("debug", false, "print a detailed activity log")
	return lib, db, debug
}

// parseArgs parses flags that may appear before or after positional arguments,
// which the stdlib flag package does not handle on its own (it stops at the
// first positional). It returns the collected positionals.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	for fs.NArg() > 0 {
		positionals = append(positionals, fs.Arg(0))
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return nil, err
		}
	}
	return positionals, nil
}

func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report actions without moving files")
	fs.Usage = func() { fmt.Print(usage) }
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) != 1 {
		fmt.Print(usage)
		os.Exit(2)
	}
	source := positionals[0]

	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	return runImport(cfg, idx, source, *dryRun, *debug, true)
}

// runImport pulls the media under source into the library: BLAKE3 dedup,
// capture-date organization, move-or-copy. With useVolume the source is
// treated as a removable card — stamped with a marker and tracked in the
// per-volume skip cache; without it (a scratch queue directory) only the
// content-hash dedup applies.
func runImport(cfg config.Config, idx *index.Index, source string, dryRun, debug, useVolume bool) error {
	start := time.Now()

	logf := debugLogger(debug)
	showProgress := !debug && isatty.IsTerminal(os.Stderr.Fd())

	paths, err := collectMedia(source)
	if err != nil {
		return err
	}
	logf("found %d media file(s) under %s", len(paths), source)

	var root, volID string
	seen := map[string]index.MediaRecord{}
	if useVolume {
		var marker volume.Marker
		var existed bool
		root, volID, marker, existed, err = volume.Resolve(source)
		if err != nil {
			return fmt.Errorf("identifying volume: %w", err)
		}
		if !existed && !dryRun {
			if err := volume.Stamp(root, marker); err != nil {
				return fmt.Errorf("stamping volume: %w", err)
			}
		}
		// Register only when there is media to record, so an empty card cannot leave
		// a volumes row with no media_files (invisible to media list/clear).
		if !dryRun && len(paths) > 0 {
			if err := idx.PutVolume(volID, filepath.Base(root)); err != nil {
				return err
			}
		}
		logf("volume %s at %s", volID, root)

		seen, err = idx.VolumeMedia(volID)
		if err != nil {
			return err
		}
	}

	daemon, err := exif.NewDaemon()
	if err != nil {
		return fmt.Errorf("starting exiftool: %w", err)
	}
	defer daemon.Close()

	var importBar *progressbar.ProgressBar
	if showProgress && len(paths) > 0 {
		importBar = progressbar.Default(int64(len(paths)), "importing")
	}

	var imported, dups, skipped, unsorted int
	touched := map[string]bool{}

	for _, src := range paths {
		if importBar != nil {
			importBar.Add(1)
		}
		fi, err := os.Stat(src)
		if err != nil {
			logf("skip %s: %v", src, err)
			continue
		}
		rel, err := filepath.Rel(root, src)
		if err != nil {
			rel = src
		}
		if rec, ok := seen[rel]; ok && rec.Size == fi.Size() && rec.Mtime == fi.ModTime().Unix() {
			skipped++
			logf("seen %s (already processed from this card)", src)
			continue
		}
		h, err := hash.File(src)
		if err != nil {
			logf("skip %s: %v", src, err)
			continue
		}

		if existing, found, err := idx.Lookup(h); err != nil {
			return err
		} else if found {
			dups++
			logf("dup  %s == %s", src, existing)
			if useVolume && !dryRun {
				if err := idx.PutMedia(volID, rel, fi.Size(), fi.ModTime().Unix()); err != nil {
					return err
				}
			}
			continue
		}

		t, dated := daemon.Date(src)
		if !dated {
			t = fi.ModTime()
		}

		var dst string
		if dated {
			dst = organize.Dest(cfg.Library, t, filepath.Base(src))
		} else {
			// No reliable capture date: file under Unsorted for manual review.
			dst = filepath.Join(cfg.Library, "Unsorted", filepath.Base(src))
		}

		dst, isDup, err := resolveCollision(dst, h)
		if err != nil {
			return err
		}
		if isDup {
			dups++
			logf("dup  %s already in library as %s", src, dst)
			if !dryRun {
				if err := idx.Put(dst, fi.Size(), fi.ModTime().Unix(), h); err != nil {
					return err
				}
				if useVolume {
					if err := idx.PutMedia(volID, rel, fi.Size(), fi.ModTime().Unix()); err != nil {
						return err
					}
				}
			}
			continue
		}

		if dryRun {
			logf("would import %s -> %s", src, dst)
		} else {
			moved, err := organize.Place(src, dst)
			if err != nil {
				return fmt.Errorf("placing %s: %w", src, err)
			}
			if err := idx.Put(dst, fi.Size(), fi.ModTime().Unix(), h); err != nil {
				return err
			}
			if useVolume {
				if err := idx.PutMedia(volID, rel, fi.Size(), fi.ModTime().Unix()); err != nil {
					return err
				}
			}
			verb := "copied"
			if moved {
				verb = "moved"
			}
			logf("%s %s -> %s", verb, src, dst)
		}

		if !dated {
			unsorted++
		}
		imported++
		touched[filepath.Dir(dst)] = true
	}

	if importBar != nil {
		importBar.Finish()
		fmt.Fprintln(os.Stderr)
	}
	printSummary(cfg.Library, imported, dups, skipped, unsorted, touched, dryRun, time.Since(start))
	return nil
}

// resolveCollision finds an available destination path. If a file already exists
// at dst with identical content, it returns isDup=true. If the bytes differ, it
// appends a numeric suffix until a free path is found.
func resolveCollision(dst, srcHash string) (path string, isDup bool, err error) {
	ext := filepath.Ext(dst)
	stem := dst[:len(dst)-len(ext)]
	for n := 0; ; n++ {
		candidate := dst
		if n > 0 {
			candidate = fmt.Sprintf("%s-%02d%s", stem, n, ext)
		}
		if _, statErr := os.Stat(candidate); os.IsNotExist(statErr) {
			return candidate, false, nil
		} else if statErr != nil {
			return "", false, statErr
		}
		existing, hErr := hash.File(candidate)
		if hErr != nil {
			return "", false, hErr
		}
		if existing == srcHash {
			return candidate, true, nil
		}
	}
}

func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	return buildIndex(cfg, idx, *debug)
}

type job struct {
	path  string
	size  int64
	mtime int64
}

func buildIndex(cfg config.Config, idx *index.Index, debug bool) error {
	logf := debugLogger(debug)
	// A live bar conflicts with --debug's per-file logging, and only makes sense
	// on a terminal.
	showProgress := !debug && isatty.IsTerminal(os.Stderr.Fd())

	var scanBar *progressbar.ProgressBar
	if showProgress {
		scanBar = progressbar.Default(-1, "scanning")
	}

	var jobs []job
	var cached int
	walkErr := filepath.WalkDir(cfg.Library, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			logf("walk %s: %v", p, err)
			return nil
		}
		if d.IsDir() {
			if media.IsExcludedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if p == cfg.Database || !media.IsMedia(d.Name()) {
			return nil
		}
		if scanBar != nil {
			scanBar.Add(1)
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size, mtime := info.Size(), info.ModTime().Unix()
		if _, ok := idx.Cached(p, size, mtime); ok {
			cached++
			return nil
		}
		jobs = append(jobs, job{p, size, mtime})
		return nil
	})
	if scanBar != nil {
		scanBar.Finish()
		fmt.Fprintln(os.Stderr)
	}
	if walkErr != nil {
		return walkErr
	}

	type result struct {
		job  job
		hash string
		err  error
	}
	jobCh := make(chan job)
	resCh := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				h, err := hash.File(j.path)
				resCh <- result{j, h, err}
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			jobCh <- j
		}
		close(jobCh)
		wg.Wait()
		close(resCh)
	}()

	var hashBar *progressbar.ProgressBar
	if showProgress && len(jobs) > 0 {
		hashBar = progressbar.Default(int64(len(jobs)), "indexing")
	}

	batch, err := idx.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			batch.Rollback()
		}
	}()

	var hashed, failed int
	for r := range resCh {
		if hashBar != nil {
			hashBar.Add(1)
		}
		if r.err != nil {
			failed++
			logf("hash %s: %v", r.job.path, r.err)
			continue
		}
		if err := batch.Put(r.job.path, r.job.size, r.job.mtime, r.hash); err != nil {
			return err
		}
		hashed++
		logf("indexed %s", r.job.path)
	}
	if err := batch.Commit(); err != nil {
		return err
	}
	committed = true
	if hashBar != nil {
		hashBar.Finish()
		fmt.Fprintln(os.Stderr)
	}

	total, _, err := idx.Stats()
	if err != nil {
		return err
	}
	fmt.Printf("Indexed %d new/changed file(s); %d unchanged; %d error(s); %d total in index\n",
		hashed, cached, failed, total)
	return nil
}

func cmdStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	lib, db, _ := commonFlags(fs)
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	count, last, err := idx.Stats()
	if err != nil {
		return err
	}
	fmt.Printf("Library:  %s\n", cfg.Library)
	fmt.Printf("Database: %s\n", cfg.Database)
	fmt.Printf("Indexed:  %d file(s)\n", count)
	if last.IsZero() {
		fmt.Println("Last run: never")
	} else {
		fmt.Printf("Last run: %s\n", last.Format(time.RFC3339))
	}
	return nil
}

func cmdConfig(args []string) error {
	if len(args) == 0 {
		return configShow()
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "path":
		fmt.Println(config.Path())
		return nil
	case "show":
		return configShow()
	case "init":
		p := config.Path()
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("config already exists at %s (use 'config set' to change values)", p)
		}
		if err := config.WriteDefault(); err != nil {
			return err
		}
		fmt.Printf("wrote default config to %s\n", p)
		return nil
	case "get":
		if len(rest) != 1 {
			return fmt.Errorf("usage: pm config get <library|database>")
		}
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}
		v, err := cfg.Get(rest[0])
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	case "set":
		if len(rest) != 2 {
			return fmt.Errorf("usage: pm config set <library|database> <value>")
		}
		cfg, err := config.LoadFile()
		if err != nil {
			return err
		}
		if err := cfg.Set(rest[0], rest[1]); err != nil {
			return err
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("set %s = %s in %s\n", rest[0], rest[1], config.Path())
		return nil
	default:
		return fmt.Errorf("unknown config command %q (want path, show, init, get, set)", sub)
	}
}

func configShow() error {
	cfg, err := config.Load("", "")
	if err != nil {
		return err
	}
	fmt.Printf("library  = %s\n", cfg.Library)
	fmt.Printf("database = %s\n", cfg.Database)
	return nil
}

// collectMedia returns the managed media files under root, recursively.
func collectMedia(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if media.IsExcludedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if media.IsMedia(d.Name()) {
			paths = append(paths, p)
		}
		return nil
	})
	return paths, err
}

func printSummary(library string, imported, dups, skipped, unsorted int, touched map[string]bool, dryRun bool, elapsed time.Duration) {
	verb := "Imported"
	if dryRun {
		verb = "Would import"
	}
	fmt.Printf("\n%s %d file(s) in %s; skipped %d duplicate(s)", verb, imported, elapsed.Round(time.Millisecond), dups)
	if skipped > 0 {
		fmt.Printf(", %d already processed from this card", skipped)
	}
	if unsorted > 0 {
		fmt.Printf("; %d undated file(s) went to Unsorted/", unsorted)
	}
	fmt.Println(".")

	if len(touched) == 0 {
		return
	}
	dirs := make([]string, 0, len(touched))
	for d := range touched {
		if rel, err := filepath.Rel(library, d); err == nil {
			d = rel
		}
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	fmt.Println("\nSynchronize these folders in Capture One:")
	for _, d := range dirs {
		fmt.Printf("  %s\n", d)
	}
}

func debugLogger(debug bool) func(string, ...any) {
	if !debug {
		return func(string, ...any) {}
	}
	return func(format string, a ...any) { log.Printf(format, a...) }
}

func cmdMedia(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pm media <list|clear>")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdMediaList(rest)
	case "clear":
		return cmdMediaClear(rest)
	default:
		return fmt.Errorf("unknown media command %q (want list, clear)", sub)
	}
}

func cmdMediaList(args []string) error {
	fs := flag.NewFlagSet("media list", flag.ExitOnError)
	lib, db, _ := commonFlags(fs)
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	vols, err := idx.Volumes()
	if err != nil {
		return err
	}
	if len(vols) == 0 {
		fmt.Println("no cached volumes")
		return nil
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("NAME", "ID", "FILES", "LAST SEEN").
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	for _, v := range vols {
		lastSeen := "—"
		if !v.LastSeen.IsZero() {
			lastSeen = v.LastSeen.Format("2006-01-02")
		}
		t.Row(displayName(v), v.VolumeID, fmt.Sprintf("%d", v.FileCount), lastSeen)
	}
	fmt.Println(t)
	return nil
}

func cmdMediaClear(args []string) error {
	fs := flag.NewFlagSet("media clear", flag.ExitOnError)
	lib, db, _ := commonFlags(fs)
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(*lib, *db)
	if err != nil {
		return err
	}
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	vols, err := idx.Volumes()
	if err != nil {
		return err
	}

	if len(positionals) == 0 {
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			return fmt.Errorf("no volume id given — run 'pm media list' to see ids, or run interactively on a terminal")
		}
		return cmdMediaClearInteractive(idx, vols)
	}

	for _, sel := range positionals {
		id, err := resolveVolume(sel, vols)
		if err != nil {
			return err
		}
		n, err := idx.ClearVolume(id)
		if err != nil {
			return err
		}
		fmt.Printf("cleared %d file(s) from volume %s\n", n, id)
	}
	return nil
}

func cmdMediaClearInteractive(idx *index.Index, vols []index.VolumeInfo) error {
	if len(vols) == 0 {
		fmt.Println("no cached volumes")
		return nil
	}
	opts := make([]huh.Option[string], len(vols))
	for i, v := range vols {
		label := fmt.Sprintf("%-20s %s  %d files", displayName(v), v.VolumeID, v.FileCount)
		opts[i] = huh.NewOption(label, v.VolumeID)
	}
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select volumes to clear").
				Options(opts...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	for _, id := range selected {
		n, err := idx.ClearVolume(id)
		if err != nil {
			return err
		}
		fmt.Printf("cleared %d file(s) from volume %s\n", n, id)
	}
	return nil
}

// displayName returns the volume's label, or a placeholder for caches created
// before naming.
func displayName(v index.VolumeInfo) string {
	if v.Label == "" {
		return "(unknown)"
	}
	return v.Label
}

// resolveVolume maps selector to a single volume id by exact match or
// unambiguous prefix. Errors on ambiguous prefix or label-only input.
func resolveVolume(selector string, vols []index.VolumeInfo) (string, error) {
	for _, v := range vols {
		if v.VolumeID == selector {
			return v.VolumeID, nil
		}
	}
	var matches []string
	for _, v := range vols {
		if strings.HasPrefix(v.VolumeID, selector) {
			matches = append(matches, v.VolumeID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no cached volume id starts with %q — run 'pm media list' to see ids", selector)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("prefix %q is ambiguous (%d matches) — use a longer prefix or the full id", selector, len(matches))
	}
}
