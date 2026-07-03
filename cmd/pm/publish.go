package main

import (
	"flag"
	"fmt"
	"os"
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

	if *photosLib != "" {
		if err := verifyActiveLibrary(*photosLib); err != nil {
			return err
		}
	}

	showProgress := !*debug && isatty.IsTerminal(os.Stderr.Fd())
	return publish(idx, photos.OSXPhotos{PhotosLibrary: *photosLib}, debugLogger(*debug), showProgress, sinceDate, *dryRun)
}

// verifyActiveLibrary guards the one operation (osxphotos import, via
// publish's Import calls) that cannot be pinned to a specific library file —
// it always writes into whichever library Photos.app currently has open. It
// compares the asset uuids visible when pinned to want against the uuids
// visible with no --library (the same "currently open" resolution import
// itself uses); a mismatch means Photos.app isn't on the target library and
// an import would silently land in the wrong place.
func verifyActiveLibrary(want string) error {
	pinned, err := (photos.OSXPhotos{PhotosLibrary: want}).Manifest()
	if err != nil {
		return fmt.Errorf("querying --photos-library %s: %w", want, err)
	}
	ambient, err := (photos.OSXPhotos{}).Manifest()
	if err != nil {
		return fmt.Errorf("querying the currently open Photos library: %w", err)
	}
	if len(pinned) != len(ambient) {
		return fmt.Errorf("--photos-library %s is not the library Photos.app currently has open (pinned query found %d asset(s), the open library has %d) — switch Photos.app to it first", want, len(pinned), len(ambient))
	}
	pinnedUUIDs := make(map[string]bool, len(pinned))
	for _, a := range pinned {
		pinnedUUIDs[a.UUID] = true
	}
	for _, a := range ambient {
		if !pinnedUUIDs[a.UUID] {
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
func publish(idx *index.Index, lib photos.Library, logf func(string, ...any), showProgress bool, sinceDate time.Time, dryRun bool) error {
	start := time.Now()

	// Refresh the manifest cache and build the natural-key matcher. Under
	// --dry-run the cache rows are not written.
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

	// Phase 2: the slow work — one osxphotos import per derivative. The bar
	// advances only here, so it reflects real progress rather than skips.
	var bar *progressbar.ProgressBar
	if showProgress && !dryRun && len(toImport) > 0 {
		bar = progressbar.Default(int64(len(toImport)), "publishing")
	}
	for _, d := range toImport {
		if bar != nil {
			bar.Add(1)
		}
		if dryRun {
			imported++
			logf("would import %s", d.HeicPath)
			continue
		}
		uuid, err := lib.Import(d.HeicPath)
		if err != nil {
			failed++
			logf("import %s: %v", d.HeicPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := idx.MarkPublished(d.SourceHash, uuid); err != nil {
			return err
		}
		imported++
		logf("imported %s as %s", d.HeicPath, uuid)
	}

	if bar != nil {
		bar.Finish()
		fmt.Fprintln(os.Stderr)
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
