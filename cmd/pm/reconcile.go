package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
	"github.com/mattn/go-isatty"
)

func cmdReconcile(args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report what would be cleared without writing")
	photosLib := fs.String("photos-library", "", "target this Photos library instead of whatever's open")
	fs.Usage = func() { fmt.Print(usage) }
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

	showProgress := !*debug && isatty.IsTerminal(os.Stderr.Fd())
	return reconcile(idx, photos.OSXPhotos{PhotosLibrary: *photosLib}, debugLogger(*debug), showProgress, *dryRun)
}

// reconcile clears the published marker from every derivative whose recorded
// Photos uuid is no longer in the live manifest, so the next publish re-imports
// it. It refuses an empty manifest, which would otherwise un-mark everything.
func reconcile(idx *index.Index, lib photos.Library, logf func(string, ...any), showProgress, dryRun bool) error {
	if showProgress {
		fmt.Fprintln(os.Stderr, "querying the Photos library manifest…")
	}
	uuids, err := lib.LiveUUIDs()
	if err != nil {
		return err
	}
	if len(uuids) == 0 {
		return fmt.Errorf("the live Photos manifest is empty — refusing to reconcile (a failed query or the wrong library would un-mark every published derivative)")
	}
	live := make(map[string]bool, len(uuids))
	for _, u := range uuids {
		live[u] = true
	}

	published, err := idx.PublishedDerivatives()
	if err != nil {
		return err
	}

	var cleared int
	for _, d := range published {
		if live[d.PhotosUUID] {
			continue
		}
		if dryRun {
			cleared++
			logf("would clear %s (uuid %s absent from Photos)", d.HeicPath, d.PhotosUUID)
			continue
		}
		if err := idx.ClearPublished(d.SourceHash); err != nil {
			return err
		}
		cleared++
		logf("cleared %s (uuid %s absent from Photos)", d.HeicPath, d.PhotosUUID)
	}

	verb := "Cleared"
	if dryRun {
		verb = "Would clear"
	}
	fmt.Printf("%s %d of %d published derivative(s) whose asset is gone from Photos; %d still present.\n",
		verb, cleared, len(published), len(published)-cleared)
	if !dryRun && cleared > 0 {
		fmt.Println("Run `pm publish` to re-import them.")
	}
	return nil
}
