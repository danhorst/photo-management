// Package photos talks to Apple Photos via osxphotos: a manifest query for
// overlap detection, and a flat import that returns the new asset's uuid. The
// natural-key matcher is a plain function so publish logic tests without a
// Photos library.
package photos

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Asset is one Apple Photos asset, as reported by the manifest query.
type Asset struct {
	UUID             string
	OriginalFilename string
	CaptureTime      time.Time
	CatalogKey       string
}

// Library abstracts the osxphotos shell-outs so publish logic is testable.
type Library interface {
	Manifest() ([]Asset, error)
	Import(path string) (uuid string, err error)
}

// stemLayout is the timestamp prefix of a canonical archive stem.
const stemLayout = "2006-01-02--15-04-05"

// ParseStem splits a canonical stem (YYYY-MM-DD--HH-MM-SS-<original>) into
// its capture time and original name.
func ParseStem(stem string) (t time.Time, original string, ok bool) {
	if len(stem) < len(stemLayout)+2 || stem[len(stemLayout)] != '-' {
		return time.Time{}, "", false
	}
	t, err := time.ParseInLocation(stemLayout, stem[:len(stemLayout)], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}
	return t, stem[len(stemLayout)+1:], true
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

// OSXPhotos implements Library by shelling out to the osxphotos CLI.
type OSXPhotos struct{}

type manifestEntry struct {
	UUID             string `json:"uuid"`
	OriginalFilename string `json:"original_filename"`
	Date             string `json:"date"`
}

// Manifest queries every asset in the Photos library.
func (OSXPhotos) Manifest() ([]Asset, error) {
	out, err := exec.Command("osxphotos", "query", "--json", "--mute").Output()
	if err != nil {
		return nil, fmt.Errorf("osxphotos query: %w", err)
	}
	var entries []manifestEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parsing osxphotos manifest: %w", err)
	}
	assets := make([]Asset, 0, len(entries))
	for _, e := range entries {
		a := Asset{UUID: e.UUID, OriginalFilename: e.OriginalFilename}
		if t, err := time.Parse(time.RFC3339Nano, e.Date); err == nil {
			a.CaptureTime = t
		}
		assets = append(assets, a)
	}
	return assets, nil
}

// Import pushes one file into Apple Photos as a flat import (no album) and
// returns the new asset's uuid from the import report.
func (OSXPhotos) Import(path string) (string, error) {
	report, err := os.CreateTemp("", "photo-management-import-*.json")
	if err != nil {
		return "", err
	}
	report.Close()
	defer os.Remove(report.Name())

	out, err := exec.Command("osxphotos", "import", path,
		"--report", report.Name(), "--no-progress").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osxphotos import %s: %v: %s", path, err, out)
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
