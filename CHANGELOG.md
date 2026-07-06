# Changelog

## [Unreleased]

## [0.10.0] - 2026-07-06

### Performance

- The Photos manifest queries now request only the fields each command uses, via
  `osxphotos --field`, instead of a full `--json` dump that computes every
  photo's exif/album/person metadata. `reconcile` and the active-library check
  query only uuids, `publish` only uuid/filename/capture-time, and `link` only
  uuid/filename. On a library of tens of thousands of assets this drops the
  manifest query from ~11 minutes (and gigabytes of RAM) to well under a minute.
  `pull` still reads the full manifest, since its device allowlist needs camera
  fields that no cheap template exposes.

### Added

- `pm sync` chains `export` and `publish` behind one command, resolving its
  `--since` cutoff from a watermark stored in the index instead of a hand-typed
  date. The first run does a full scan; every clean run after that advances the
  watermark to today, so the next `sync` only considers what's new. A run with
  any failures leaves the watermark in place so the next `sync` retries it.
  `--since` still overrides the watermark for one run, and `--set-since` seeds
  the watermark directly — for a library that's already been brought in sync by
  hand — without running export or publish.
- `pm export` now stamps a frame's canonical stem date onto its derivative HEIC
  when the source file carries no `DateTimeOriginal` of its own, so every
  exported HEIC has a real capture date and Apple Photos never files a scan
  under its import day. A date the source already carries is never overwritten.
- `pm recanon` now writes an XMP sidecar (`<canonical-name>.xmp`) beside each
  renamed file that embeds no capture date, carrying the stem date so Capture
  One shows the true date. The original file's bytes are left untouched.
- `pm publish` now imports in batches (`--batch-size`, default 250) with a short
  pause between them (`--settle`, default 2s), cutting a whole-library push from
  thousands of per-file `osxphotos` sessions to a handful. It keeps Photos.app
  running and warm across the run rather than restarting it — `osxphotos` drives
  Photos over AppleScript, which hangs against a cold, still-loading library.
- `pm reconcile` re-syncs the index's published state to the live Photos library:
  it clears the published marker from derivatives whose asset is no longer in
  Photos (deleted, or never actually kept) so the next publish re-imports them.
  It refuses an empty manifest and never deletes anything.
- `pm publish --stage DIR` hardlinks the derivatives it would import into
  `DIR/YYYY/MM` instead of importing, so a large first-time library can be seeded
  through Photos' native folder import — which, unlike `osxphotos`, does not
  degrade past a couple thousand files. It reuses publish's selection (frames
  already in Photos are associated, not staged) and marks nothing published.
- `pm link` reconnects the index after a native import: it matches each
  unpublished derivative to a Photos asset by filename and marks it published, so
  later publishes skip it. It refuses an empty manifest, never clobbers an
  existing association, and skips a filename claimed by more than one asset.

### Fixed

- `pm publish` now records a derivative as published only when `osxphotos`
  reports it actually imported, instead of trusting any uuid in the report — so a
  file Photos rejects is left unpublished for retry rather than falsely marked.

## [0.9.2] - 2026-07-03

### Fixed

- `pm publish` and `pm pull` now announce the slow osxphotos calls — the
  full-library manifest query, the active-library check, and the export — on
  stderr instead of sitting silent before the first output.

## [0.9.1] - 2026-07-03

### Fixed

- `pm export` now renders the master directly via `sips` when a RAW-only frame
  carries no embedded JPEG — Linear Raw DNGs (HDR merges, panoramas, Topaz
  upscales) and JPEGs misnamed with a RAW extension — instead of failing with
  `no embedded JPEG`. The embedded `PreviewImage`/`JpgFromRaw` path stays
  preferred; identity is unchanged.

## [0.9.0] - 2026-07-03

### Added

- `pm recanon` renames non-canonical archive files — those with no
  `YYYY-MM-DD--HH-MM-SS-` stem, like film scans filed by month — to a
  reduced-precision canonical name derived from their `YYYY/MM` folder
  (`YYYY-MM-<original>`), or `YYYY-MM-DD-<original>` with `--date`. `--match`
  scopes to a batch, `--dry-run` previews, and the content index follows the
  rename.

### Changed

- Canonical stems now parse at day (`YYYY-MM-DD-<original>`) and month
  (`YYYY-MM-<original>`) precision as well as full second precision, through one
  shared parser. `export`, `publish`, and `pull` accept reduced-precision frames
  instead of skipping or silently rejecting them.

## [0.8.1] - 2026-07-03

### Fixed

- `pm export` now extracts the embedded full-resolution JPEG from RAW-only
  frames whose RAW stores it as `PreviewImage` (Canon CR2, and Fuji RAF bodies
  that don't populate `JpgFromRaw`); previously these failed with
  `no JpgFromRaw in <path>`. The extractor tries `PreviewImage` first, then
  `JpgFromRaw`.

## [0.8.0] - 2026-07-03

### Changed

- `pm export` hashes source files in parallel and reuses the shared content
  index (the `files` table) instead of re-hashing, so an unchanged file is never
  re-read — re-exporting an already-indexed library drops from tens of minutes
  to seconds. HEIC generation is parallelized across CPUs.

### Fixed

- `pm export` no longer descends into nested `Unsorted/` directories, and skips
  (rather than panicking on) archive files whose name is not a canonical
  `YYYY-MM-DD--HH-MM-SS-<original>` stem; skipped frames are reported.

## [0.7.0] - 2026-07-02

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

[Unreleased]: https://github.com/danhorst/photo-management/compare/v0.10.0...HEAD
[0.10.0]: https://github.com/danhorst/photo-management/compare/v0.9.2...v0.10.0
[0.9.2]: https://github.com/danhorst/photo-management/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/danhorst/photo-management/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/danhorst/photo-management/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/danhorst/photo-management/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/danhorst/photo-management/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/danhorst/photo-management/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/danhorst/photo-management/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/danhorst/photo-management/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/danhorst/photo-management/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/danhorst/photo-management/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/danhorst/photo-management/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/danhorst/photo-management/compare/v0.1.0...v0.2.0
