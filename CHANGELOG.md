# Changelog

## [Unreleased]

### Added

- `pm publish` gained `--since YYYY-MM-DD` to limit publishing to derivatives
  captured on or after a given date, matching `export`/`pull`.

## [0.6.0] - 2026-07-02

### Added

- `pm export` generates presentation HEIC derivatives from archive frames into
  `Export/YYYY/MM`: one base per frame (camera JPEG, or the RAF's embedded
  `JpgFromRaw` when RAW-only) plus one per baked edit, resized to a configurable
  long edge (default 4096 px, quality 70) with `DateTimeOriginal`/GPS/orientation
  carried over and `catalogKey`/`catalogStem` stamped in XMP. Runs are
  incremental via a new `derivative` index table; `--since` scopes, `--dry-run`
  previews.
- `pm publish` imports exported HEICs into Apple Photos via `osxphotos` (new
  runtime dependency), flat with no albums. Two dedup layers: derivatives
  already pushed are skipped by their recorded `photos_uuid`, and frames already
  present in Photos are matched on `DateTimeOriginal` + original filename
  against a new `photos_manifest` cache and associated instead of re-imported.
  Nothing is ever deleted or replaced; edits import as new assets.
- `pm pull` reverse-syndicates iPhone photos: `osxphotos export --update` into a
  queue directory, scoped by a `pull_devices` model allowlist (default
  `Apple iPhone 13 mini`) and excluding anything this tool published, then the
  existing import pipeline over the queue — BLAKE3 dedup and `YYYY/MM`
  organizing unchanged. Live Photos import the still only.
- `pm publish`/`pm pull` gained `--photos-library PATH` to target a specific
  Photos library instead of whatever's open (useful for testing against a
  throwaway library). `publish` verifies the pinned library matches what
  Photos.app actually has open before writing and aborts otherwise, since
  `osxphotos import` itself always writes into whichever library is open
  regardless of this flag.
- `osxphotos` errors that carry macOS's Full Disk Access failure signature now
  get a plain-language hint instead of just the raw Python traceback.

### Changed

- Renamed the tool from `photo-import` to `photo-management`; the binary is now
  `pm` and installs with `brew install danhorst/tap/pm`. The GitHub repo moved to
  `danhorst/photo-management` (old URLs redirect).
- The config file moved to `~/.config/photo-management/photo-management.toml`
  and the card marker to `.photo-management.toml`. Old-named configs and card
  markers are still read; stamps and saves write the new names, and the first
  config save migrates the old file.

## [0.5.1] - 2026-06-28

### Changed

- `media list` renders volumes as a bordered table instead of plain columns.

## [0.5.0] - 2026-06-27

### Added

- `media list` prints every cached volume (name, id, file count, last seen), including caches created before this change.
- `media clear [<id>…]` removes a volume's skip-cache entries by exact id or unambiguous prefix; with no arguments on a terminal, shows an interactive multiselect.
- Each import now records the source volume's human-readable name (mount-point label) and last-seen time in a new `volumes` table, so the cache can be presented by name rather than opaque id.

## [0.4.0] - 2026-06-27

### Added

- Re-importing a memory card that still holds already-imported files now skips
  them by size and modification time without re-hashing, making repeat imports of
  an un-wiped card near-instant. Each card is identified by a `.photo-import.toml`
  marker stamped at its volume root and tracked in a `media_files` table.
- `import` shows a per-file progress bar on a terminal and reports elapsed time
  in its summary.

### Performance

- `import` reads capture dates from a long-running `exiftool -stay_open` daemon
  instead of one batched call, avoiding repeated process startup.

## [0.3.0] - 2026-06-23

### Added

- `index` shows a scan spinner and a hashing progress bar (with percentage and
  ETA) on a terminal. Suppressed when piped or under `--debug`.

### Performance

- `index` writes are batched into transactions instead of one autocommit per
  file, with `synchronous=NORMAL` and `busy_timeout=5000` pragmas, cutting fsync
  overhead on the bulk build while staying resumable.

## [0.2.0] - 2026-06-22

### Added

- `config` subcommand to read and write the config file: `config path`,
  `config show`, `config init`, `config get <key>`, and `config set <key> <value>`
  for `library` and `database`.

## [0.1.0] - 2026-06-22

### Added

- Initial Go rewrite. `photo-import <source>` organizes media into the
  `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original>` library layout, skipping content
  duplicates via a BLAKE3 hash index stored in SQLite.
- `index` builds/refreshes the content-hash index; `stats` reports it.
- `--debug` activity log and `--dry-run` preview.
- TOML configuration at `~/.config/photo-import/photo-import.toml` with
  `--library`/`-L` and `--db` overrides.

[Unreleased]: https://github.com/danhorst/photo-management/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/danhorst/photo-management/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/danhorst/photo-management/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/danhorst/photo-management/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/danhorst/photo-management/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/danhorst/photo-management/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/danhorst/photo-management/compare/v0.1.0...v0.2.0
