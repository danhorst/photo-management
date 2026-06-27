package volume

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMarkerGeneratesThenReuses(t *testing.T) {
	root := t.TempDir()

	id, m, existed, err := loadMarker(root)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Error("existed should be false on a fresh volume")
	}
	if id == "" || m.VolumeID != id {
		t.Fatalf("got id %q, marker %q; want a generated id", id, m.VolumeID)
	}
	if _, err := os.Stat(filepath.Join(root, markerName)); !os.IsNotExist(err) {
		t.Error("loadMarker must not write the marker")
	}

	if err := Stamp(root, m); err != nil {
		t.Fatal(err)
	}

	id2, _, existed2, err := loadMarker(root)
	if err != nil {
		t.Fatal(err)
	}
	if !existed2 {
		t.Error("existed should be true after Stamp")
	}
	if id2 != id {
		t.Errorf("id changed across reads: %q then %q", id, id2)
	}
}

func TestVolumeRootContainsSource(t *testing.T) {
	sub := filepath.Join(t.TempDir(), "DCIM", "100_FUJI")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Best-effort: under a single test filesystem the walk climbs to the mount
	// point, which must still be an ancestor of the source.
	root, err := volumeRoot(sub)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(sub)
	if !strings.HasPrefix(abs, root) {
		t.Errorf("volumeRoot = %q is not an ancestor of %q", root, abs)
	}
}
