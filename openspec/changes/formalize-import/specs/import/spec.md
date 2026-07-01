## ADDED Requirements

### Requirement: Content-based deduplication

The system SHALL treat a source file as a duplicate when its BLAKE3 content hash is already in
the index, and SHALL skip duplicates silently without copying, moving, or re-indexing them.
Dedup is by content, never by name or path, so a renamed or relocated copy of an
already-imported file is still recognized.

#### Scenario: Duplicate content is skipped

- **WHEN** an import processes a file whose BLAKE3 hash is already recorded in the index
- **THEN** the file is skipped silently and the library is unchanged

#### Scenario: Renamed duplicate is still caught

- **WHEN** an import processes a file that is byte-identical to an already-imported file but has a different name
- **THEN** it is recognized as a duplicate by content hash and skipped

#### Scenario: New content is imported and indexed

- **WHEN** an import processes a file whose hash is not in the index
- **THEN** the file is organized into the library and its hash is recorded in the index

### Requirement: Capture-date organization

The system SHALL organize each imported file to `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original-name>`,
deriving the date from EXIF `DateTimeOriginal` and falling back to `CreateDate`. A file with no
readable capture date SHALL be placed under `Unsorted/`.

#### Scenario: Organized by DateTimeOriginal

- **WHEN** a file carries `DateTimeOriginal` of `2026-06-01 12:00:00` and original name `DSCF1234.RAF`
- **THEN** it is placed at `2026/06/2026-06-01--12-00-00-DSCF1234.RAF`

#### Scenario: Falls back to CreateDate

- **WHEN** a file has no `DateTimeOriginal` but has a `CreateDate`
- **THEN** the `CreateDate` is used for the folder and filename

#### Scenario: Undated file goes to Unsorted

- **WHEN** a file has no readable capture date
- **THEN** it is placed under `Unsorted/`

### Requirement: Managed-media filtering

The system SHALL import only managed media extensions
(`jpg/jpeg/heic/png/gif/tif/tiff/cr2/raf/dng/crw/mov/mp4/avi`) and SHALL ignore AppleDouble
`._` sidecar files and the index database file itself.

#### Scenario: Unmanaged extension is ignored

- **WHEN** a source directory contains a file whose extension is not in the managed set
- **THEN** that file is not imported

#### Scenario: AppleDouble sidecar is ignored

- **WHEN** a source directory contains a `._`-prefixed AppleDouble sidecar
- **THEN** that sidecar is not imported

### Requirement: Move on same volume, copy across volumes

The system SHALL move a source file into the library when the source is on the same volume as
the library, and SHALL copy it otherwise, so same-volume imports are fast and cross-volume
imports leave the source intact.

#### Scenario: Same-volume import moves

- **WHEN** the source file is on the same volume as the library
- **THEN** the file is moved into the library and no longer exists at the source path

#### Scenario: Cross-volume import copies

- **WHEN** the source file is on a different volume from the library (e.g. a camera card)
- **THEN** the file is copied into the library and the source copy is left intact

### Requirement: Dry-run writes nothing

The system SHALL, when run with `--dry-run`, report what would happen and write nothing — no
files moved or copied, and no index or volume records changed.

#### Scenario: Dry-run leaves everything untouched

- **WHEN** an import runs with `--dry-run` over a source with new files
- **THEN** the intended actions are reported and no file, index row, or volume record is written
