// Package organize computes destination paths in the YYYY/MM library layout and
// places files there, moving within a filesystem and copying across.
package organize

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Precision is how much of a capture date a canonical stem carries. Frames with
// no reliable time (film scans, hand-filed frames) drop the unknown trailing
// fields rather than fabricate them.
type Precision int

const (
	Second Precision = iota // YYYY-MM-DD--HH-MM-SS-<original>
	Day                     // YYYY-MM-DD-<original>
	Month                   // YYYY-MM-<original>
)

const (
	secondLayout = "2006-01-02--15-04-05"
	dayLayout    = "2006-01-02"
	monthLayout  = "2006-01"
)

// Stem returns the canonical stem for t at the given precision, with name (the
// original filename, extension included) appended after the date prefix.
func Stem(t time.Time, prec Precision, name string) string {
	switch prec {
	case Day:
		return t.Format(dayLayout) + "-" + name
	case Month:
		return t.Format(monthLayout) + "-" + name
	default:
		return t.Format(secondLayout) + "-" + name
	}
}

// ParseStem parses a canonical stem into its capture time, precision, and
// original name. It tries second, then day, then month precision, so the
// longest matching form wins; a stem with no leading date fails.
func ParseStem(stem string) (t time.Time, prec Precision, original string, ok bool) {
	// Second: YYYY-MM-DD--HH-MM-SS-<orig>, distinguished by the "--" at 10.
	if len(stem) > len(secondLayout)+1 && stem[10] == '-' && stem[11] == '-' && stem[len(secondLayout)] == '-' {
		if tt, err := time.ParseInLocation(secondLayout, stem[:len(secondLayout)], time.Local); err == nil {
			return tt, Second, stem[len(secondLayout)+1:], true
		}
	}
	// Day: YYYY-MM-DD-<orig>, a single dash after the date.
	if len(stem) > len(dayLayout)+1 && stem[10] == '-' && stem[11] != '-' {
		if tt, err := time.ParseInLocation(dayLayout, stem[:len(dayLayout)], time.Local); err == nil {
			return tt, Day, stem[len(dayLayout)+1:], true
		}
	}
	// Month: YYYY-MM-<orig>, reached only when the day form did not match.
	if len(stem) > len(monthLayout)+1 && stem[len(monthLayout)] == '-' {
		if tt, err := time.ParseInLocation(monthLayout, stem[:len(monthLayout)], time.Local); err == nil {
			return tt, Month, stem[len(monthLayout)+1:], true
		}
	}
	return time.Time{}, Second, "", false
}

// DestAt returns <library>/YYYY/MM/<stem> for t at the given precision.
func DestAt(library string, t time.Time, prec Precision, origName string) string {
	return filepath.Join(library,
		fmt.Sprintf("%04d", t.Year()),
		fmt.Sprintf("%02d", int(t.Month())),
		Stem(t, prec, origName),
	)
}

// Dest returns <library>/YYYY/MM/YYYY-MM-DD--HH-MM-SS-<origName>.
func Dest(library string, t time.Time, origName string) string {
	return DestAt(library, t, Second, origName)
}

// Place moves src to dst when they share a filesystem, otherwise copies src to
// dst (preserving mtime) and leaves src in place. It creates dst's parent
// directory. The returned bool reports whether the file was moved.
func Place(src, dst string) (moved bool, err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	if err := os.Rename(src, dst); err == nil {
		return true, nil
	} else if !isCrossDevice(err) {
		return false, err
	}
	return false, copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if fi, err := os.Stat(src); err == nil {
		_ = os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	}
	return nil
}

func isCrossDevice(err error) bool {
	var le *os.LinkError
	if errors.As(err, &le) {
		return errors.Is(le.Err, syscall.EXDEV)
	}
	return false
}
