package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
)

type fakeLibrary struct {
	assets    []photos.Asset
	imported  []string
	importErr error           // when set, ImportBatch fails wholesale (osxphotos couldn't run)
	rejects   map[string]bool // paths Photos rejects, returned as per-file errors
	batches   int
}

func (f *fakeLibrary) Manifest() ([]photos.Asset, error)      { return f.assets, nil }
func (f *fakeLibrary) ManifestNames() ([]photos.Asset, error) { return f.assets, nil }

func (f *fakeLibrary) LiveUUIDs() ([]string, error) {
	us := make([]string, len(f.assets))
	for i, a := range f.assets {
		us[i] = a.UUID
	}
	return us, nil
}

func (f *fakeLibrary) ImportBatch(paths []string) (map[string]photos.ImportResult, error) {
	f.batches++
	if f.importErr != nil {
		return nil, f.importErr
	}
	res := make(map[string]photos.ImportResult, len(paths))
	for _, p := range paths {
		if f.rejects[p] {
			res[p] = photos.ImportResult{Err: fmt.Errorf("Photos rejected %s", p)}
			continue
		}
		f.imported = append(f.imported, p)
		res[p] = photos.ImportResult{UUID: "new-uuid-" + filepath.Base(p)}
	}
	return res, nil
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

	if _, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, "", false); err != nil {
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
	if _, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, "", false); err != nil {
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
	if _, err := publish(idx, lib, discard, false, since, 0, 0, "", false); err != nil {
		t.Fatal(err)
	}
	if len(lib.imported) != 1 || lib.imported[0] != "/Export/2026/06/"+late+".heic" {
		t.Fatalf("imported %v, want only the derivative on/after --since", lib.imported)
	}
}

func TestPublishRejectedFileIsNonFatal(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// One file Photos rejects, one it accepts, in the same batch.
	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutDerivative("hash-b", "2026-06-01--12-05-00-DSCF1235", "jpeg", "/b.heic"); err != nil {
		t.Fatal(err)
	}
	lib := &fakeLibrary{rejects: map[string]bool{"/a.heic": true}}
	failed, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, "", false)
	if err != nil {
		t.Fatalf("a rejected file must not abort the run: %v", err)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1 (the rejected file)", failed)
	}
	un, err := idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 1 || un[0].SourceHash != "hash-a" {
		t.Errorf("only the rejected file must stay unpublished, got %d unpublished %v", len(un), un)
	}
	if len(lib.imported) != 1 || lib.imported[0] != "/b.heic" {
		t.Errorf("the accepted file must still import, got %v", lib.imported)
	}
}

func TestPublishWholeBatchFailureAborts(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	// A top-level ImportBatch failure means osxphotos couldn't run — abort so
	// the user fixes the environment and re-runs (publish is resumable).
	lib := &fakeLibrary{importErr: errors.New("osxphotos: command not found")}
	if _, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, "", false); err == nil {
		t.Fatal("a whole-batch osxphotos failure must abort the run")
	}
}

func TestPublishBatches(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	for i := 0; i < 5; i++ {
		stem := fmt.Sprintf("2026-06-01--12-0%d-00-DSCF120%d", i, i)
		if err := idx.PutDerivative(fmt.Sprintf("hash-%d", i), stem, "jpeg", "/"+stem+".heic"); err != nil {
			t.Fatal(err)
		}
	}
	lib := &fakeLibrary{}
	if _, err := publish(idx, lib, discard, false, time.Time{}, 2, 0, "", false); err != nil {
		t.Fatal(err)
	}
	if len(lib.imported) != 5 {
		t.Errorf("all 5 derivatives must import, got %d", len(lib.imported))
	}
	// 5 files at batch size 2 => three import calls (2, 2, 1).
	if lib.batches != 3 {
		t.Errorf("batches = %d, want 3", lib.batches)
	}
}

func TestPublishStageHardlinks(t *testing.T) {
	dir := t.TempDir()
	idx, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	exportRoot := filepath.Join(dir, "Export")
	stageDir := filepath.Join(dir, "stage")

	// Real files so os.Link has something to point at; stageTarget derives
	// YYYY/MM from the last two path components.
	mk := func(stem string) string {
		p := filepath.Join(exportRoot, "2026", "06", stem+".heic")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("heic-"+stem), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := "2026-06-01--12-00-00-DSCF1234"
	b := "2026-06-01--12-05-00-DSCF1235"
	pub := "2026-06-01--12-10-00-DSCF1236"
	pa := mk(a)
	mk(b)
	if err := idx.PutDerivative("hash-a", a, "jpeg", pa); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutDerivative("hash-b", b, "jpeg", filepath.Join(exportRoot, "2026", "06", b+".heic")); err != nil {
		t.Fatal(err)
	}
	// Already-published derivative must not be staged.
	if err := idx.PutDerivative("hash-pub", pub, "jpeg", mk(pub)); err != nil {
		t.Fatal(err)
	}
	if err := idx.MarkPublished("hash-pub", "already-uuid"); err != nil {
		t.Fatal(err)
	}

	lib := &fakeLibrary{} // empty manifest, no natural-key associations
	if _, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, stageDir, false); err != nil {
		t.Fatal(err)
	}

	for _, stem := range []string{a, b} {
		if _, err := os.Stat(filepath.Join(stageDir, "2026", "06", stem+".heic")); err != nil {
			t.Errorf("expected %s staged: %v", stem, err)
		}
	}
	if _, err := os.Stat(filepath.Join(stageDir, "2026", "06", pub+".heic")); !os.IsNotExist(err) {
		t.Errorf("already-published derivative must not be staged")
	}

	// It is a hardlink, not a copy: same inode as the source.
	src, _ := os.Stat(pa)
	dst, _ := os.Stat(filepath.Join(stageDir, "2026", "06", a+".heic"))
	if !os.SameFile(src, dst) {
		t.Errorf("staged file must be a hardlink to the source")
	}

	// Staging marks nothing published.
	un, err := idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 2 {
		t.Errorf("staging must not mark anything published, got %d unpublished", len(un))
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
	if _, err := publish(idx, lib, discard, false, time.Time{}, 0, 0, "", true); err != nil {
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
