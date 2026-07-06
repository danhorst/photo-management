package index

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
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

func TestVolumeRegistry(t *testing.T) {
	idx := open(t)

	// seed two volumes in media_files
	if err := idx.PutMedia("aabb1122", "DCIM/a.raf", 10, 100); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutMedia("ccdd3344", "DCIM/b.raf", 20, 200); err != nil {
		t.Fatal(err)
	}

	// PutVolume registers names
	if err := idx.PutVolume("aabb1122", "EOS_DIGITAL"); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutVolume("ccdd3344", "SD_CARD"); err != nil {
		t.Fatal(err)
	}

	vols, err := idx.Volumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 2 {
		t.Fatalf("len(Volumes) = %d, want 2", len(vols))
	}
	// Labels round-trip correctly.
	labels := map[string]string{}
	for _, v := range vols {
		labels[v.VolumeID] = v.Label
		if v.FileCount != 1 {
			t.Errorf("volume %s: FileCount = %d, want 1", v.VolumeID, v.FileCount)
		}
	}
	if labels["aabb1122"] != "EOS_DIGITAL" {
		t.Errorf("label aabb1122 = %q, want EOS_DIGITAL", labels["aabb1122"])
	}

	// Re-PutVolume updates last_seen without duplicating the row.
	if err := idx.PutVolume("aabb1122", "EOS_DIGITAL"); err != nil {
		t.Fatal(err)
	}
	vols2, err := idx.Volumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(vols2) != 2 {
		t.Errorf("after re-PutVolume: len = %d, want 2", len(vols2))
	}

	// ClearVolume removes media_files rows and volumes row.
	n, err := idx.ClearVolume("aabb1122")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("ClearVolume rows = %d, want 1", n)
	}
	vols3, err := idx.Volumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(vols3) != 1 {
		t.Errorf("after ClearVolume: len = %d, want 1", len(vols3))
	}
	if vols3[0].VolumeID != "ccdd3344" {
		t.Errorf("remaining volume = %q, want ccdd3344", vols3[0].VolumeID)
	}
}

func TestVolumesOrderedByLastSeen(t *testing.T) {
	idx := open(t)
	for _, id := range []string{"older", "newer"} {
		if err := idx.PutMedia(id, "DCIM/x.raf", 10, 100); err != nil {
			t.Fatal(err)
		}
	}
	// Stamp last_seen explicitly so ordering does not depend on wall-clock ties.
	if _, err := idx.db.Exec(`INSERT INTO volumes(volume_id, label, last_seen) VALUES('older','',100),('newer','',200)`); err != nil {
		t.Fatal(err)
	}

	vols, err := idx.Volumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 2 || vols[0].VolumeID != "newer" || vols[1].VolumeID != "older" {
		t.Errorf("order = %v; want [newer older] (last_seen desc)", []string{vols[0].VolumeID, vols[1].VolumeID})
	}
}

func TestVolumesUnnamedStillAppear(t *testing.T) {
	idx := open(t)
	// volume with no volumes row (pre-naming cache)
	if err := idx.PutMedia("oldvol", "DCIM/x.raf", 10, 100); err != nil {
		t.Fatal(err)
	}
	vols, err := idx.Volumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 1 {
		t.Fatalf("len = %d, want 1", len(vols))
	}
	if vols[0].Label != "" {
		t.Errorf("label = %q, want empty", vols[0].Label)
	}
	if vols[0].FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", vols[0].FileCount)
	}
}

func TestDerivativeRoundTrip(t *testing.T) {
	idx := open(t)

	has, err := idx.HasDerivative("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("HasDerivative should be false before Put")
	}

	if err := idx.PutDerivative("abc123", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/lib/Export/2026/06/x.heic"); err != nil {
		t.Fatal(err)
	}
	has, err = idx.HasDerivative("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("HasDerivative should be true after Put")
	}

	ds, err := idx.Derivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 {
		t.Fatalf("got %d derivatives, want 1", len(ds))
	}
	d := ds[0]
	if d.SourceHash != "abc123" || d.Stem != "2026-06-01--12-00-00-DSCF1234" ||
		d.SourceKind != "jpeg" || d.HeicPath != "/lib/Export/2026/06/x.heic" {
		t.Errorf("round-trip mismatch: %+v", d)
	}
	if d.GeneratedAt.IsZero() {
		t.Error("generated_at should be set")
	}
	if d.PhotosUUID != "" || !d.PublishedAt.IsZero() {
		t.Error("publish columns should start null")
	}
}

func TestDerivativeRepeatHashUpserts(t *testing.T) {
	idx := open(t)

	if err := idx.PutDerivative("abc123", "stem", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutDerivative("abc123", "stem", "jpeg", "/b.heic"); err != nil {
		t.Fatal(err)
	}
	ds, err := idx.Derivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 {
		t.Fatalf("repeat source_hash duplicated: %d rows", len(ds))
	}
	if ds[0].HeicPath != "/b.heic" {
		t.Errorf("upsert should take the latest path, got %s", ds[0].HeicPath)
	}
}

func TestManifestUpsertAndNaturalKeyLookup(t *testing.T) {
	idx := open(t)
	when := time.Date(2026, 6, 1, 12, 0, 0, 0, time.Local)

	if err := idx.PutManifest("uuid-1", "DSCF1234.JPG", when, ""); err != nil {
		t.Fatal(err)
	}
	// Repeat uuid updates in place.
	if err := idx.PutManifest("uuid-1", "DSCF1234.JPG", when, "key1"); err != nil {
		t.Fatal(err)
	}

	uuid, found, err := idx.ManifestLookup("DSCF1234", when)
	if err != nil {
		t.Fatal(err)
	}
	if !found || uuid != "uuid-1" {
		t.Errorf("lookup = %q %v, want uuid-1 (name matched without extension)", uuid, found)
	}

	uuid, found, err = idx.ManifestLookup("dscf1234", when)
	if err != nil {
		t.Fatal(err)
	}
	if !found || uuid != "uuid-1" {
		t.Errorf("lookup should be case-insensitive, got %q %v", uuid, found)
	}

	if _, found, _ := idx.ManifestLookup("DSCF1234", when.Add(time.Second)); found {
		t.Error("different capture time should not match")
	}
	if _, found, _ := idx.ManifestLookup("DSCF9999", when); found {
		t.Error("different name should not match")
	}
}

func TestMarkPublishedAndUnpublished(t *testing.T) {
	idx := open(t)

	if err := idx.PutDerivative("hash-a", "stem-a", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	if err := idx.PutDerivative("hash-b", "stem-b", "edit", "/b.heic"); err != nil {
		t.Fatal(err)
	}

	un, err := idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 2 {
		t.Fatalf("got %d unpublished, want 2", len(un))
	}

	if err := idx.MarkPublished("hash-a", "photos-uuid-a"); err != nil {
		t.Fatal(err)
	}
	un, err = idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 1 || un[0].SourceHash != "hash-b" {
		t.Errorf("unpublished after mark = %+v, want only hash-b", un)
	}

	ds, err := idx.Derivatives()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.SourceHash == "hash-a" && (d.PhotosUUID != "photos-uuid-a" || d.PublishedAt.IsZero()) {
			t.Errorf("mark published not recorded: %+v", d)
		}
	}
}

func TestSyncSinceRoundTrip(t *testing.T) {
	idx := open(t)

	if _, ok, err := idx.LastSyncSince(); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Error("LastSyncSince should report not-ok before any sync has run")
	}

	first := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	if err := idx.SetSyncSince(first); err != nil {
		t.Fatal(err)
	}
	got, ok, err := idx.LastSyncSince()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !got.Equal(first) {
		t.Errorf("LastSyncSince = %v, %v, want %v, true", got, ok, first)
	}

	// A repeat call upserts in place rather than erroring on the singleton row.
	second := time.Date(2026, 7, 10, 0, 0, 0, 0, time.Local)
	if err := idx.SetSyncSince(second); err != nil {
		t.Fatal(err)
	}
	got, ok, err = idx.LastSyncSince()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !got.Equal(second) {
		t.Errorf("LastSyncSince after second set = %v, %v, want %v, true", got, ok, second)
	}
}
