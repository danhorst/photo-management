// Package exif extracts capture timestamps from media files via exiftool.
package exif

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// layout is the Go time layout used to parse exiftool's output; strftimeFmt is
// the matching strftime format passed to exiftool's -d flag.
const (
	layout      = "2006-01-02 15:04:05"
	strftimeFmt = "%Y-%m-%d %H:%M:%S"
)

type entry struct {
	SourceFile       string `json:"SourceFile"`
	DateTimeOriginal string `json:"DateTimeOriginal"`
	CreateDate       string `json:"CreateDate"`
}

func parseDate(e entry) (time.Time, bool) {
	ds := e.DateTimeOriginal
	if ds == "" {
		ds = e.CreateDate
	}
	if ds == "" || strings.HasPrefix(ds, "0000") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(layout, ds, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Dates returns capture timestamps keyed by the input path. Files whose date is
// missing or unparseable are absent from the map; callers fall back to mtime.
// The whole batch is read in one exiftool call.
func Dates(paths []string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	if len(paths) == 0 {
		return out, nil
	}

	// Pass the file list through an args file to avoid command-line length limits.
	args, err := os.CreateTemp("", "photo-import-exif-*.args")
	if err != nil {
		return nil, err
	}
	defer os.Remove(args.Name())
	for _, p := range paths {
		if _, err := args.WriteString(p + "\n"); err != nil {
			args.Close()
			return nil, err
		}
	}
	args.Close()

	cmd := exec.Command("exiftool",
		"-json", "-q",
		"-d", strftimeFmt,
		"-DateTimeOriginal", "-CreateDate",
		"-@", args.Name(),
	)
	stdout, err := cmd.Output()
	// exiftool exits non-zero when some files lack metadata but still emits JSON
	// for the rest, so only treat an empty output as a hard failure.
	if err != nil && len(stdout) == 0 {
		return nil, err
	}

	var entries []entry
	if err := json.Unmarshal(stdout, &entries); err != nil {
		return nil, err
	}
	for _, e := range entries {
		if t, ok := parseDate(e); ok {
			out[e.SourceFile] = t
		}
	}
	return out, nil
}

// Daemon wraps a long-running exiftool process in -stay_open mode,
// allowing per-file date queries without repeated process startup.
type Daemon struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
	seq    int
}

// NewDaemon starts an exiftool daemon. Callers must call Close when done.
func NewDaemon() (*Daemon, error) {
	cmd := exec.Command("exiftool", "-stay_open", "True", "-@", "-")
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, err
	}
	return &Daemon{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdinPipe),
		stdout: bufio.NewScanner(stdoutPipe),
	}, nil
}

// Date returns the capture time for path. ok is false when no date is found.
func (d *Daemon) Date(path string) (time.Time, bool) {
	d.seq++
	fmt.Fprintf(d.stdin, "-json\n-q\n-d\n%s\n-DateTimeOriginal\n-CreateDate\n%s\n-execute%d\n",
		strftimeFmt, path, d.seq)
	d.stdin.Flush()

	ready := fmt.Sprintf("{ready%d}", d.seq)
	var buf strings.Builder
	for d.stdout.Scan() {
		line := d.stdout.Text()
		if line == ready {
			break
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	text := strings.TrimSpace(buf.String())
	if text == "" {
		return time.Time{}, false
	}
	var entries []entry
	if err := json.Unmarshal([]byte(text), &entries); err != nil || len(entries) == 0 {
		return time.Time{}, false
	}
	return parseDate(entries[0])
}

// Close shuts down the daemon process.
func (d *Daemon) Close() error {
	fmt.Fprintln(d.stdin, "-stay_open\nFalse\n-execute")
	d.stdin.Flush()
	return d.cmd.Wait()
}
