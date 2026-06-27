package index

import (
	"fmt"
	"path/filepath"
	"testing"
)

func open(t *testing.T) *Index {
	t.Helper()
	idx, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestBatchCrossesBoundary(t *testing.T) {
	idx := open(t)

	b, err := idx.Begin()
	if err != nil {
		t.Fatal(err)
	}
	n := batchSize + 5 // force at least one mid-batch commit
	for i := 0; i < n; i++ {
		if err := b.Put(fmt.Sprintf("/lib/f%05d", i), int64(i), 1, fmt.Sprintf("%064x", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Commit(); err != nil {
		t.Fatal(err)
	}

	count, _, err := idx.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(n) {
		t.Fatalf("count = %d, want %d", count, n)
	}

	// A row written before the batch boundary must be findable by hash.
	path, found, err := idx.Lookup(fmt.Sprintf("%064x", 0))
	if err != nil {
		t.Fatal(err)
	}
	if !found || path != "/lib/f00000" {
		t.Errorf("Lookup = %q, %v; want /lib/f00000, true", path, found)
	}
}

func TestCachedDetectsChange(t *testing.T) {
	idx := open(t)
	if err := idx.Put("/a", 10, 100, "deadbeef"); err != nil {
		t.Fatal(err)
	}
	if h, ok := idx.Cached("/a", 10, 100); !ok || h != "deadbeef" {
		t.Errorf("Cached(unchanged) = %q, %v; want deadbeef, true", h, ok)
	}
	if _, ok := idx.Cached("/a", 11, 100); ok {
		t.Error("Cached should miss when size changes")
	}
}

func TestVolumeMedia(t *testing.T) {
	idx := open(t)
	if err := idx.PutMedia("vol1", "DCIM/x.raf", 10, 100); err != nil {
		t.Fatal(err)
	}
	// Same relative path on a different card must not bleed into vol1's set.
	if err := idx.PutMedia("vol2", "DCIM/x.raf", 99, 999); err != nil {
		t.Fatal(err)
	}

	m, err := idx.VolumeMedia("vol1")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("len(VolumeMedia) = %d, want 1", len(m))
	}
	if rec, ok := m["DCIM/x.raf"]; !ok || rec.Size != 10 || rec.Mtime != 100 {
		t.Errorf("record = %+v, %v; want {10 100}, true", rec, ok)
	}
}

func TestPutMediaUpsert(t *testing.T) {
	idx := open(t)
	if err := idx.PutMedia("vol1", "x.raf", 10, 100); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutMedia("vol1", "x.raf", 20, 200); err != nil {
		t.Fatal(err)
	}
	m, err := idx.VolumeMedia("vol1")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Errorf("len = %d, want 1 (upsert, not insert)", len(m))
	}
	if rec := m["x.raf"]; rec.Size != 20 || rec.Mtime != 200 {
		t.Errorf("after upsert = %+v; want {20 200}", rec)
	}
}
