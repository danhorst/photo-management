package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
	"github.com/mattn/go-isatty"
)

// puller is the slice of osxphotos that pull needs.
type puller interface {
	PullManifest() ([]photos.Asset, error)
	Export(dir string, uuids []string) error
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report what would be pulled without writing anything")
	since := fs.String("since", "", "limit to assets captured on/after this date (YYYY-MM-DD)")
	photosLib := fs.String("photos-library", "", "target this Photos library instead of whatever's open")
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

	return pull(cfg, idx, photos.OSXPhotos{PhotosLibrary: *photosLib}, debugLogger(*debug), sinceDate, *dryRun, *debug)
}

// pull exports allowlisted iPhone-origin assets from Apple Photos into the
// queue directory (osxphotos --update skips prior exports), then reuses the
// import core over the queue — BLAKE3 dedup and YYYY/MM organizing unchanged.
// Our own published derivatives are excluded by uuid.
func pull(cfg config.Config, idx *index.Index, lib puller, logf func(string, ...any), since time.Time, dryRun, debug bool) error {
	showProgress := !debug && isatty.IsTerminal(os.Stderr.Fd())
	devices := cfg.PullDevices
	if len(devices) == 0 {
		devices = config.DefaultPullDevices
	}
	allowed := photos.AllowedDevices(devices)

	published, err := idx.PublishedUUIDs()
	if err != nil {
		return err
	}

	if showProgress {
		fmt.Fprintln(os.Stderr, "querying the Photos library manifest…")
	}
	assets, err := lib.PullManifest()
	if err != nil {
		return err
	}
	var uuids []string
	for _, a := range assets {
		if photos.Pullable(a, allowed, published, since) {
			uuids = append(uuids, a.UUID)
		}
	}
	logf("%d of %d asset(s) match the pull filter", len(uuids), len(assets))

	if dryRun {
		fmt.Printf("Would pull %d asset(s) into the queue and import them.\n", len(uuids))
		return nil
	}
	if len(uuids) == 0 {
		fmt.Println("Nothing to pull.")
		return nil
	}

	queue := queueDir()
	if showProgress {
		fmt.Fprintf(os.Stderr, "exporting %d asset(s) from Photos…\n", len(uuids))
	}
	if err := lib.Export(queue, uuids); err != nil {
		return err
	}
	logf("exported queue at %s", queue)

	return runImport(cfg, idx, queue, false, debug, false)
}

// queueDir is the scratch export location pull feeds the importer from. It
// lives in the user cache, off the library volume, so imports copy and the
// queue persists as osxphotos --update state.
func queueDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "photo-management", "pull-queue")
}
