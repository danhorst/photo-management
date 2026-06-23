# Changelog

## [Unreleased]

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

[Unreleased]: https://github.com/danhorst/photo-import/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/danhorst/photo-import/compare/v0.1.0...v0.2.0
