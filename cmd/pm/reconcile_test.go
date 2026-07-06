package main

import (
	"path/filepath"
	"testing"

	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
)

func seedPublished(t *testing.T, idx *index.Index, hash, stem, uuid string) {
	t.Helper()
	if err := idx.PutDerivative(hash, stem, "jpeg", "/Export/"+stem+".heic"); err != nil {
		t.Fatal(err)
	}
	if err := idx.MarkPublished(hash, uuid); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileClearsMissing(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	seedPublished(t, idx, "hash-present", "2005-06-01--00-00-00-A", "UUID-PRESENT")
	seedPublished(t, idx, "hash-gone", "2005-06-02--00-00-00-B", "UUID-GONE")

	lib := &fakeLibrary{assets: []photos.Asset{{UUID: "UUID-PRESENT"}}}

	// Dry run changes nothing.
	if err := reconcile(idx, lib, discard, false, true); err != nil {
		t.Fatal(err)
	}
	if pub, _ := idx.PublishedDerivatives(); len(pub) != 2 {
		t.Fatalf("dry-run must not clear anything, got %d published", len(pub))
	}

	// Apply: the gone asset is cleared, the present one kept.
	if err := reconcile(idx, lib, discard, false, false); err != nil {
		t.Fatal(err)
	}
	pub, err := idx.PublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].SourceHash != "hash-present" {
		t.Fatalf("only the present asset must stay published, got %v", pub)
	}
	un, _ := idx.UnpublishedDerivatives()
	if len(un) != 1 || un[0].SourceHash != "hash-gone" {
		t.Fatalf("the gone asset must be re-queued for publish, got %v", un)
	}
}

func TestReconcileRefusesEmptyManifest(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	seedPublished(t, idx, "hash-a", "2005-06-01--00-00-00-A", "UUID-A")

	lib := &fakeLibrary{} // empty manifest
	if err := reconcile(idx, lib, discard, false, false); err == nil {
		t.Fatal("an empty manifest must abort reconcile")
	}
	if pub, _ := idx.PublishedDerivatives(); len(pub) != 1 {
		t.Fatalf("nothing must be cleared when reconcile aborts, got %d published", len(pub))
	}
}
