// Package photos talks to Apple Photos via osxphotos: a manifest query for
// overlap detection, and a flat import that returns the new asset's uuid. The
// natural-key matcher is a plain function so publish logic tests without a
// Photos library.
package photos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbh/photo-management/internal/organize"
)

// Asset is one Apple Photos asset, as reported by the manifest query.
type Asset struct {
	UUID             string
	OriginalFilename string
	CaptureTime      time.Time
	CatalogKey       string
	CameraMake       string
	CameraModel      string
	IsMovie          bool
}

// Device returns the asset's capture device as "<make> <model>", the form the
// pull allowlist uses (e.g. "Apple iPhone 13 mini").
func (a Asset) Device() string {
	return strings.TrimSpace(a.CameraMake + " " + a.CameraModel)
}

// Library abstracts the osxphotos shell-outs so publish logic is testable.
type Library interface {
	Manifest() ([]Asset, error)
	Import(path string) (uuid string, err error)
}

// stemLayout is the timestamp prefix of a canonical archive stem, the natural
// key's wall-clock format.
const stemLayout = "2006-01-02--15-04-05"

// ParseStem splits a canonical stem into its capture time and original name,
// accepting second, day, or month precision (reduced-precision times fall on
// midnight / the first of the month).
func ParseStem(stem string) (t time.Time, original string, ok bool) {
	t, _, original, ok = organize.ParseStem(stem)
	return t, original, ok
}

// Matcher matches archive frames against Photos assets on the natural key:
// capture wall-clock time plus original filename compared without extension,
// case-insensitively. Wall-clock comparison sidesteps timezone drift between
// EXIF capture dates and Photos' zoned dates.
type Matcher map[string]string

func naturalKey(name string, t time.Time) string {
	return strings.ToLower(name) + "|" + t.Format(stemLayout)
}

// NewMatcher indexes assets by natural key.
func NewMatcher(assets []Asset) Matcher {
	m := Matcher{}
	for _, a := range assets {
		name := strings.TrimSuffix(a.OriginalFilename, filepath.Ext(a.OriginalFilename))
		m[naturalKey(name, a.CaptureTime)] = a.UUID
	}
	return m
}

// Match returns the uuid of the Photos asset the stem's frame corresponds to,
// if one is already present.
func (m Matcher) Match(stem string) (uuid string, ok bool) {
	t, original, parsed := ParseStem(stem)
	if !parsed {
		return "", false
	}
	uuid, ok = m[naturalKey(original, t)]
	return uuid, ok
}

// OSXPhotos implements Library by shelling out to the osxphotos CLI. When
// PhotosLibrary is set, every query/export call is pinned to that library
// file directly; import cannot be pinned this way (see Import) so callers
// that write must ensure Photos.app already has PhotosLibrary open.
type OSXPhotos struct {
	PhotosLibrary string
}

// libraryArgs returns the --library flag pair when PhotosLibrary is set, or
// nil otherwise.
func (o OSXPhotos) libraryArgs() []string {
	if o.PhotosLibrary == "" {
		return nil
	}
	return []string{"--library", o.PhotosLibrary}
}

// fullDiskAccessHint appends a plain-language hint to osxphotos output that
// carries macOS's TCC signature for a missing Full Disk Access grant.
// osxphotos copies the library's sqlite files out of the (often
// TCC-protected, e.g. ~/Pictures) library path; without Full Disk Access that
// copy fails and osxphotos surfaces a raw Python traceback instead of a clean
// error.
func fullDiskAccessHint(output []byte) string {
	s := string(output)
	if strings.Contains(s, "Operation not permitted") || strings.Contains(s, "NSCocoaErrorDomain Code=513") {
		return "\nhint: this looks like a missing Full Disk Access grant — add your terminal app in System Settings > Privacy & Security > Full Disk Access, then restart it and retry"
	}
	return ""
}

// automationHint appends a plain-language hint to osxphotos output that carries
// macOS's TCC signature for a missing Automation (Apple Events) grant. Without
// it osxphotos cannot drive Photos.app and surfaces a raw Python traceback
// ending in AppleScriptError -1743.
func automationHint(output []byte) string {
	s := string(output)
	if strings.Contains(s, "-1743") || strings.Contains(s, "Not authorized to send Apple events") {
		return "\nhint: your terminal app isn't allowed to control Photos — enable it in System Settings > Privacy & Security > Automation (or run `tccutil reset AppleEvents` and retry to re-trigger the prompt)"
	}
	return ""
}

// lastLine returns the last non-blank line of output, trimmed — the actual
// error osxphotos prints (e.g. AppleScriptError: …) without the traceback above
// it.
func lastLine(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}

// exitErrOutput returns the captured stderr from err when it is an
// *exec.ExitError (as populated by Cmd.Output), or nil otherwise.
func exitErrOutput(err error) []byte {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.Stderr
	}
	return nil
}

type manifestEntry struct {
	UUID             string `json:"uuid"`
	OriginalFilename string `json:"original_filename"`
	Date             string `json:"date"`
	IsMovie          bool   `json:"ismovie"`
	ExifInfo         *struct {
		CameraMake  string `json:"camera_make"`
		CameraModel string `json:"camera_model"`
	} `json:"exif_info"`
}

// Manifest queries every asset in the Photos library.
func (o OSXPhotos) Manifest() ([]Asset, error) {
	args := append([]string{"query", "--json", "--mute"}, o.libraryArgs()...)
	out, err := exec.Command("osxphotos", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("osxphotos query: %w%s", err, fullDiskAccessHint(exitErrOutput(err)))
	}
	var entries []manifestEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parsing osxphotos manifest: %w", err)
	}
	assets := make([]Asset, 0, len(entries))
	for _, e := range entries {
		a := Asset{UUID: e.UUID, OriginalFilename: e.OriginalFilename, IsMovie: e.IsMovie}
		if t, err := time.Parse(time.RFC3339Nano, e.Date); err == nil {
			a.CaptureTime = t
		}
		if e.ExifInfo != nil {
			a.CameraMake, a.CameraModel = e.ExifInfo.CameraMake, e.ExifInfo.CameraModel
		}
		assets = append(assets, a)
	}
	return assets, nil
}

// Import pushes one file into Apple Photos as a flat import (no album) and
// returns the new asset's uuid from the import report. Unlike query/export,
// osxphotos import does not honor --library as a target selector — it always
// writes into whichever library Photos.app currently has open; --library is
// passed here only as the fallback hint osxphotos itself documents, not a
// safety guarantee. Callers pinning PhotosLibrary must ensure Photos.app is
// already switched to it before calling Import.
func (o OSXPhotos) Import(path string) (string, error) {
	report, err := os.CreateTemp("", "photo-management-import-*.json")
	if err != nil {
		return "", err
	}
	report.Close()
	defer os.Remove(report.Name())

	args := append([]string{"import", path, "--report", report.Name(), "--no-progress"}, o.libraryArgs()...)
	out, err := exec.Command("osxphotos", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osxphotos import %s: %v: %s%s%s", path, err, lastLine(out), fullDiskAccessHint(out), automationHint(out))
	}

	data, err := os.ReadFile(report.Name())
	if err != nil {
		return "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", fmt.Errorf("parsing import report for %s: %w", path, err)
	}
	for _, r := range rows {
		if u, _ := r["uuid"].(string); u != "" {
			return u, nil
		}
	}
	return "", fmt.Errorf("osxphotos import %s: no uuid in report", path)
}
