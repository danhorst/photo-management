package organize

import (
	"testing"
	"time"
)

func TestDest(t *testing.T) {
	when := time.Date(2026, 6, 2, 15, 47, 30, 0, time.Local)
	got := Dest("/Volumes/Photos", when, "DSCF1297.RAF")
	want := "/Volumes/Photos/2026/06/2026-06-02--15-47-30-DSCF1297.RAF"
	if got != want {
		t.Errorf("Dest() = %q, want %q", got, want)
	}
}

func TestDestZeroPads(t *testing.T) {
	when := time.Date(2004, 1, 9, 7, 5, 3, 0, time.Local)
	got := Dest("/lib", when, "x.jpg")
	want := "/lib/2004/01/2004-01-09--07-05-03-x.jpg"
	if got != want {
		t.Errorf("Dest() = %q, want %q", got, want)
	}
}

func TestDestAtReducedPrecision(t *testing.T) {
	when := time.Date(2021, 12, 5, 0, 0, 0, 0, time.Local)
	if got := DestAt("/lib", when, Day, "IMG_0003.jpg"); got != "/lib/2021/12/2021-12-05-IMG_0003.jpg" {
		t.Errorf("day DestAt() = %q", got)
	}
	if got := DestAt("/lib", when, Month, "IMG_0003.jpg"); got != "/lib/2021/12/2021-12-IMG_0003.jpg" {
		t.Errorf("month DestAt() = %q", got)
	}
}

func TestParseStemRoundTrips(t *testing.T) {
	cases := []struct {
		stem     string
		prec     Precision
		date     string
		original string
	}{
		{"2021-12-05--14-03-22-IMG_0003", Second, "2021-12-05 14:03:22", "IMG_0003"},
		{"2021-12-05-IMG_0003", Day, "2021-12-05 00:00:00", "IMG_0003"},
		{"2021-12-IMG_0003", Month, "2021-12-01 00:00:00", "IMG_0003"},
		// Hyphens inside the original name stay part of the name.
		{"2021-12-IMG_0003-ZF-9821-41309-1-001-004", Month, "2021-12-01 00:00:00", "IMG_0003-ZF-9821-41309-1-001-004"},
	}
	for _, c := range cases {
		tm, prec, orig, ok := ParseStem(c.stem)
		if !ok {
			t.Errorf("%q should parse", c.stem)
			continue
		}
		if prec != c.prec {
			t.Errorf("%q precision = %v, want %v", c.stem, prec, c.prec)
		}
		if tm.Format("2006-01-02 15:04:05") != c.date {
			t.Errorf("%q date = %v, want %s", c.stem, tm, c.date)
		}
		if orig != c.original {
			t.Errorf("%q original = %q, want %q", c.stem, orig, c.original)
		}
	}
}

func TestParseStemAmbiguity(t *testing.T) {
	// A month-precision original beginning NN- reads as day; an original
	// beginning HH-MM-SS- reads as second. Documented, accepted collisions.
	if _, prec, orig, ok := ParseStem("2021-12-05-x"); !ok || prec != Day || orig != "x" {
		t.Errorf("NN- original: prec=%v orig=%q ok=%v, want day/x", prec, orig, ok)
	}
	if _, prec, _, ok := ParseStem("2021-12-05--14-03-22-x"); !ok || prec != Second {
		t.Errorf("HH-MM-SS- original: prec=%v ok=%v, want second", prec, ok)
	}
}

func TestParseStemRejectsJunk(t *testing.T) {
	for _, s := range []string{"IMG_0003-ZF-9821", "nope", "", "2021-13-40-x"} {
		if _, _, _, ok := ParseStem(s); ok {
			t.Errorf("%q should not parse", s)
		}
	}
}
