package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathHonorsXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := filepath.Join(dir, "photo-management", "photo-management.toml")
	if got := Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestSaveLoadFileRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var c Config
	if err := c.Set("library", "/Volumes/Archive"); err != nil {
		t.Fatal(err)
	}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}

	got, err := LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Library != "/Volumes/Archive" {
		t.Errorf("library = %q, want /Volumes/Archive", got.Library)
	}
	if got.Database != "" {
		t.Errorf("database should be unset (omitempty), got %q", got.Database)
	}
}

func TestLoadFileFallsBackToLegacyPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	legacy := legacyPath()
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("library = \"/Volumes/Old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Library != "/Volumes/Old" {
		t.Errorf("library = %q, want the legacy config's value", got.Library)
	}
}

func TestSaveMigratesLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	legacy := legacyPath()
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("library = \"/Volumes/Old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Errorf("Save must write the new path: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("Save must remove the legacy config")
	}

	got, err := LoadFile()
	if err != nil {
		t.Fatal(err)
	}
	if got.Library != "/Volumes/Old" {
		t.Errorf("library = %q after migration, want /Volumes/Old", got.Library)
	}
}

func TestLoadDerivesDatabaseFromLibrary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var c Config
	c.Set("library", "/Volumes/Archive")
	if err := Save(c); err != nil {
		t.Fatal(err)
	}

	resolved, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/Volumes/Archive", indexFileName)
	if resolved.Database != want {
		t.Errorf("database = %q, want %q", resolved.Database, want)
	}
}

func TestLoadFileMissingIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := LoadFile()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if got != (Config{}) {
		t.Errorf("missing file should yield zero Config, got %+v", got)
	}
}

func TestSetUnknownKey(t *testing.T) {
	var c Config
	if err := c.Set("bogus", "x"); err == nil {
		t.Error("expected error for unknown key")
	}
}
