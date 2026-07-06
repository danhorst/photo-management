package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

func cmdPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report imports and skips without writing anything")
	photosLib := fs.String("photos-library", "", "target this Photos library instead of whatever's open (see caveat: osxphotos import ignores this — Photos.app must already have it open)")
	since := fs.String("since", "", "limit to derivatives captured on/after this date (YYYY-MM-DD)")
	batchSize := fs.Int("batch-size", defaultBatchSize, "import this many files per osxphotos call (0 = one batch)")
	settle := fs.Duration("settle", defaultSettle, "pause this long between batches so Photos can drain its background import queue")
	stage := fs.String("stage", "", "hardlink the derivatives that would be imported into DIR/YYYY/MM (for native Photos import) instead of importing; run `pm link` afterward")
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
	idx, err := index.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer idx.Close()

	showProgress := !*debug && isatty.IsTerminal(os.Stderr.Fd())
	if *photosLib != "" {
		if showProgress {
			fmt.Fprintf(os.Stderr, "verifying Photos.app has %s open (two library queries)…\n", *photosLib)
		}
		if err := verifyActiveLibrary(*photosLib); err != nil {
			return err
		}
	}

	return publish(idx, photos.OSXPhotos{PhotosLibrary: *photosLib}, debugLogger(*debug), showProgress, sinceDate, *batchSize, *settle, *stage, *dryRun)
}

// defaultBatchSize is how many files publish sends per osxphotos import call.
// Fewer, larger calls mean fewer osxphotos/AppleScript sessions against Photos —
// the churn that degrades a whole-library run — while staying small enough to
// keep each call's report and progress legible.
const defaultBatchSize = 250

// defaultSettle is the pause between batches. osxphotos drives Photos over
// AppleScript, and Photos stays responsive only while it is warm; the pause lets
// its background import queue drain between batches without quitting it (a
// restart would cold-launch the next batch into an unloaded library and hang).
const defaultSettle = 2 * time.Second

// verifyActiveLibrary guards the one operation (osxphotos import, via
// publish's Import calls) that cannot be pinned to a specific library file —
// it always writes into whichever library Photos.app currently has open. It
// compares the asset uuids visible when pinned to want against the uuids
// visible with no --library (the same "currently open" resolution import
// itself uses); a mismatch means Photos.app isn't on the target library and
// an import would silently land in the wrong place.
func verifyActiveLibrary(want string) error {
	pinned, err := (photos.OSXPhotos{PhotosLibrary: want}).LiveUUIDs()
	if err != nil {
		return fmt.Errorf("querying --photos-library %s: %w", want, err)
	}
	ambient, err := (photos.OSXPhotos{}).LiveUUIDs()
	if err != nil {
		return fmt.Errorf("querying the currently open Photos library: %w", err)
	}
	if len(pinned) != len(ambient) {
		return fmt.Errorf("--photos-library %s is not the library Photos.app currently has open (pinned query found %d asset(s), the open library has %d) — switch Photos.app to it first", want, len(pinned), len(ambient))
	}
	pinnedUUIDs := make(map[string]bool, len(pinned))
	for _, u := range pinned {
		pinnedUUIDs[u] = true
	}
	for _, u := range ambient {
		if !pinnedUUIDs[u] {
			return fmt.Errorf("--photos-library %s is not the library Photos.app currently has open — switch Photos.app to it first", want)
		}
	}
	return nil
}

// publish pushes unpublished derivatives into Apple Photos. Layer 1 skips our
// own prior pushes (photos_uuid already set, so not selected); layer 2 skips a
// base derivative whose frame already exists in Photos by natural key,
// recording the association. Edits always import as new assets; nothing is
// ever deleted or replaced.
func publish(idx *index.Index, lib photos.Library, logf func(string, ...any), showProgress bool, sinceDate time.Time, batchSize int, settle time.Duration, stageDir string, dryRun bool) error {
	start := time.Now()

	// Refresh the manifest cache and build the natural-key matcher. Under
	// --dry-run the cache rows are not written.
	if showProgress {
		fmt.Fprintln(os.Stderr, "querying the Photos library manifest…")
	}
	assets, err := lib.Manifest()
	if err != nil {
		return err
	}
	if !dryRun {
		for _, a := range assets {
			if err := idx.PutManifest(a.UUID, a.OriginalFilename, a.CaptureTime, a.CatalogKey); err != nil {
				return err
			}
		}
	}
	matcher := photos.NewMatcher(assets)
	logf("manifest holds %d asset(s)", len(assets))

	pending, err := idx.UnpublishedDerivatives()
	if err != nil {
		return err
	}

	var imported, associated, failed int
	var firstErr error

	// Phase 1: drop --since skips and settle Layer-2 associations (fast index
	// writes), collecting the derivatives that still need an actual Photos
	// import. Sizing the bar to this set — not len(pending) — keeps its length
	// honest under --since and keeps it from blasting through instant skips.
	var toImport []index.Derivative
	for _, d := range pending {
		if !sinceDate.IsZero() {
			captureTime, _, ok := photos.ParseStem(d.Stem)
			if ok && captureTime.Before(sinceDate) {
				continue
			}
		}
		// Layer 2 applies to the base only: an edit render exists nowhere in
		// Photos, so a natural-key hit on its frame must not suppress it.
		if d.SourceKind != "edit" {
			if uuid, ok := matcher.Match(d.Stem); ok {
				associated++
				if dryRun {
					logf("would associate %s with existing asset %s", d.HeicPath, uuid)
					continue
				}
				logf("associate %s with existing asset %s", d.HeicPath, uuid)
				if err := idx.MarkPublished(d.SourceHash, uuid); err != nil {
					return err
				}
				continue
			}
		}
		toImport = append(toImport, d)
	}

	// Phase 2: the slow work — import in batches so a large run makes a handful
	// of osxphotos/AppleScript sessions instead of thousands, with a settle pause
	// between batches to let Photos drain while staying warm. The bar advances per
	// file so it reflects real progress rather than skips.
	staging := stageDir != ""
	var bar *progressbar.ProgressBar
	if showProgress && !dryRun && len(toImport) > 0 {
		label := "publishing"
		if staging {
			label = "staging"
		}
		bar = progressbar.Default(int64(len(toImport)), label)
	}
	switch {
	case staging:
		// Stage mode: hardlink each derivative into DIR/YYYY/MM for a native
		// Photos import, marking nothing published — `pm link` reconnects the
		// index by filename after the import.
		for _, d := range toImport {
			if bar != nil {
				bar.Add(1)
			}
			dst := stageTarget(stageDir, d.HeicPath)
			if dryRun {
				imported++
				logf("would stage %s -> %s", d.HeicPath, dst)
				continue
			}
			if _, err := os.Stat(dst); err == nil {
				imported++ // already staged on a prior run
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.Link(d.HeicPath, dst); err != nil {
				if errors.Is(err, syscall.EXDEV) {
					return fmt.Errorf("cannot hardlink into %s: --stage must be on the same volume as the library — choose a directory under the library volume: %w", stageDir, err)
				}
				return fmt.Errorf("hardlink %s -> %s: %w", d.HeicPath, dst, err)
			}
			imported++
			logf("staged %s -> %s", d.HeicPath, dst)
		}
	case dryRun:
		for _, d := range toImport {
			imported++
			logf("would import %s", d.HeicPath)
		}
	default:
		step := batchSize
		if step <= 0 {
			step = len(toImport)
		}
		for start := 0; start < len(toImport); start += step {
			end := start + step
			if end > len(toImport) {
				end = len(toImport)
			}
			batch := toImport[start:end]

			paths := make([]string, len(batch))
			byPath := make(map[string]index.Derivative, len(batch))
			for i, d := range batch {
				paths[i] = d.HeicPath
				byPath[d.HeicPath] = d
			}
			results, err := lib.ImportBatch(paths)
			if err != nil {
				return err // osxphotos couldn't run at all
			}
			for _, p := range paths {
				if bar != nil {
					bar.Add(1)
				}
				r := results[p]
				if r.Err != nil {
					failed++
					logf("import %s: %v", p, r.Err)
					if firstErr == nil {
						firstErr = r.Err
					}
					continue
				}
				if err := idx.MarkPublished(byPath[p].SourceHash, r.UUID); err != nil {
					return err
				}
				imported++
				logf("imported %s as %s", p, r.UUID)
			}
			// Pause between batches (not after the last) so Photos can drain its
			// background import queue while staying warm.
			if settle > 0 && end < len(toImport) {
				time.Sleep(settle)
			}
		}
	}

	if bar != nil {
		bar.Finish()
		fmt.Fprintln(os.Stderr)
	}

	if staging {
		verb := "Staged"
		if dryRun {
			verb = "Would stage"
		}
		fmt.Printf("%s %d HEIC(s) to %s in %s; %d matched existing assets.\n",
			verb, imported, stageDir, time.Since(start).Round(time.Millisecond), associated)
		if !dryRun && imported > 0 {
			fmt.Println("Import them into Photos, then run `pm link`.")
		}
		return nil
	}

	verb := "Published"
	if dryRun {
		verb = "Would publish"
	}
	fmt.Printf("%s %d HEIC(s) in %s; %d matched existing assets", verb, imported, time.Since(start).Round(time.Millisecond), associated)
	if failed > 0 {
		fmt.Printf("; %d error(s)", failed)
	}
	fmt.Println(".")
	if firstErr != nil {
		fmt.Fprintf(os.Stderr, "first error: %v\n", firstErr)
	}
	return nil
}

// stageTarget maps a derivative's Export HEIC path to its hardlink destination
// under stageDir, preserving the trailing YYYY/MM so the staged tree mirrors
// Export/ and can be imported into Photos a year at a time.
func stageTarget(stageDir, heicPath string) string {
	dir := filepath.Dir(heicPath)            // .../Export/YYYY/MM
	mm := filepath.Base(dir)                 // MM
	yyyy := filepath.Base(filepath.Dir(dir)) // YYYY
	return filepath.Join(stageDir, yyyy, mm, filepath.Base(heicPath))
}
