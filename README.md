# photo-management

Fast, deduplicating photo importer for a Capture One library.

Imports media into a `YYYY/MM` tree named `YYYY-MM-DD--HH-MM-SS-<original>`, skipping files whose contents are already in the index.
Duplicate detection is a BLAKE3 content-hash lookup against a SQLite index, so it stays fast across a terabyte-scale library where Capture One's own import scan is slow.

## Install

```
brew install danhorst/tap/pm
```

Requires `exiftool`, installed automatically as a dependency.

## Use

Build the index once, and after any change made to the library outside this tool:

```
pm index
```

Import a card or queue directory:

```
pm /Volumes/UNTITLED/DCIM
```

New files are organized into the library; content duplicates are skipped silently.
The summary lists the `YYYY/MM` folders that changed — **Synchronize** those folders in Capture One to bring them into the catalog.
The library uses referenced images, so a per-folder sync is fast and sidesteps Capture One's slow whole-library duplicate scan.

Files move when the source is on the same volume as the library, and copy otherwise.
Files without a readable capture date go to `Unsorted/`.

Re-importing a card that still holds already-imported files is near-instant: each card is stamped with a `.photo-management.toml` marker at its root, and files already pulled from it are skipped by size and modification time without re-reading their contents.

## Commands

- `pm <source>` — import from a directory. Flags: `--dry-run`, `--debug`.
- `pm export` — generate presentation HEICs into `Export/` (see below). Flags: `--since YYYY-MM-DD`, `--dry-run`, `--debug`.
- `pm publish` — import exported HEICs into Apple Photos (see below). Flags: `--dry-run`, `--debug`, `--photos-library PATH`, `--batch-size N`, `--settle DUR`, `--stage DIR`.
- `pm link` — link natively-imported Photos assets back into the index by filename (see below). Flags: `--dry-run`, `--debug`, `--photos-library PATH`.
- `pm pull` — pull iPhone photos from Apple Photos into the archive (see below). Flags: `--since YYYY-MM-DD`, `--dry-run`, `--debug`, `--photos-library PATH`.
- `pm index` — build or refresh the content-hash index.
- `pm stats` — show index location and size.
- `pm config <cmd>` — read/write the config file (see below).
- `pm media list` — list every cached volume: name, id, file count, and last-seen date.
- `pm media clear [<id>…]` — remove a volume's skip-cache entries by id or unambiguous id prefix; with no arguments on a terminal, opens an interactive multiselect.
- `pm version` — print the version.

### Export

`pm export` turns archive frames into downsized HEIC derivatives under `Export/YYYY/MM` inside the library.
Each frame yields one base derivative — from the sibling camera JPEG, or the RAF's embedded `JpgFromRaw` when RAW-only — named `<stem>.heic`, plus one `<stem>-<suffix>.heic` per baked Capture One edit.
iPhone-origin frames (a `.HEIC` with no camera JPEG) are left alone.

Derivatives are resized to a 4096 px long edge at quality 70 (configurable via `export_long_edge` / `export_quality`), carry `DateTimeOriginal` (falling back to the frame's stem date when the source has none, so a derivative is never undated) along with GPS/orientation, and are stamped with `catalogKey` (BLAKE3 of the source file) and `catalogStem` (the frame stem) in XMP.
Export is incremental: a source whose hash is already recorded in the `derivative` table is skipped, so re-runs only generate what's new.
`--since YYYY-MM-DD` scopes a run; `--dry-run` reports without writing.

`Export/` is a regenerable presentation mirror — exclude it from backup; only the master tier is backed up.

### Publish

`pm publish` imports the `Export/` HEICs that `export` recorded but has not yet pushed into Apple Photos, as a flat import with no album creation.
It requires [`osxphotos`](https://github.com/RhetTbull/osxphotos) — not packaged for Homebrew, so it's managed as a separate tool dependency rather than by the `pm` formula.
`osxphotos` reads the Photos library's files directly, which macOS gates behind Full Disk Access for whatever terminal app is running it — if a run fails with a Python traceback mentioning "Operation not permitted", grant Full Disk Access to your terminal in System Settings > Privacy & Security, then restart it.

`--photos-library PATH` targets a specific library instead of whatever's open — useful for testing against a throwaway library. It pins `osxphotos query`, but **not** `osxphotos import`: import always writes into whichever library Photos.app currently has open, regardless of this flag, so publish checks the pinned and ambient manifests match before writing and aborts if Photos.app isn't actually on the target library. Switching Photos.app itself (File > Switch Library) is still on you.

Two independent layers keep it from duplicating:

- Our own pushes: a derivative whose `photos_uuid` is already recorded is skipped, so re-runs import nothing already pushed.
- Pre-existing overlap: a manifest of current Photos assets is built via `osxphotos` at the start of each run, and a frame matching on the natural key (`DateTimeOriginal` + original filename, both encoded in the archive filename) is skipped and associated instead of imported. This also leaves pulled iPhone-origin frames alone.

Nothing is ever deleted or replaced: an edit imports as a new asset alongside existing renders of the frame.
Apple Photos has no unattended programmatic delete by design — `osxphotos` cannot delete, and PhotoKit forces a confirmation prompt — so publish never supersedes.

Imports run in batches (`--batch-size`, default 250) with a short pause between them (`--settle`, default 2s).
`osxphotos` drives Photos over AppleScript, which stays responsive only while Photos is warm, so publish keeps Photos running for the whole run and lets its background queue drain between batches rather than quitting it — a restart would cold-launch the next batch into an unloaded library and hang.
Leave Photos.app open on the target library before starting.
A derivative is marked published only when `osxphotos` reports it actually imported; a file Photos rejects stays unpublished, so a re-run retries it.

### Reconcile

`pm reconcile` re-syncs the index's published state to what Apple Photos actually holds.
It queries the live manifest and clears the published marker from any derivative whose recorded asset is gone from Photos — deleted by hand, or never kept — so the next `pm publish` re-imports it.
It never deletes anything, and it refuses to run against an empty manifest (a failed query or the wrong open library would otherwise un-mark everything).
`--dry-run` reports the count without writing.

### Seeding a large library

`osxphotos import` drives Photos over AppleScript, and a whole-library first push (tens of thousands of derivatives) degrades Photos' import subsystem after a couple thousand files — it starts rejecting valid files, and only a full Photos restart clears it.
For that one-time bulk seed, skip `osxphotos` and let Photos import the files itself:

1. `pm publish --stage DIR` — instead of importing, hardlink every derivative that would be imported into `DIR/YYYY/MM`, mirroring `Export/`. It reuses publish's selection, so frames already in Photos are associated (not staged) and nothing is duplicated. `DIR` must be on the library volume (hardlinks don't cross volumes). Nothing is marked published.
2. Import the staged tree through Photos' own **File → Import** — a first-party bulk path that doesn't hit the AppleScript wall. Import a year folder at a time if the whole tree is too large in one go.
3. `pm link` — reconnect the index to what landed. It matches each unpublished derivative to a Photos asset by filename (the asset's original name equals the derivative's HEIC stem — the filename without its extension, a unique key) and marks it published, so later `pm publish` runs skip it. It refuses an empty manifest, never clobbers an existing association, and skips a filename claimed by more than one asset. `--dry-run` reports the counts without writing.

Steady-state incremental publishing stays on `pm publish` — small runs never approach the wall.
Delete the staging directory once the seed is linked.

### Pull

`pm pull` makes the archive canonical for iPhone photos: it exports phone-origin assets from Apple Photos into a queue directory via `osxphotos`, then runs the existing import pipeline over the queue, so BLAKE3 dedup and `YYYY/MM` organizing apply unchanged.
Same `osxphotos`/Full Disk Access requirement as publish, above.

Assets are scoped by a device model allowlist — `pull_devices` in the config, defaulting to `Apple iPhone 13 mini` — and any asset this tool published is excluded so a derivative is never re-ingested.
Two independent layers keep repeat pulls cheap: `osxphotos --update` skips re-exporting to the queue, and the content-hash index skips re-importing.
Live Photos import the still only; the motion component is left behind.

### Media cache and reformatted cards

Each card is identified by a `.photo-management.toml` marker stamped at the card's volume root.
A card stamped with the old `.photo-import.toml` marker is still recognized; new stamps use the new name.
Reformatting a card wipes that marker, so the next import treats it as a new card and mints a fresh volume id.
The old id's cache entries become orphaned: they are never read again but also never removed automatically.
Use `pm media list` to see all cached volumes and `pm media clear <id>` to remove stale entries.
Clearing touches only the index — the card and its files are unchanged — and the next import of that card re-hashes from scratch.

## Configuration

`~/.config/photo-management/photo-management.toml`:

```toml
library = "/Volumes/Photos"
database = "/Volumes/Photos/.photo-index.db"
```

Both default as shown; the database defaults to a dotfile inside the library so it travels with the drive.
An existing config at the old `~/.config/photo-import/photo-import.toml` path is still read; the first `config set` migrates it to the new path.
Override per run with `--library`/`-L` and `--db`.

Manage the file from the CLI instead of editing by hand:

```
pm config init                       # write a default config file
pm config set library /Volumes/Archive
pm config show                        # print the effective values
pm config path                        # print the file location
```

`database` derives from `library` unless set explicitly, so changing the library moves the index with it.

## Organization

Photos are renamed and organized by date.
This descends from [work by @cliss](https://gist.github.com/cliss/6854904) which, in turn, was based on a [script by Dr. Drang](http://www.leancrew.com/all-this/2013/10/photo-management-via-the-finder/).
