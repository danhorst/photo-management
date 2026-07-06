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
)

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report what export/publish would do without writing anything")
	since := fs.String("since", "", "override the stored watermark; limit to frames captured on/after this date (YYYY-MM-DD)")
	setSince := fs.String("set-since", "", "set the stored watermark to this date (YYYY-MM-DD) without running export or publish, then exit")
	photosLib := fs.String("photos-library", "", "target this Photos library instead of whatever's open (see caveat: osxphotos import ignores this — Photos.app must already have it open)")
	batchSize := fs.Int("batch-size", defaultBatchSize, "import this many files per osxphotos call (0 = one batch)")
	settle := fs.Duration("settle", defaultSettle, "pause this long between batches so Photos can drain its background import queue")
	fs.Usage = func() { fmt.Print(usage) }
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	if *since != "" && *setSince != "" {
		return fmt.Errorf("--since and --set-since are mutually exclusive")
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

	if *setSince != "" {
		t, err := time.ParseInLocation("2006-01-02", *setSince, time.Local)
		if err != nil {
			return fmt.Errorf("--set-since must be YYYY-MM-DD, got %q", *setSince)
		}
		if *dryRun {
			fmt.Printf("Would set watermark to %s.\n", t.Format("2006-01-02"))
			return nil
		}
		if err := idx.SetSyncSince(t); err != nil {
			return err
		}
		fmt.Printf("Watermark set to %s. The next sync only considers frames captured on or after that date.\n", t.Format("2006-01-02"))
		return nil
	}

	var explicitSince time.Time
	if *since != "" {
		t, err := time.ParseInLocation("2006-01-02", *since, time.Local)
		if err != nil {
			return fmt.Errorf("--since must be YYYY-MM-DD, got %q", *since)
		}
		explicitSince = t
	}

	showProgress := !*debug && isatty.IsTerminal(os.Stderr.Fd())
	if *photosLib != "" {
		if showProgress {
			fmt.Fprintf(os.Stderr, "verifying Photos.app has %s open (two library queries)…\n", *photosLib)
		}
		if err := verifyActiveLibrary(*photosLib); err != nil {
			return err
		}
	}

	return runSync(idx, cfg, photos.OSXPhotos{PhotosLibrary: *photosLib}, explicitSince, *dryRun, debugLogger(*debug), showProgress, *batchSize, *settle)
}

// runSync resolves the sync cutoff (an explicit override, else the stored
// watermark, else a full scan), runs export then publish against it, and —
// on a clean, non-dry-run pass — advances the stored watermark to today so
// the next sync only considers what's new. Any failure in either step, or
// --dry-run, leaves the watermark exactly as it was.
func runSync(idx *index.Index, cfg config.Config, lib photos.Library, explicitSince time.Time, dryRun bool, logf func(string, ...any), showProgress bool, batchSize int, settle time.Duration) error {
	sinceDate := explicitSince
	if sinceDate.IsZero() {
		stored, ok, err := idx.LastSyncSince()
		if err != nil {
			return err
		}
		if ok {
			sinceDate = stored
		}
	}
	if sinceDate.IsZero() {
		logf("sync: no stored watermark, full scan")
	} else {
		logf("sync: cutoff %s", sinceDate.Format("2006-01-02"))
	}

	_, _, expFailed, err := runExport(idx, cfg, sinceDate, dryRun, logf, showProgress)
	if err != nil {
		return err
	}

	pubFailed, err := publish(idx, lib, logf, showProgress, sinceDate, batchSize, settle, "", dryRun)
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}
	if expFailed == 0 && pubFailed == 0 {
		today := time.Now()
		if err := idx.SetSyncSince(today); err != nil {
			return err
		}
		fmt.Printf("Watermark advanced to %s.\n", today.Format("2006-01-02"))
	} else {
		fmt.Println("Watermark left unchanged due to failures above; re-run sync to retry.")
	}
	return nil
}
