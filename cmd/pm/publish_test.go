package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
)

type fakeLibrary struct {
	assets   []photos.Asset
	imported []string
}

func (f *fakeLibrary) Manifest() ([]photos.Asset, error) { return f.assets, nil }

func (f *fakeLibrary) Import(path string) (string, error) {
	f.imported = append(f.imported, path)
	return "new-uuid-" + filepath.Base(path), nil
}

func discard(string, ...any) {}

func TestPublishTwoDedupLayers(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// One frame already in Photos by natural key; one frame new; one edit of
	// the already-present frame.
	when := time.Date(2026, 6, 1, 12, 0, 0, 0, time.Local)
	lib := &fakeLibrary{assets: []photos.Asset{
		{UUID: "existing-uuid", OriginalFilename: "DSCF1234.JPG", CaptureTime: when},
	}}

	present := "2026-06-01--12-00-00-DSCF1234"
	fresh := "2026-06-01--12-05-00-DSCF1235"
	for _, d := range []struct{ hash, stem, kind, path string }{
		{"hash-base-present", present, "jpeg", "/Export/2026/06/" + present + ".heic"},
		{"hash-edit-present", present, "edit", "/Export/2026/06/" + present + "-bw.heic"},
		{"hash-base-fresh", fresh, "jpeg", "/Export/2026/06/" + fresh + ".heic"},
	} {
		if err := idx.PutDerivative(d.hash, d.stem, d.kind, d.path); err != nil {
			t.Fatal(err)
		}
	}

	if err := publish(idx, lib, discard, time.Time{}, false); err != nil {
		t.Fatal(err)
	}

	// Layer 2: the present frame's base is associated, not imported; the edit
	// and the fresh base are imported.
	if len(lib.imported) != 2 {
		t.Fatalf("imported %v, want the edit and the fresh base only", lib.imported)
	}
	ds, err := idx.Derivatives()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.PhotosUUID == "" {
			t.Errorf("%s should have a photos_uuid after publish", d.SourceHash)
		}
		if d.SourceHash == "hash-base-present" && d.PhotosUUID != "existing-uuid" {
			t.Errorf("present base should associate with existing-uuid, got %s", d.PhotosUUID)
		}
	}

	// Layer 1: a re-run selects nothing and imports nothing.
	lib.imported = nil
	if err := publish(idx, lib, discard, time.Time{}, false); err != nil {
		t.Fatal(err)
	}
	if len(lib.imported) != 0 {
		t.Errorf("re-run imported %v, want nothing", lib.imported)
	}
}

func TestPublishSinceFiltersByCaptureDate(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	early := "2026-05-01--09-00-00-DSCF1000"
	late := "2026-06-15--09-00-00-DSCF1001"
	for _, d := range []struct{ hash, stem, kind, path string }{
		{"hash-early", early, "jpeg", "/Export/2026/05/" + early + ".heic"},
		{"hash-late", late, "jpeg", "/Export/2026/06/" + late + ".heic"},
	} {
		if err := idx.PutDerivative(d.hash, d.stem, d.kind, d.path); err != nil {
			t.Fatal(err)
		}
	}

	lib := &fakeLibrary{}
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	if err := publish(idx, lib, discard, since, false); err != nil {
		t.Fatal(err)
	}
	if len(lib.imported) != 1 || lib.imported[0] != "/Export/2026/06/"+late+".heic" {
		t.Fatalf("imported %v, want only the derivative on/after --since", lib.imported)
	}
}

func TestPublishDryRunWritesNothing(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	lib := &fakeLibrary{}
	if err := publish(idx, lib, discard, time.Time{}, true); err != nil {
		t.Fatal(err)
	}
	if len(lib.imported) != 0 {
		t.Errorf("dry-run imported %v", lib.imported)
	}
	un, err := idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 1 {
		t.Errorf("dry-run must not mark anything published")
	}
}
