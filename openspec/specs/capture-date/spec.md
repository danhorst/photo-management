# capture-date Specification

## Purpose
TBD - created by archiving change backfill-capture-date. Update Purpose after archive.
## Requirements
### Requirement: Every published derivative carries a capture date

The export process SHALL ensure that every derivative HEIC it generates carries a `DateTimeOriginal` (and matching `CreateDate`).
When the source file provides its own capture date, that date SHALL be preserved unchanged.
When the source provides none, the frame's canonical stem date SHALL be stamped instead.
The stem-derived date SHALL NOT overwrite a capture date the source already carries.

Because export only processes frames whose stem is canonical, a stem date is always available, so no derivative is ever emitted without a capture date.

#### Scenario: Source carries its own capture date

- **WHEN** a derivative is generated from a source file that embeds `DateTimeOriginal`
- **THEN** the derivative's `DateTimeOriginal` equals the source's date, not the stem date

#### Scenario: Source lacks a capture date

- **WHEN** a derivative is generated from a source file with no embedded capture date, for a frame whose stem is `2005-06-15-scan001`
- **THEN** the derivative's `DateTimeOriginal` is stamped as `2005:06:15 00:00:00` from the stem

#### Scenario: Month-precision stem

- **WHEN** the frame's stem carries only month precision (`2005-06-scan001`) and the source has no capture date
- **THEN** the derivative's `DateTimeOriginal` is stamped as the first of that month at midnight (`2005:06:01 00:00:00`)

### Requirement: Recanon writes a sidecar for originals lacking a capture date

When `recanon` gives a frame a canonical stem, it SHALL write an XMP sidecar (`<canonical-name>.xmp`) beside each of the frame's files that has no embedded capture date, carrying the stem date as `DateTimeOriginal`.
It SHALL NOT modify the original file's bytes.
A file that already embeds a capture date SHALL NOT receive a sidecar.
Under `--dry-run`, no sidecar SHALL be written.

#### Scenario: Original with no embedded date

- **WHEN** `recanon` renames a frame file that embeds no capture date to `2005-06-15-scan001.jpg`
- **THEN** a `2005-06-15-scan001.xmp` sidecar is written carrying `DateTimeOriginal` `2005:06:15 00:00:00`
- **AND** the original file's bytes (and BLAKE3 hash) are unchanged

#### Scenario: Original that already embeds a date

- **WHEN** `recanon` renames a frame file that already embeds a capture date
- **THEN** no sidecar is written for that file

#### Scenario: Dry run

- **WHEN** `recanon --dry-run` processes a frame file with no embedded capture date
- **THEN** no sidecar file is written

