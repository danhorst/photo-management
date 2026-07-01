# Changelog

## [Unreleased]

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

[Unreleased]: https://github.com/danhorst/photo-management/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/danhorst/photo-management/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/danhorst/photo-management/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/danhorst/photo-management/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/danhorst/photo-management/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/danhorst/photo-management/compare/v0.1.0...v0.2.0
