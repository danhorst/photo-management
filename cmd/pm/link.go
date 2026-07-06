package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
	"github.com/mattn/go-isatty"
)

func cmdLink(args []string) error {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report what would be linked without writing")
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
	return link(idx, photos.OSXPhotos{PhotosLibrary: *photosLib}, debugLogger(*debug), showProgress, *dryRun)
}

// link reconnects the index to a native Photos import: it matches each
// unpublished derivative to a live Photos asset by filename (the asset's
// original_filename equals the derivative's HEIC basename, a unique key) and
// marks it published. It is the inverse of reconcile — it only sets published
// markers, never clears them — so it touches only unpublished rows and never
// clobbers an existing association. It refuses an empty manifest, which would
// otherwise link nothing and mask a failed query.
func link(idx *index.Index, lib photos.Library, logf func(string, ...any), showProgress, dryRun bool) error {
	if showProgress {
		fmt.Fprintln(os.Stderr, "querying the Photos library manifest (uuid + filename)…")
	}
	assets, err := lib.ManifestNames()
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return fmt.Errorf("the live Photos manifest is empty — refusing to link (a failed query or the wrong library would match nothing)")
	}

	// Build filename -> uuid. A filename claimed by more than one asset is
	// ambiguous (a duplicate slipped into Photos); mark it so we skip rather
	// than guess.
	byName := make(map[string]string, len(assets))
	ambiguous := make(map[string]bool)
	for _, a := range assets {
		if _, seen := byName[a.OriginalFilename]; seen {
			ambiguous[a.OriginalFilename] = true
			continue
		}
		byName[a.OriginalFilename] = a.UUID
	}

	pending, err := idx.UnpublishedDerivatives()
	if err != nil {
		return err
	}

	var linked, ambig int
	for _, d := range pending {
		// osxphotos' {original_name} strips the extension, so match on the
		// derivative's stem (basename without .heic).
		base := filepath.Base(d.HeicPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if ambiguous[name] {
			ambig++
			logf("skip %s: filename matches multiple Photos assets", d.HeicPath)
			continue
		}
		uuid, ok := byName[name]
		if !ok {
			continue
		}
		if dryRun {
			linked++
			logf("would link %s to asset %s", d.HeicPath, uuid)
			continue
		}
		if err := idx.MarkPublished(d.SourceHash, uuid); err != nil {
			return err
		}
		linked++
		logf("linked %s to asset %s", d.HeicPath, uuid)
	}

	verb := "Linked"
	if dryRun {
		verb = "Would link"
	}
	fmt.Printf("%s %d of %d unpublished derivative(s) to existing Photos assets; %d unmatched",
		verb, linked, len(pending), len(pending)-linked-ambig)
	if ambig > 0 {
		fmt.Printf("; %d ambiguous", ambig)
	}
	fmt.Println(".")
	return nil
}
