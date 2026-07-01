// Package volume identifies removable media across imports via a TOML marker
// stamped at the volume root, so files already seen on a card can be skipped
// without re-reading their contents.
package volume

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/BurntSushi/toml"
)

const (
	markerName = ".photo-management.toml"
	// legacyMarkerName is the pre-rename marker, read as a fallback so cards
	// stamped by photo-import keep their volume identity.
	legacyMarkerName = ".photo-import.toml"
)

// Marker is the per-volume record stored at the volume root.
type Marker struct {
	VolumeID string `toml:"volume_id"`
	Label    string `toml:"label,omitempty"`
}

// Resolve locates the volume root containing source and loads its marker. When
// no marker exists it returns a Marker carrying a freshly generated VolumeID and
// existed=false; the marker is written only when the caller invokes Stamp.
func Resolve(source string) (root, id string, m Marker, existed bool, err error) {
	root, err = volumeRoot(source)
	if err != nil {
		return "", "", Marker{}, false, err
	}
	id, m, existed, err = loadMarker(root)
	return root, id, m, existed, err
}

// loadMarker reads the marker at root, falling back to the legacy
// photo-import name, or returns a Marker with a freshly generated VolumeID and
// existed=false when neither is present.
func loadMarker(root string) (id string, m Marker, existed bool, err error) {
	path := filepath.Join(root, markerName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		path = filepath.Join(root, legacyMarkerName)
		data, err = os.ReadFile(path)
	}
	if err == nil {
		if _, err := toml.Decode(string(data), &m); err != nil {
			return "", Marker{}, false, fmt.Errorf("parsing %s: %w", path, err)
		}
		return m.VolumeID, m, true, nil
	}
	if !os.IsNotExist(err) {
		return "", Marker{}, false, err
	}
	gid, err := newID()
	if err != nil {
		return "", Marker{}, false, err
	}
	return gid, Marker{VolumeID: gid}, false, nil
}

// Stamp writes m to the marker file at root.
func Stamp(root string, m Marker) error {
	f, err := os.Create(filepath.Join(root, markerName))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(m)
}

// volumeRoot walks up from source while staying on the same filesystem device,
// returning the topmost same-device directory (the mount point). This keeps a
// card's identity stable whether the caller passes the card root or a subdir
// like DCIM.
func volumeRoot(source string) (string, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	dev, err := deviceID(abs)
	if err != nil {
		return "", err
	}
	root := abs
	for {
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		pdev, err := deviceID(parent)
		if err != nil || pdev != dev {
			break
		}
		root = parent
	}
	return root, nil
}

func deviceID(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("no device id for %s", path)
	}
	return uint64(st.Dev), nil
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
