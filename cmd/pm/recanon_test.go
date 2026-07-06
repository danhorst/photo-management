package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dbh/photo-management/internal/index"
)

func writeStub(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lsNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// muteStdout silences the command's progress prints for the duration of a test.
func muteStdout(t *testing.T) {
	t.Helper()
	orig := os.Stdout
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f
	t.Cleanup(func() { os.Stdout = orig; f.Close() })
}

func TestRecanon(t *testing.T) {
	muteStdout(t)
	lib := t.TempDir()
	db := filepath.Join(t.TempDir(), "idx.db")
	dir := filepath.Join(lib, "2021", "12")

	zf := []string{
		"IMG_0003-ZF-9821-41309-1-001-004.jpg",
		"IMG_0005-ZF-9821-41309-1-001-002.jpg",
	}
	for _, n := range zf {
		writeStub(t, filepath.Join(dir, n))
	}
	// An already-canonical frame and an Unsorted file must be left alone.
	writeStub(t, filepath.Join(dir, "2021-12-05--10-00-00-DSCF1.jpg"))
	writeStub(t, filepath.Join(lib, "Unsorted", "random.jpg"))

	// Seed the index for one straggler to prove the path row follows the rename.
	idx, err := index.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, zf[0])
	if err := idx.Put(old, 1, 1, "hash0"); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Dry run changes nothing on disk.
	if err := cmdRecanon([]string{"-L", lib, "--db", db, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("dry-run should not rename: %v", err)
	}

	// Apply.
	if err := cmdRecanon([]string{"-L", lib, "--db", db}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2021-12-IMG_0003-ZF-9821-41309-1-001-004.jpg",
		"2021-12-IMG_0005-ZF-9821-41309-1-001-002.jpg",
		"2021-12-05--10-00-00-DSCF1.jpg",
	}
	sort.Strings(want)
	if got := withoutXMP(lsNames(t, dir)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after recanon = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(lib, "Unsorted", "random.jpg")); err != nil {
		t.Errorf("Unsorted file should be untouched: %v", err)
	}

	// The index path row followed the rename.
	idx, err = index.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "2021-12-IMG_0003-ZF-9821-41309-1-001-004.jpg")
	if p, found, err := idx.Lookup("hash0"); err != nil || !found || p != newPath {
		t.Errorf("index path = %q found=%v err=%v, want %q", p, found, err, newPath)
	}
	idx.Close()

	// Idempotent: a second run finds nothing, so no double prefix appears.
	if err := cmdRecanon([]string{"-L", lib, "--db", db}); err != nil {
		t.Fatal(err)
	}
	if got := withoutXMP(lsNames(t, dir)); !reflect.DeepEqual(got, want) {
		t.Fatalf("second run changed names to %v", got)
	}
}

func TestRecanonDayPrecisionFromFlag(t *testing.T) {
	muteStdout(t)
	lib := t.TempDir()
	db := filepath.Join(t.TempDir(), "idx.db")
	dir := filepath.Join(lib, "2021", "12")
	writeStub(t, filepath.Join(dir, "IMG_0003-ZF-9821.jpg"))

	if err := cmdRecanon([]string{"-L", lib, "--db", db, "--match", "ZF-9821", "--date", "2021-12-05"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2021-12-05-IMG_0003-ZF-9821.jpg")); err != nil {
		t.Errorf("day-precision rename missing: %v", err)
	}
}

func TestRecanonSidecars(t *testing.T) {
	requireBinary(t, "exiftool")
	muteStdout(t)
	lib := t.TempDir()
	db := filepath.Join(t.TempDir(), "idx.db")
	dir := filepath.Join(lib, "2005", "06")

	// An undated scan gets a sidecar; a frame that already embeds a date does not.
	undated := filepath.Join(dir, "scan-777.jpg")
	writeRealJPEG(t, undated)
	dated := filepath.Join(dir, "cam-888.jpg")
	writeRealJPEG(t, dated)
	stampDate(t, dated, "2018:03:03 09:09:09")

	// Dry run writes no sidecar.
	if err := cmdRecanon([]string{"-L", lib, "--db", db, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range lsNames(t, dir) {
		if strings.HasSuffix(n, ".xmp") {
			t.Fatalf("dry-run wrote a sidecar: %s", n)
		}
	}

	if err := cmdRecanon([]string{"-L", lib, "--db", db}); err != nil {
		t.Fatal(err)
	}

	// Undated file: sidecar carries the month-precision stem date; the original's
	// own bytes stay undated (never rewritten).
	sidecar := filepath.Join(dir, "2005-06-scan-777.xmp")
	if got := readDate(t, sidecar); got != "2005:06:01 00:00:00" {
		t.Errorf("sidecar date = %q, want month-precision 2005:06:01 00:00:00", got)
	}
	if got := readDate(t, filepath.Join(dir, "2005-06-scan-777.jpg")); got != "" {
		t.Errorf("original bytes should stay undated, got embedded date %q", got)
	}

	// Dated file: renamed, but no sidecar (its embedded date is authoritative).
	if _, err := os.Stat(filepath.Join(dir, "2005-06-cam-888.xmp")); !os.IsNotExist(err) {
		t.Errorf("a file that already embeds a date must not get a sidecar")
	}
}

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available", name)
	}
}

// writeRealJPEG writes a tiny valid JPEG carrying no EXIF capture date.
func writeRealJPEG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// stampDate writes a real DateTimeOriginal into the file's own EXIF.
func stampDate(t *testing.T, path, date string) {
	t.Helper()
	if out, err := exec.Command("exiftool", "-q", "-overwrite_original",
		"-DateTimeOriginal="+date, path).CombinedOutput(); err != nil {
		t.Fatalf("stamp %s: %v: %s", path, err, out)
	}
}

// readDate returns DateTimeOriginal (from a media file or an XMP sidecar), or "".
func readDate(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("exiftool", "-s3", "-d", "%Y:%m:%d %H:%M:%S",
		"-DateTimeOriginal", path).Output()
	if err != nil {
		t.Fatalf("read date %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

func withoutXMP(names []string) []string {
	var out []string
	for _, n := range names {
		if !strings.HasSuffix(n, ".xmp") {
			out = append(out, n)
		}
	}
	return out
}
