# Project Context

## Purpose

photo-management is bidirectional photo syndication around a canonical on-disk archive.

The archive is a Capture One referenced library at `/Volumes/Photos`, organized `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original>`, and it is the single source of truth.
Apple Photos, iCloud, and Google Photos are downstream, regenerable presentation catalogs, not authorities.

Photos move through four symmetrical, single-hop operations around that archive:

- **import** — camera cards / queue directories → archive (the original, fast, deduplicating importer).
- **export** — archive → downsized HEIC derivatives in `Export/` on disk, for browse/share size.
- **publish** — `Export/` HEICs → Apple Photos, for browse/share on the phone and onward sync to Google.
- **pull** — iPhone-captured photos in Apple Photos → archive, so the archive is canonical for phone photos too.

Each verb is one discrete step, so they compose. A `sync` operation may be added to chain steps — typically `export` then `publish`, and potentially a full round-trip — but the four verbs are the primitives and `sync` is only convenience.

The tool has two overriding goals:

1. Allow the canonical photo library to:
	1. Be a plain, vendor-independent directory tree of all personal photos, regardless of capture device — dead simple to back up to the cloud.
	2. Facilitate RAW workflows in Capture One.
	3. Be quickly and easily accessible in Apple Photos and Google Photos.
2. Make importing into that library fast and reliable.
	The Capture One import process is very slow when scanning removable media with large numbers of files.

The library uses referenced images, so the tool only ever moves and names files on disk — it never writes to the Capture One catalog.

## Design guarantees

- The archive on disk is authoritative for all four operations; the index is a rebuildable cache, never the truth.
- Import is idempotent and stays fast across a terabyte-scale library, where a naive content scan would not.
- Export is incremental: a run skips any source content already generated and regenerates nothing that already exists.
- Publish is idempotent: a run skips derivatives already pushed and frames already present in Apple Photos.
- Pull makes the archive canonical for iPhone frames, reusing the import core so BLAKE3 dedup and `YYYY/MM` organizing apply unchanged.
- `Export/` is a persistent local mirror of exactly what is in the cloud catalog — regenerable, and explicitly excluded from backup.
- The tool is safe to interrupt and re-run: a crash or Ctrl-C leaves the archive and index in a recoverable state.

## Critical behavior

Deduplication is by content, not by name or path.
A file is a duplicate when its BLAKE3 content hash is already in the index; duplicates are skipped silently.

Organization is by capture date.
Each imported file lands at `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original-name>`.
The date comes from EXIF `DateTimeOriginal`, falling back to `CreateDate`; a file with no readable capture date goes to `Unsorted/`.

There are two levels of identity, doing different jobs:

- **Frame identity is the stem** (`2026-06-01--12-00-00-DSCF1234`), shared by the master, camera JPEG, baked edits, and every derivative of that frame.
- **Version identity is the source content hash** (BLAKE3 of the file that produced a HEIC); the base derivative and each edit derivative have different version ids.

Export stamps each derivative HEIC, in XMP that survives renaming, with `catalogKey` (version id) and `catalogStem` (frame id).
The version id gives per-derivative idempotency — export skips a source already generated, publish skips a derivative already pushed — and marks the file as our own for reverse-sync exclusion; the stem groups all versions of a frame after the fact.

Export yields one **base** derivative plus one derivative **per baked edit**; edits are additive and never suppress the base.
Publishing an edited frame imports the new render as a **new asset** — nothing is superseded or deleted, because Apple Photos has no unattended programmatic delete and a synced Google copy is independent anyway.

Two independent layers keep publish from duplicating: our own pushes are recognized by version id, and pre-existing overlap (manually added or iPhone-origin) is matched on a natural key (`DateTimeOriginal` + original filename, both encoded in the archive filename).

Files move when the source is on the same volume as the library and copy otherwise, so same-volume imports are fast and cross-volume imports leave the card intact.

Only managed media is imported (`jpg/jpeg/heic/png/gif/tif/tiff/cr2/raf/dng/crw/mov/mp4/avi`); AppleDouble `._` sidecars and the index database itself are ignored.

The index is a rebuildable BLAKE3 content-hash cache in SQLite, refreshed with `index` and required after any change made to the library outside this tool.

Each source card is stamped with a `.photo-management.toml` marker at its volume root and tracked in a skip cache.
Files already pulled from a known card are skipped by size and modification time without re-reading their contents.
Reformatting a card wipes the marker and mints a fresh volume id, orphaning the old cache entries; `media list` and `media clear` manage stale entries.
This capability is specified in `specs/media-cache/`.

`--dry-run` reports what would happen and writes nothing — no files moved, no index or volume records changed.

## Tech Stack

- Go (single static binary, distributed via the `danhorst/tap` Homebrew tap).
- SQLite for the content-hash index and syndication state.
- BLAKE3 for content hashing.
- `exiftool` for capture timestamps, embedded-JPEG extraction, and metadata carry-over / `catalogKey`/`catalogStem` stamping; driven in batch and in long-running `-stay_open` daemon mode; installed as a Homebrew dependency.
- `sips` (macOS built-in) for JPEG/RAF → HEIC transcode and resize.
- `osxphotos` for the Apple Photos manifest query, import into Photos, and reverse export.

## Project Conventions

- Personal DBH repo style: terse commit messages with a `Co-Authored-By` footer, sparse comments, rules-first README, one-sentence-per-line Markdown.
- Standard Go layout: `cmd/pm` for the CLI surface, `internal/*` packages per concern (`index`, `exif`, `hash`, `organize`, `media`, `volume`, `config`, and new packages for export/publish/pull).
- Tests live beside the code they cover; behavior changes ship with tests.
- Spec-driven changes go through OpenSpec; specs in `specs/` are the current truth, proposals in `changes/`.

## Important Constraints

- The default library is a live 1TB Capture One library at `/Volumes/Photos`; runs against it are real. Test against a sandbox library with `-L`/`--db`, never the default.
- The index database defaults to a dotfile inside the library so it travels with the drive; deriving `database` from `library` is intentional.
- The tool must never modify the Capture One catalog; it only moves and names files on disk.
- Apple Photos has no unattended programmatic delete (osxphotos cannot delete; PhotoKit forces a confirmation prompt), which is why publish never supersedes or replaces.
- `Export/` is regenerable and excluded from backup; only the master tier (RAF, camera JPEG, baked edits, iPhone-origin originals) is backed up.
- Reverse syndication is gated by a device model allowlist, initially `Apple iPhone 13 mini`.
- No fixity or bit-rot detection today: the BLAKE3 index dedups on ingest, it does not verify the archive against silent corruption. A future `verify`/`fsck` capability could add this.

## External Dependencies

- `exiftool` — capture-date extraction, embedded-JPEG extraction, metadata stamping (hard runtime dependency).
- `sips` — HEIC transcode/resize (macOS built-in).
- `osxphotos` — Apple Photos manifest, import, and reverse export; note it has no delete capability, by Apple design.
- Capture One — the downstream catalog and a Process Recipe that bakes each edit to `<stem>-<suffix>.<ext>` beside the master; integration is a manual per-folder Synchronize / per-frame export, not automated.
- Google Photos — the onward presentation target, seeded by the phone trickle or bulk from `Export/`; its copies are independent of iCloud once backed up.
- Homebrew tap `danhorst/tap` — distribution.
