package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbh/photo-management/internal/config"
	"github.com/dbh/photo-management/internal/index"
)

func TestSyncAdvancesWatermarkOnCleanRun(t *testing.T) {
	dir := t.TempDir()
	idx, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Library: libDir}

	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	lib := &fakeLibrary{}

	if err := runSync(idx, cfg, lib, time.Time{}, false, discard, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	got, ok, err := idx.LastSyncSince()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("watermark should be set after a clean run")
	}
	if !got.Truncate(24 * time.Hour).Equal(time.Now().Truncate(24 * time.Hour)) {
		t.Errorf("watermark = %v, want today", got)
	}
	if len(lib.imported) != 1 {
		t.Errorf("imported = %v, want the one derivative published", lib.imported)
	}
}

func TestSyncLeavesWatermarkOnPublishFailure(t *testing.T) {
	dir := t.TempDir()
	idx, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Library: libDir}

	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	lib := &fakeLibrary{rejects: map[string]bool{"/a.heic": true}}

	if err := runSync(idx, cfg, lib, time.Time{}, false, discard, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := idx.LastSyncSince(); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Error("watermark must not advance when publish reports a failure")
	}

	un, err := idx.UnpublishedDerivatives()
	if err != nil {
		t.Fatal(err)
	}
	if len(un) != 1 {
		t.Errorf("rejected derivative should still be unpublished for the next sync to retry, got %d unpublished", len(un))
	}
}

func TestSyncDryRunLeavesWatermarkUntouched(t *testing.T) {
	dir := t.TempDir()
	idx, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Library: libDir}

	if err := idx.PutDerivative("hash-a", "2026-06-01--12-00-00-DSCF1234", "jpeg", "/a.heic"); err != nil {
		t.Fatal(err)
	}
	lib := &fakeLibrary{}

	if err := runSync(idx, cfg, lib, time.Time{}, true, discard, false, 0, 0); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := idx.LastSyncSince(); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Error("--dry-run must never write the watermark")
	}
}

func TestSyncSetSinceSeedsWatermarkWithoutRunning(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := cmdSync([]string{"--set-since", "2026-07-01", "-L", libDir, "--db", dbPath}); err != nil {
		t.Fatal(err)
	}

	idx, err := index.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	got, ok, err := idx.LastSyncSince()
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	if !ok || !got.Equal(want) {
		t.Errorf("LastSyncSince = %v, %v, want %v, true", got, ok, want)
	}
}

func TestSyncSetSinceDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := cmdSync([]string{"--set-since", "2026-07-01", "--dry-run", "-L", libDir, "--db", dbPath}); err != nil {
		t.Fatal(err)
	}

	idx, err := index.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if _, ok, err := idx.LastSyncSince(); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Error("--set-since --dry-run must not write the watermark")
	}
}

func TestSyncSinceAndSetSinceAreMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := cmdSync([]string{"--since", "2026-07-01", "--set-since", "2026-07-01", "-L", libDir, "--db", dbPath})
	if err == nil {
		t.Fatal("--since and --set-since together must be rejected")
	}
}
