// Package export generates presentation-tier HEIC derivatives from archive
// frames into Export/, grouping files into frames by their canonical stem and
// recording each derivative for incremental regeneration.
package export

import (
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// rawExt is the set of RAW master extensions (lowercase, with dot).
var rawExt = map[string]bool{
	".raf": true, ".cr2": true, ".dng": true, ".crw": true,
}

// imageExt is the set of image extensions an edit or base can carry.
var imageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".heic": true, ".png": true,
	".gif": true, ".tif": true, ".tiff": true,
}

// Edit is one baked Capture One edit belonging to a frame.
type Edit struct {
	Suffix string
	Path   string
}

// Frame groups the archive files sharing one canonical stem
// (YYYY-MM-DD--HH-MM-SS-<original>).
type Frame struct {
	Stem   string
	Master string // RAW file path, if any
	JPEG   string // camera JPEG path, if any
	HEIC   string // iPhone-origin HEIC path, if any
	Edits  []Edit
}

// CaptureDate parses the frame's capture date from the stem's timestamp
// prefix.
func (f Frame) CaptureDate() (time.Time, bool) {
	if len(f.Stem) < 10 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", f.Stem[:10], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Group classifies paths into frames keyed off the canonical stem. A name that
// extends a known stem with `-<suffix>` is an edit of that stem (longest stem
// wins, so hyphens inside stems are never mistaken for a suffix); every other
// image name defines its own frame. RAW files always define a frame.
func Group(paths []string) []Frame {
	type entry struct{ name, ext, path string }
	var entries []entry
	stems := map[string]bool{}
	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		if !rawExt[ext] && !imageExt[ext] {
			continue
		}
		base := filepath.Base(p)
		name := base[:len(base)-len(filepath.Ext(base))]
		entries = append(entries, entry{name, ext, p})
		if rawExt[ext] {
			stems[name] = true
		}
	}
	// Non-RAW names that don't extend a known stem define their own frame.
	// Shortest first, so a base name becomes a stem before any of its edits
	// (`<stem>-<suffix>`) is considered.
	var candidates []string
	for _, e := range entries {
		if !rawExt[e.ext] && !stems[e.name] {
			candidates = append(candidates, e.name)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return len(candidates[i]) < len(candidates[j]) })
	for _, name := range candidates {
		if !stems[name] && editStem(name, stems) == "" {
			stems[name] = true
		}
	}

	frames := map[string]*Frame{}
	frame := func(stem string) *Frame {
		if f, ok := frames[stem]; ok {
			return f
		}
		f := &Frame{Stem: stem}
		frames[stem] = f
		return f
	}
	for _, e := range entries {
		if stems[e.name] {
			f := frame(e.name)
			switch {
			case rawExt[e.ext]:
				f.Master = e.path
			case e.ext == ".jpg" || e.ext == ".jpeg":
				f.JPEG = e.path
			case e.ext == ".heic":
				f.HEIC = e.path
			}
			continue
		}
		if stem := editStem(e.name, stems); stem != "" {
			f := frame(stem)
			f.Edits = append(f.Edits, Edit{Suffix: e.name[len(stem)+1:], Path: e.path})
		}
	}

	out := make([]Frame, 0, len(frames))
	for _, f := range frames {
		sort.Slice(f.Edits, func(i, j int) bool { return f.Edits[i].Suffix < f.Edits[j].Suffix })
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Stem < out[j].Stem })
	return out
}

// editStem returns the longest known stem that name extends as
// `<stem>-<suffix>`, or "" when name is not an edit of any known stem.
func editStem(name string, stems map[string]bool) string {
	best := ""
	for stem := range stems {
		if len(stem) > len(best) && len(name) > len(stem)+1 &&
			strings.HasPrefix(name, stem) && name[len(stem)] == '-' {
			best = stem
		}
	}
	return best
}

// Source is one file a derivative is generated from.
type Source struct {
	Kind   string // "jpeg", "embedded", or "edit"
	Path   string // archive file the version id is hashed from
	Suffix string // edit label; empty for the base
}

// Sources resolves the frame's derivative sources: one base (the camera JPEG,
// or the RAW's embedded JPEG when RAW-only) plus one per baked edit.
// Edits never suppress the base. An iPhone-origin frame (HEIC with no camera
// JPEG) yields no base.
func (f Frame) Sources() []Source {
	var out []Source
	switch {
	case f.JPEG != "":
		out = append(out, Source{Kind: "jpeg", Path: f.JPEG})
	case f.Master != "":
		out = append(out, Source{Kind: "embedded", Path: f.Master})
	}
	for _, e := range f.Edits {
		out = append(out, Source{Kind: "edit", Path: e.Path, Suffix: e.Suffix})
	}
	return out
}
