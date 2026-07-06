## Why

A file with no EXIF capture date (a scanned photo, a stripped image) is quarantined to `Unsorted/` at import, then given a canonical stem date by `pm recanon`.
That date then lives only in the filename.
It never reaches the exported HEIC, so `publish` (which runs `osxphotos import` with no date flag) lets Apple Photos stamp the import day — a 2005 scan is filed as shot today.
It never reaches the original's metadata either, so Capture One shows the frame undated.

## What Changes

- `export` stamps the stem-derived capture date onto each derivative HEIC as a fallback, only when the source file carries no `DateTimeOriginal`/`CreateDate` of its own.
  Since `export` already skips non-canonical frames, a stem date is always available — so every exported HEIC is guaranteed to carry a real capture date.
- `recanon` writes an XMP sidecar (`<canonical-name>.xmp`) next to each archived original that lacks an embedded capture date, carrying the stem date.
  Capture One reads the sidecar's date automatically; the original's bytes are never touched, so its BLAKE3 hash is unchanged and RAW write-unsafety is sidestepped.
- Neither write ever overwrites an existing real capture date — the stem is a fallback only.

## Capabilities

### New Capabilities

- `capture-date`: the canonical capture date carried by a frame's stem is materialized into every published derivative and, for archived originals lacking embedded EXIF, into an XMP sidecar — so downstream catalogs (Apple Photos, Capture One) show the true capture date rather than the import day.

### Modified Capabilities

<!-- None: export and recanon behavior is not yet formalized in openspec/specs/, so the new guarantee is introduced as its own capability rather than a delta. -->

## Impact

- `internal/export/generate.go` (`Generate`): second, guarded exiftool call for the stem-date fallback.
- `cmd/pm/recanon.go`: sidecar write after each canonical rename; new dependency on `internal/exif` to detect an already-embedded date; respects `--dry-run`.
- Archive gains `.xmp` sidecar files, which `media.IsMedia` already excludes from import/export/indexing — no dedup or backup-hash impact.
- Preserves the project invariant that the tool only moves and names files on disk, never rewriting their contents.
- Unchanged: `import` (EXIF-dated files already embed the date; no-EXIF files have no date until recanon) and `publish` (keeps reading the HEIC's embedded EXIF).
