package export

import (
	"reflect"
	"testing"
)

func TestGroupRafPlusJpegWithEdits(t *testing.T) {
	frames := Group([]string{
		"/lib/2026/06/2026-06-01--12-00-00-DSCF1234.RAF",
		"/lib/2026/06/2026-06-01--12-00-00-DSCF1234.JPG",
		"/lib/2026/06/2026-06-01--12-00-00-DSCF1234-edit.jpg",
		"/lib/2026/06/2026-06-01--12-00-00-DSCF1234-bw.jpg",
	})
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	f := frames[0]
	if f.Stem != "2026-06-01--12-00-00-DSCF1234" {
		t.Errorf("stem = %q", f.Stem)
	}
	if f.Master == "" || f.JPEG == "" {
		t.Errorf("master/JPEG not classified: %+v", f)
	}
	if len(f.Edits) != 2 || f.Edits[0].Suffix != "bw" || f.Edits[1].Suffix != "edit" {
		t.Errorf("edits = %+v", f.Edits)
	}

	srcs := f.Sources()
	kinds := make([]string, len(srcs))
	for i, s := range srcs {
		kinds[i] = s.Kind
	}
	if !reflect.DeepEqual(kinds, []string{"jpeg", "edit", "edit"}) {
		t.Errorf("sources = %v, want base jpeg plus two edits", kinds)
	}
	if srcs[0].Path != f.JPEG {
		t.Errorf("base source should be the camera JPEG, got %s", srcs[0].Path)
	}
}

func TestGroupRawOnlyUsesEmbedded(t *testing.T) {
	frames := Group([]string{"/lib/2026/06/2026-06-01--12-00-00-DSCF9999.RAF"})
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	srcs := frames[0].Sources()
	if len(srcs) != 1 || srcs[0].Kind != "embedded" || srcs[0].Path != frames[0].Master {
		t.Errorf("sources = %+v, want one embedded source from the RAF", srcs)
	}
}

func TestGroupIPhoneOriginYieldsNoSource(t *testing.T) {
	frames := Group([]string{"/lib/2026/06/2026-06-01--12-00-00-IMG_0001.HEIC"})
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	if f := frames[0]; f.HEIC == "" {
		t.Errorf("HEIC not classified: %+v", f)
	}
	if srcs := frames[0].Sources(); len(srcs) != 0 {
		t.Errorf("iPhone-origin frame should resolve no sources, got %+v", srcs)
	}
}

func TestGroupHyphenatedStemNotMistakenForSuffix(t *testing.T) {
	// IMG-20260601 contains hyphens; only the -pano file is an edit.
	frames := Group([]string{
		"/lib/2026/06/2026-06-01--12-00-00-IMG-20260601.RAF",
		"/lib/2026/06/2026-06-01--12-00-00-IMG-20260601-pano.jpg",
	})
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1: %+v", len(frames), frames)
	}
	f := frames[0]
	if f.Stem != "2026-06-01--12-00-00-IMG-20260601" {
		t.Errorf("stem = %q", f.Stem)
	}
	if len(f.Edits) != 1 || f.Edits[0].Suffix != "pano" {
		t.Errorf("edits = %+v, want one edit with suffix pano", f.Edits)
	}
}

func TestGroupJpegOnlyFrameWithEditRegardlessOfOrder(t *testing.T) {
	// No RAW: the JPG defines the frame and the -bw file is its edit, even
	// when the edit sorts before the base.
	frames := Group([]string{
		"/lib/2026/06/2026-06-02--09-00-00-P1000001-bw.jpg",
		"/lib/2026/06/2026-06-02--09-00-00-P1000001.jpg",
	})
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1: %+v", len(frames), frames)
	}
	f := frames[0]
	if f.JPEG == "" || len(f.Edits) != 1 || f.Edits[0].Suffix != "bw" {
		t.Errorf("frame = %+v, want JPEG base plus bw edit", f)
	}
}

func TestGroupSeparateFramesStaySeparate(t *testing.T) {
	frames := Group([]string{
		"/lib/2026/06/2026-06-01--12-00-00-DSCF0001.RAF",
		"/lib/2026/06/2026-06-01--12-00-05-DSCF0002.RAF",
	})
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
}

func TestCaptureDate(t *testing.T) {
	f := Frame{Stem: "2026-06-01--12-00-00-DSCF1234"}
	d, ok := f.CaptureDate()
	if !ok || d.Format("2006-01-02") != "2026-06-01" {
		t.Errorf("CaptureDate = %v %v", d, ok)
	}
	if _, ok := (Frame{Stem: "nope"}).CaptureDate(); ok {
		t.Error("malformed stem should not parse")
	}
}

func TestCaptureDateReducedPrecision(t *testing.T) {
	// Day- and month-precision stems are canonical too: export must date them
	// rather than skip them as non-canonical.
	for stem, want := range map[string]string{
		"2021-12-05-IMG_0003":                      "2021-12-05",
		"2021-12-IMG_0003-ZF-9821-41309-1-001-004": "2021-12-01",
	} {
		d, ok := (Frame{Stem: stem}).CaptureDate()
		if !ok || d.Format("2006-01-02") != want {
			t.Errorf("CaptureDate(%q) = %v %v, want %s", stem, d, ok, want)
		}
	}
	// A name with no leading date is still non-canonical.
	if _, ok := (Frame{Stem: "IMG_0003-ZF-9821-41309-1-001-004"}).CaptureDate(); ok {
		t.Error("bare non-canonical stem should not parse")
	}
}
