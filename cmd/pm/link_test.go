package main

import (
	"path/filepath"
	"testing"

	"github.com/dbh/photo-management/internal/index"
	"github.com/dbh/photo-management/internal/photos"
)

func TestLinkMatchesByFilename(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// Two unpublished derivatives; only one has a Photos asset whose
	// original_filename equals its HEIC basename.
	if err := idx.PutDerivative("hash-a", "2005-06-01--00-00-00-A", "jpeg", "/Export/2005/06/2005-06-01--00-00-00-A.heic"); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutDerivative("hash-b", "2005-06-02--00-00-00-B", "jpeg", "/Export/2005/06/2005-06-02--00-00-00-B.heic"); err != nil {
		t.Fatal(err)
	}
	// osxphotos' {original_name} strips the extension, so assets carry the stem.
	lib := &fakeLibrary{assets: []photos.Asset{
		{UUID: "UUID-A", OriginalFilename: "2005-06-01--00-00-00-A"},
		{UUID: "OTHER", OriginalFilename: "IMG_9999"},
	}}

	// Dry run changes nothing.
	if err := link(idx, lib, discard, false, true); err != nil {
		t.Fatal(err)
	}
	if pub, _ := idx.PublishedDerivatives(); len(pub) != 0 {
		t.Fatalf("dry-run must not link anything, got %d published", len(pub))
	}

	// Apply: the filename-matched derivative links, the other stays unpublished.
	if err := link(idx, lib, discard, false, false); err != nil {
		t.Fatal(err)
	}
	pub, err := idx.PublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].SourceHash != "hash-a" || pub[0].PhotosUUID != "UUID-A" {
		t.Fatalf("only the filename-matched derivative must link, got %v", pub)
	}
	un, _ := idx.UnpublishedDerivatives()
	if len(un) != 1 || un[0].SourceHash != "hash-b" {
		t.Fatalf("the unmatched derivative must stay unpublished, got %v", un)
	}
}

func TestLinkRefusesEmptyManifest(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	if err := idx.PutDerivative("hash-a", "2005-06-01--00-00-00-A", "jpeg", "/Export/2005/06/2005-06-01--00-00-00-A.heic"); err != nil {
		t.Fatal(err)
	}

	lib := &fakeLibrary{} // empty manifest
	if err := link(idx, lib, discard, false, false); err == nil {
		t.Fatal("an empty manifest must abort link")
	}
	if un, _ := idx.UnpublishedDerivatives(); len(un) != 1 {
		t.Fatalf("nothing must change when link aborts, got %d unpublished", len(un))
	}
}

func TestLinkSkipsAmbiguousFilename(t *testing.T) {
	idx, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	if err := idx.PutDerivative("hash-a", "2005-06-01--00-00-00-A", "jpeg", "/Export/2005/06/2005-06-01--00-00-00-A.heic"); err != nil {
		t.Fatal(err)
	}
	// Two assets claim the same filename — a duplicate slipped into Photos.
	lib := &fakeLibrary{assets: []photos.Asset{
		{UUID: "U1", OriginalFilename: "2005-06-01--00-00-00-A"},
		{UUID: "U2", OriginalFilename: "2005-06-01--00-00-00-A"},
	}}
	if err := link(idx, lib, discard, false, false); err != nil {
		t.Fatal(err)
	}
	if pub, _ := idx.PublishedDerivatives(); len(pub) != 0 {
		t.Fatalf("an ambiguous filename must not link, got %d published", len(pub))
	}
}
