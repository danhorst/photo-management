package export

import (
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestGeneratorConfigConcurrent(t *testing.T) {
	g := &Generator{}
	defer g.Close()

	var wg sync.WaitGroup
	paths := make([]string, 20)
	errs := make([]error, 20)
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = g.config()
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if paths[i] == "" || paths[i] != paths[0] {
			t.Fatalf("call %d returned %q, want the same path every call", i, paths[i])
		}
	}
}

// writeJPEG writes a tiny solid-color JPEG. A plain JPEG carries no
// PreviewImage/JpgFromRaw, so it stands in for a master with no embedded JPEG.
func writeJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
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

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available", name)
	}
}

func TestExtractEmbeddedNoJPEG(t *testing.T) {
	requireBinary(t, "exiftool")

	src := filepath.Join(t.TempDir(), "master.jpg")
	writeJPEG(t, src)

	if _, err := extractEmbedded(src); !errors.Is(err, errNoEmbeddedJPEG) {
		t.Fatalf("extractEmbedded = %v, want errNoEmbeddedJPEG", err)
	}
}

func TestGenerateFallbackRendersMaster(t *testing.T) {
	requireBinary(t, "exiftool")
	requireBinary(t, "sips")

	dir := t.TempDir()
	src := filepath.Join(dir, "master.jpg") // no embedded JPEG
	writeJPEG(t, src)
	dst := filepath.Join(dir, "out.heic")

	g := &Generator{LongEdge: DefaultLongEdge, Quality: DefaultQuality}
	defer g.Close()

	err := g.Generate(Source{Kind: "embedded", Path: src}, "2020-01-01--00-00-00-master", "v1", dst)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if fi, err := os.Stat(dst); err != nil || fi.Size() == 0 {
		t.Fatalf("no HEIC produced: stat=%v", err)
	}
}
