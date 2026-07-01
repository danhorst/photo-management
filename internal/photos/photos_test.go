package photos

import (
	"testing"
	"time"
)

func TestParseStem(t *testing.T) {
	tm, orig, ok := ParseStem("2026-06-01--12-00-00-DSCF1234")
	if !ok {
		t.Fatal("canonical stem should parse")
	}
	if orig != "DSCF1234" {
		t.Errorf("original = %q", orig)
	}
	if tm.Format("2006-01-02 15:04:05") != "2026-06-01 12:00:00" {
		t.Errorf("time = %v", tm)
	}

	// Hyphens in the original name stay part of the name.
	_, orig, ok = ParseStem("2026-06-01--12-00-00-IMG-20260601")
	if !ok || orig != "IMG-20260601" {
		t.Errorf("hyphenated original = %q %v", orig, ok)
	}

	if _, _, ok := ParseStem("not-a-stem"); ok {
		t.Error("malformed stem should not parse")
	}
}

func date(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMatcherMatchesNaturalKey(t *testing.T) {
	m := NewMatcher([]Asset{
		{UUID: "uuid-1", OriginalFilename: "DSCF1234.JPG", CaptureTime: date("2026-06-01T12:00:00-05:00")},
	})
	uuid, ok := m.Match("2026-06-01--12-00-00-DSCF1234")
	if !ok || uuid != "uuid-1" {
		t.Errorf("Match = %q %v, want uuid-1 (wall-clock + name sans extension)", uuid, ok)
	}
}

func TestMatcherNoMatch(t *testing.T) {
	m := NewMatcher([]Asset{
		{UUID: "uuid-1", OriginalFilename: "DSCF1234.JPG", CaptureTime: date("2026-06-01T12:00:00-05:00")},
	})
	if _, ok := m.Match("2026-06-01--12-00-01-DSCF1234"); ok {
		t.Error("different capture second should not match")
	}
	if _, ok := m.Match("2026-06-01--12-00-00-DSCF9999"); ok {
		t.Error("different original name should not match")
	}
}

func TestMatcherIPhoneOriginAlreadyPresent(t *testing.T) {
	// A pulled iPhone frame: Photos holds IMG_0001.HEIC, the archive stem
	// encodes the same capture time and original name.
	m := NewMatcher([]Asset{
		{UUID: "uuid-ip", OriginalFilename: "IMG_0001.HEIC", CaptureTime: date("2026-05-30T08:15:00-05:00")},
	})
	uuid, ok := m.Match("2026-05-30--08-15-00-IMG_0001")
	if !ok || uuid != "uuid-ip" {
		t.Errorf("Match = %q %v, want the existing iPhone asset", uuid, ok)
	}
}
