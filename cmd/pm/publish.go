package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
)

func cmdPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report imports and skips without writing anything")
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

	return publish(idx, photos.OSXPhotos{}, debugLogger(*debug), *dryRun)
}

// publish pushes unpublished derivatives into Apple Photos. Layer 1 skips our
// own prior pushes (photos_uuid already set, so not selected); layer 2 skips a
// base derivative whose frame already exists in Photos by natural key,
// recording the association. Edits always import as new assets; nothing is
// ever deleted or replaced.
func publish(idx *index.Index, lib photos.Library, logf func(string, ...any), dryRun bool) error {
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
	for _, d := range pending {
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
		if dryRun {
			imported++
			logf("would import %s", d.HeicPath)
			continue
		}
		uuid, err := lib.Import(d.HeicPath)
		if err != nil {
			failed++
			logf("import %s: %v", d.HeicPath, err)
			continue
		}
		if err := idx.MarkPublished(d.SourceHash, uuid); err != nil {
			return err
		}
		imported++
		logf("imported %s as %s", d.HeicPath, uuid)
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
	return nil
}
