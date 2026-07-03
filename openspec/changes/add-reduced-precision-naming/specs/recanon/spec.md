## ADDED Requirements

### Requirement: Catalogue non-canonical stragglers

`pm recanon` SHALL walk the `YYYY/` archive tree, group files into frames, and select frames whose stem is not a canonical stem at any precision.
It SHALL skip `Unsorted/` and excluded directories, matching how `export` walks the archive.

#### Scenario: Selects a non-canonical batch in a year folder

- **WHEN** `2021/12/` contains files named `IMG_####-ZF-9821-41309-1-001-###.jpg`
- **THEN** `pm recanon` SHALL select those frames as non-canonical

#### Scenario: Ignores already-canonical and Unsorted files

- **WHEN** a frame already has a canonical stem, or a file lives under `Unsorted/`
- **THEN** `pm recanon` SHALL not select it

### Requirement: Rename to a reduced-precision canonical name

`pm recanon` SHALL derive a fallback date from the frame's folder path (`YYYY/MM` giving month precision) and rename the file in place to the corresponding reduced-precision canonical name, keeping the current filename as the original component.
A `--date YYYY-MM-DD` flag SHALL stamp day precision for the matched frames instead of month-from-folder, and a `--match <substr>` flag SHALL scope the operation to frames whose name contains the substring.

#### Scenario: Month precision from folder

- **WHEN** `pm recanon` renames `2021/12/IMG_0003-ZF-9821-41309-1-001-004.jpg`
- **THEN** the new name SHALL be `2021/12/2021-12-IMG_0003-ZF-9821-41309-1-001-004.jpg`

#### Scenario: Day precision from flag

- **WHEN** `pm recanon --match ZF-9821 --date 2021-12-05` runs
- **THEN** matched frames SHALL be renamed to `2021-12-05-<original>` day-precision names

### Requirement: Safe, idempotent, and index-consistent

`pm recanon` SHALL honor the global `--dry-run` flag, writing nothing and only reporting proposed renames.
A second run after a successful rename SHALL select nothing, because the renamed frames now parse as canonical.
After renaming, the BLAKE3 content index SHALL remain consistent — the tool SHALL update the index path for the moved file, or instruct the user to run `pm index`.

#### Scenario: Dry run writes nothing

- **WHEN** `pm recanon --dry-run` runs against non-canonical frames
- **THEN** it SHALL list the proposed renames and leave every file and index record unchanged

#### Scenario: Idempotent re-run

- **WHEN** `pm recanon` runs a second time after a successful rename
- **THEN** it SHALL find no non-canonical frames and make no changes
