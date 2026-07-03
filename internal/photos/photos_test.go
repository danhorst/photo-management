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

	// Reduced-precision stems now parse, so publish/pull no longer reject them.
	tm, orig, ok = ParseStem("2021-12-05-IMG_0003")
	if !ok || orig != "IMG_0003" || tm.Format("2006-01-02") != "2021-12-05" {
		t.Errorf("day stem = %v %q %v", tm, orig, ok)
	}
	tm, orig, ok = ParseStem("2021-12-IMG_0003")
	if !ok || orig != "IMG_0003" || tm.Format("2006-01-02") != "2021-12-01" {
		t.Errorf("month stem = %v %q %v", tm, orig, ok)
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

func TestPullable(t *testing.T) {
	allowed := AllowedDevices([]string{"Apple iPhone 13 mini"})
	published := map[string]bool{"our-derivative": true}
	iphone := Asset{UUID: "a1", CameraMake: "Apple", CameraModel: "iPhone 13 mini",
		CaptureTime: date("2026-06-01T12:00:00-05:00")}

	if !Pullable(iphone, allowed, published, time.Time{}) {
		t.Error("allowlisted iPhone photo should be pullable")
	}

	fuji := iphone
	fuji.CameraMake, fuji.CameraModel = "FUJIFILM", "X-T5"
	if Pullable(fuji, allowed, published, time.Time{}) {
		t.Error("non-allowlisted device should not be pullable")
	}

	movie := iphone
	movie.IsMovie = true
	if Pullable(movie, allowed, published, time.Time{}) {
		t.Error("movies should not be pullable")
	}

	ours := iphone
	ours.UUID = "our-derivative"
	if Pullable(ours, allowed, published, time.Time{}) {
		t.Error("a published derivative must never be re-ingested")
	}

	since := date("2026-06-02T00:00:00-05:00")
	if Pullable(iphone, allowed, published, since) {
		t.Error("asset before --since should not be pullable")
	}
}

func TestFullDiskAccessHint(t *testing.T) {
	if h := fullDiskAccessHint([]byte("some unrelated osxphotos error")); h != "" {
		t.Errorf("unrelated output should yield no hint, got %q", h)
	}
	if h := fullDiskAccessHint([]byte("...Operation not permitted...")); h == "" {
		t.Error("the TCC 'Operation not permitted' signature should yield a hint")
	}
	if h := fullDiskAccessHint([]byte("...NSCocoaErrorDomain Code=513...")); h == "" {
		t.Error("the TCC 'NSCocoaErrorDomain Code=513' signature should yield a hint")
	}
	if h := fullDiskAccessHint(nil); h != "" {
		t.Errorf("nil output should yield no hint, got %q", h)
	}
}

func TestAutomationHint(t *testing.T) {
	if h := automationHint([]byte("some unrelated osxphotos error")); h != "" {
		t.Errorf("unrelated output should yield no hint, got %q", h)
	}
	if h := automationHint([]byte("...ScriptError: Not authorized to send Apple events to Photos. (-1743)...")); h == "" {
		t.Error("the -1743 automation signature should yield a hint")
	}
	if h := automationHint(nil); h != "" {
		t.Errorf("nil output should yield no hint, got %q", h)
	}
}

func TestLastLine(t *testing.T) {
	traceback := "Traceback (most recent call last):\n  frame\n\nAppleScriptError: Not authorized to send Apple events to Photos. (-1743)\n\n"
	if got := lastLine([]byte(traceback)); got != "AppleScriptError: Not authorized to send Apple events to Photos. (-1743)" {
		t.Errorf("lastLine = %q, want the final AppleScriptError line", got)
	}
	if got := lastLine(nil); got != "" {
		t.Errorf("lastLine(nil) = %q, want empty", got)
	}
}
