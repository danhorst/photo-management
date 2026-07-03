package export

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	// DefaultLongEdge is the derivative's long-edge pixel size when the config
	// does not override it.
	DefaultLongEdge = 4096
	// DefaultQuality is the HEIC encode quality when the config does not
	// override it.
	DefaultQuality = 70
)

// exiftoolConfig defines the XMP namespace for the catalogKey/catalogStem
// identity tags, so they can be written and survive renaming.
const exiftoolConfig = `%Image::ExifTool::UserDefined = (
    'Image::ExifTool::XMP::Main' => {
        PhotoManagement => {
            SubDirectory => {
                TagTable => 'Image::ExifTool::UserDefined::PhotoManagement',
            },
        },
    },
);
%Image::ExifTool::UserDefined::PhotoManagement = (
    GROUPS => { 0 => 'XMP', 1 => 'XMP-PhotoManagement', 2 => 'Image' },
    NAMESPACE => { 'PhotoManagement' => 'https://danhorst.com/ns/photo-management/1.0/' },
    WRITABLE => 'string',
    CatalogKey => {},
    CatalogStem => {},
);
1;
`

// Generator transcodes derivative sources to presentation HEICs via sips and
// stamps identity and carried metadata via exiftool. Safe for concurrent use
// by multiple goroutines: Generate has no other shared mutable state once the
// exiftool config is materialized.
type Generator struct {
	LongEdge int
	Quality  int

	configOnce sync.Once
	configPath string // materialized exiftool config, created lazily
	configErr  error
}

// Close removes the materialized exiftool config, if any. Call only after
// every Generate call has returned.
func (g *Generator) Close() {
	if g.configPath != "" {
		os.Remove(g.configPath)
		g.configPath = ""
	}
}

func (g *Generator) config() (string, error) {
	g.configOnce.Do(func() {
		f, err := os.CreateTemp("", "photo-management-exiftool-*.config")
		if err != nil {
			g.configErr = err
			return
		}
		if _, err := f.WriteString(exiftoolConfig); err != nil {
			f.Close()
			os.Remove(f.Name())
			g.configErr = err
			return
		}
		if err := f.Close(); err != nil {
			os.Remove(f.Name())
			g.configErr = err
			return
		}
		g.configPath = f.Name()
	})
	return g.configPath, g.configErr
}

// Generate produces the HEIC at dst from src, carrying
// DateTimeOriginal/GPS/orientation from the archive file and stamping
// catalogKey (the version id) and catalogStem (the frame id). An embedded
// source extracts the RAF's JpgFromRaw first.
func (g *Generator) Generate(src Source, stem, versionID, dst string) error {
	input := src.Path
	if src.Kind == "embedded" {
		tmp, err := extractEmbedded(src.Path)
		if err != nil {
			return err
		}
		defer os.Remove(tmp)
		input = tmp
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := exec.Command("sips",
		"-s", "format", "heic",
		"-s", "formatOptions", strconv.Itoa(g.Quality),
		"--resampleHeightWidthMax", strconv.Itoa(g.LongEdge),
		input, "--out", dst,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sips %s: %v: %s", src.Path, err, out)
	}

	cfg, err := g.config()
	if err != nil {
		return err
	}
	out, err = exec.Command("exiftool",
		"-config", cfg,
		"-overwrite_original", "-q",
		"-tagsFromFile", src.Path,
		"-DateTimeOriginal", "-CreateDate", "-OffsetTimeOriginal", "-Orientation", "-gps:all",
		"-XMP-PhotoManagement:CatalogKey="+versionID,
		"-XMP-PhotoManagement:CatalogStem="+stem,
		dst,
	).CombinedOutput()
	if err != nil {
		os.Remove(dst)
		return fmt.Errorf("exiftool %s: %v: %s", dst, err, out)
	}
	return nil
}

// extractEmbedded writes the RAF's embedded JpgFromRaw to a temp file and
// returns its path.
func extractEmbedded(rafPath string) (string, error) {
	f, err := os.CreateTemp("", "photo-management-embedded-*.jpg")
	if err != nil {
		return "", err
	}
	cmd := exec.Command("exiftool", "-b", "-JpgFromRaw", rafPath)
	cmd.Stdout = f
	runErr := cmd.Run()
	closeErr := f.Close()
	if runErr != nil || closeErr != nil {
		os.Remove(f.Name())
		if runErr != nil {
			return "", fmt.Errorf("extracting JpgFromRaw from %s: %v", rafPath, runErr)
		}
		return "", closeErr
	}
	if fi, err := os.Stat(f.Name()); err != nil || fi.Size() == 0 {
		os.Remove(f.Name())
		return "", fmt.Errorf("no JpgFromRaw in %s", rafPath)
	}
	return f.Name(), nil
}

// DestPath returns Export/YYYY/MM/<stem>.heic for the base or
// <stem>-<suffix>.heic for an edit, under the library root.
func DestPath(library string, f Frame, src Source) string {
	name := f.Stem
	if src.Suffix != "" {
		name += "-" + src.Suffix
	}
	return filepath.Join(library, "Export", f.Stem[:4], f.Stem[5:7], name+".heic")
}
