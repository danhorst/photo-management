package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/export"
	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/organize"
)

func cmdRecanon(args []string) error {
	fs := flag.NewFlagSet("recanon", flag.ExitOnError)
	lib, db, debug := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report renames without moving files or touching the index")
	match := fs.String("match", "", "limit to frames whose stem contains this substring")
	dateStr := fs.String("date", "", "stamp day precision YYYY-MM-DD instead of month-from-folder")
	fs.Usage = func() { fmt.Print(usage) }
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}

	var dayDate time.Time
	dayPrecision := false
	if *dateStr != "" {
		t, err := time.ParseInLocation("2006-01-02", *dateStr, time.Local)
		if err != nil {
			return fmt.Errorf("--date must be YYYY-MM-DD, got %q", *dateStr)
		}
		dayDate, dayPrecision = t, true
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

	logf := debugLogger(*debug)

	paths, err := collectArchive(cfg.Library)
	if err != nil {
		return err
	}
	frames := export.Group(paths)

	var renamed, skipped int
	for _, f := range frames {
		if _, _, _, ok := organize.ParseStem(f.Stem); ok {
			continue // already canonical at some precision
		}
		if *match != "" && !strings.Contains(f.Stem, *match) {
			continue
		}

		files := framePaths(f)
		if len(files) == 0 {
			continue
		}

		var t time.Time
		var prec organize.Precision
		if dayPrecision {
			t, prec = dayDate, organize.Day
		} else {
			mt, ok := monthFromPath(cfg.Library, files[0])
			if !ok {
				skipped++
				logf("skip %s: cannot derive year/month from folder", f.Stem)
				continue
			}
			t, prec = mt, organize.Month
		}
		newStem := organize.Stem(t, prec, f.Stem)

		for _, p := range files {
			base := filepath.Base(p)
			newBase := newStem + base[len(f.Stem):]
			newPath := filepath.Join(filepath.Dir(p), newBase)
			if *dryRun {
				fmt.Printf("%s -> %s\n", relTo(cfg.Library, p), newBase)
				continue
			}
			if _, err := os.Stat(newPath); err == nil {
				return fmt.Errorf("renaming %s: %s already exists", p, newPath)
			} else if !os.IsNotExist(err) {
				return err
			}
			if _, err := organize.Place(p, newPath); err != nil {
				return fmt.Errorf("renaming %s: %w", p, err)
			}
			if err := idx.Rename(p, newPath); err != nil {
				return err
			}
			logf("renamed %s -> %s", p, newPath)
		}
		renamed++
	}

	verb := "Renamed"
	if *dryRun {
		verb = "Would rename"
	}
	fmt.Printf("%s %d frame(s)", verb, renamed)
	if skipped > 0 {
		fmt.Printf("; skipped %d with no derivable date (see --debug)", skipped)
	}
	fmt.Println(".")
	if !*dryRun && renamed > 0 {
		fmt.Println("Run `pm export` to generate their derivatives.")
	}
	return nil
}

// framePaths returns every archive file belonging to a frame — master, camera
// JPEG, iPhone HEIC, and each baked edit — so a rename moves the whole frame.
func framePaths(f export.Frame) []string {
	var out []string
	for _, p := range []string{f.Master, f.JPEG, f.HEIC} {
		if p != "" {
			out = append(out, p)
		}
	}
	for _, e := range f.Edits {
		out = append(out, e.Path)
	}
	return out
}

// monthFromPath reads the YYYY/MM the file is filed under, giving month
// precision for a frame whose name carries no date.
func monthFromPath(library, path string) (time.Time, bool) {
	rel, err := filepath.Rel(library, path)
	if err != nil {
		return time.Time{}, false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006/01", parts[0]+"/"+parts[1], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// relTo returns path relative to library for display, falling back to the full
// path.
func relTo(library, path string) string {
	if rel, err := filepath.Rel(library, path); err == nil {
		return rel
	}
	return path
}
