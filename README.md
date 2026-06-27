# photo-import

Fast, deduplicating photo importer for a Capture One library.

Imports media into a `YYYY/MM` tree named `YYYY-MM-DD--HH-MM-SS-<original>`, skipping files whose contents are already in the index.
Duplicate detection is a BLAKE3 content-hash lookup against a SQLite index, so it stays fast across a terabyte-scale library where Capture One's own import scan is slow.

## Install

```
brew install danhorst/tap/photo-import
```

Requires `exiftool`, installed automatically as a dependency.

## Use

Build the index once, and after any change made to the library outside this tool:

```
photo-import index
```

Import a card or queue directory:

```
photo-import /Volumes/UNTITLED/DCIM
```

New files are organized into the library; content duplicates are skipped silently.
The summary lists the `YYYY/MM` folders that changed — **Synchronize** those folders in Capture One to bring them into the catalog.
The library uses referenced images, so a per-folder sync is fast and sidesteps Capture One's slow whole-library duplicate scan.

Files move when the source is on the same volume as the library, and copy otherwise.
Files without a readable capture date go to `Unsorted/`.

Re-importing a card that still holds already-imported files is near-instant: each card is stamped with a `.photo-import.toml` marker at its root, and files already pulled from it are skipped by size and modification time without re-reading their contents.

## Commands

- `photo-import <source>` — import from a directory. Flags: `--dry-run`, `--debug`.
- `photo-import index` — build or refresh the content-hash index.
- `photo-import stats` — show index location and size.
- `photo-import config <cmd>` — read/write the config file (see below).
- `photo-import version` — print the version.

## Configuration

`~/.config/photo-import/photo-import.toml`:

```toml
library = "/Volumes/Photos"
database = "/Volumes/Photos/.photo-index.db"
```

Both default as shown; the database defaults to a dotfile inside the library so it travels with the drive.
Override per run with `--library`/`-L` and `--db`.

Manage the file from the CLI instead of editing by hand:

```
photo-import config init                       # write a default config file
photo-import config set library /Volumes/Archive
photo-import config show                        # print the effective values
photo-import config path                        # print the file location
```

`database` derives from `library` unless set explicitly, so changing the library moves the index with it.

## Organization

Photos are renamed and organized by date.
This descends from [work by @cliss](https://gist.github.com/cliss/6854904) which, in turn, was based on a [script by Dr. Drang](http://www.leancrew.com/all-this/2013/10/photo-management-via-the-finder/).
