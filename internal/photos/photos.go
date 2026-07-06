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

// Library abstracts the osxphotos shell-outs so publish logic is testable. Each
// query fetches only the fields its caller needs: Manifest for publish's
// natural-key match, ManifestNames for link, LiveUUIDs for reconcile.
type Library interface {
	Manifest() ([]Asset, error)
	ManifestNames() ([]Asset, error)
	LiveUUIDs() ([]string, error)
	ImportBatch(paths []string) (map[string]ImportResult, error)
}

// ImportResult is the per-file outcome of an import batch: a new asset uuid on
// success, or Err when Photos rejected the file (or it was absent from the
// report).
type ImportResult struct {
	UUID string
	Err  error
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

// pullEntry is the full per-asset record PullManifest parses: enough for pull to
// apply the device allowlist (camera make/model), the movie filter, and --since.
type pullEntry struct {
	UUID             string `json:"uuid"`
	OriginalFilename string `json:"original_filename"`
	Date             string `json:"date"`
	IsMovie          bool   `json:"ismovie"`
	ExifInfo         *struct {
		CameraMake  string `json:"camera_make"`
		CameraModel string `json:"camera_model"`
	} `json:"exif_info"`
}

// PullManifest queries every asset with the fields pull needs. Unlike the other
// queries it cannot be field-limited: the camera make/model it filters on live
// in exif_info, which no cheap template exposes, so it pays the full --json cost.
func (o OSXPhotos) PullManifest() ([]Asset, error) {
	args := append([]string{"query", "--json", "--mute"}, o.libraryArgs()...)
	out, err := exec.Command("osxphotos", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("osxphotos query: %w%s", err, fullDiskAccessHint(exitErrOutput(err)))
	}
	var entries []pullEntry
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

// strftimeLayout / fieldDateLayout are the same wall-clock format either side of
// the osxphotos {created.strftime,…} field: osxphotos renders capture time in it
// (zone-free local wall-clock), and Manifest parses it back with time.Parse.
// Wall-clock matches what the natural-key matcher and canonical stems compare on.
const (
	strftimeLayout  = "%Y-%m-%dT%H:%M:%S"
	fieldDateLayout = "2006-01-02T15:04:05"
)

// fieldEntry is the record the field-limited queries parse; each populates only
// the fields it requests via --field, the rest stay zero.
type fieldEntry struct {
	UUID             string `json:"uuid"`
	OriginalFilename string `json:"original_filename"`
	CaptureTime      string `json:"capture_time"`
}

// queryFields runs a field-limited osxphotos query. A full --json manifest
// computes every field (exif, albums, persons) per photo, which is prohibitively
// slow and memory-heavy once the library holds tens of thousands of assets;
// requesting only named fields skips that work.
func (o OSXPhotos) queryFields(fields ...string) ([]fieldEntry, error) {
	args := append([]string{"query", "--json", "--mute"}, fields...)
	args = append(args, o.libraryArgs()...)
	out, err := exec.Command("osxphotos", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("osxphotos query: %w%s", err, fullDiskAccessHint(exitErrOutput(err)))
	}
	var entries []fieldEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parsing osxphotos manifest: %w", err)
	}
	return entries, nil
}

// Manifest queries the fields publish matches on: uuid, original filename, and
// capture time. Field-limited (see queryFields).
func (o OSXPhotos) Manifest() ([]Asset, error) {
	entries, err := o.queryFields(
		"--field", "uuid", "{uuid}",
		"--field", "original_filename", "{original_name}",
		"--field", "capture_time", "{created.strftime,"+strftimeLayout+"}",
	)
	if err != nil {
		return nil, err
	}
	assets := make([]Asset, 0, len(entries))
	for _, e := range entries {
		a := Asset{UUID: e.UUID, OriginalFilename: e.OriginalFilename}
		if t, err := time.ParseInLocation(fieldDateLayout, e.CaptureTime, time.Local); err == nil {
			a.CaptureTime = t
		}
		assets = append(assets, a)
	}
	return assets, nil
}

// ManifestNames queries only uuid and original filename — all pm link needs to
// match by name. Field-limited (see queryFields).
func (o OSXPhotos) ManifestNames() ([]Asset, error) {
	entries, err := o.queryFields(
		"--field", "uuid", "{uuid}",
		"--field", "original_filename", "{original_name}",
	)
	if err != nil {
		return nil, err
	}
	assets := make([]Asset, 0, len(entries))
	for _, e := range entries {
		assets = append(assets, Asset{UUID: e.UUID, OriginalFilename: e.OriginalFilename})
	}
	return assets, nil
}

// LiveUUIDs queries just the uuid of every asset — all reconcile and the
// active-library check need. Field-limited (see queryFields).
func (o OSXPhotos) LiveUUIDs() ([]string, error) {
	entries, err := o.queryFields("--field", "uuid", "{uuid}")
	if err != nil {
		return nil, err
	}
	uuids := make([]string, 0, len(entries))
	for _, e := range entries {
		uuids = append(uuids, e.UUID)
	}
	return uuids, nil
}

// importReportRecord is the subset of an osxphotos import --report JSON record
// we rely on: filename is the source basename, imported/error the outcome, and
// uuid the new asset id (set only on a genuine import).
type importReportRecord struct {
	Filename string `json:"filename"`
	Imported bool   `json:"imported"`
	Error    bool   `json:"error"`
	UUID     string `json:"uuid"`
}

// ImportBatch imports paths into Apple Photos in a single osxphotos call and
// returns a per-path result keyed by the input path. A path succeeds only when
// the report marks it imported with no error and a uuid; every other path
// (rejected by Photos, or absent from the report) carries an Err, so the caller
// leaves it unpublished for a later retry. A non-nil top-level error means
// osxphotos could not run at all.
//
// Unlike query/export, osxphotos import does not honor --library as a target
// selector — it always writes into whichever library Photos.app currently has
// open; --library is only the fallback hint osxphotos documents. Callers pinning
// PhotosLibrary must ensure Photos.app already has it open.
func (o OSXPhotos) ImportBatch(paths []string) (map[string]ImportResult, error) {
	if len(paths) == 0 {
		return map[string]ImportResult{}, nil
	}
	report, err := os.CreateTemp("", "photo-management-import-*.json")
	if err != nil {
		return nil, err
	}
	report.Close()
	defer os.Remove(report.Name())

	args := append([]string{"import"}, paths...)
	args = append(args, "--report", report.Name(), "--no-progress")
	args = append(args, o.libraryArgs()...)
	out, err := exec.Command("osxphotos", args...).CombinedOutput()

	// osxphotos exits non-zero when some files fail but still writes a report
	// for the rest, so only a missing/empty report is a hard failure.
	data, readErr := os.ReadFile(report.Name())
	if len(data) == 0 {
		if err != nil {
			return nil, fmt.Errorf("osxphotos import: %v: %s%s%s", err, lastLine(out), fullDiskAccessHint(out), automationHint(out))
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return resolveImportReport(data, paths)
}

// resolveImportReport maps each input path to its outcome from the osxphotos
// import report bytes. A path succeeds only when its record is imported, not
// errored, and carries a uuid; a record marked error, or a path absent from the
// report, is a failure the caller leaves unpublished. Records match input paths
// by basename (unique within a batch).
func resolveImportReport(data []byte, paths []string) (map[string]ImportResult, error) {
	var records []importReportRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parsing import report: %w", err)
	}
	byBase := make(map[string]importReportRecord, len(records))
	for _, r := range records {
		byBase[r.Filename] = r
	}
	results := make(map[string]ImportResult, len(paths))
	for _, p := range paths {
		switch r, ok := byBase[filepath.Base(p)]; {
		case !ok:
			results[p] = ImportResult{Err: fmt.Errorf("osxphotos import %s: not in report", p)}
		case r.Error || !r.Imported || r.UUID == "":
			results[p] = ImportResult{Err: fmt.Errorf("osxphotos import %s: Photos did not import it", p)}
		default:
			results[p] = ImportResult{UUID: r.UUID}
		}
	}
	return results, nil
}
