## ADDED Requirements

### Requirement: Import exported derivatives into Apple Photos

The system SHALL import each `Export/` HEIC recorded by `export` but not yet pushed (its
`derivative` row has a null `photos_uuid`) into Apple Photos via `osxphotos` as a flat import with
no album creation, and SHALL record the returned `photos_uuid` and `published_at` on the
derivative's index row.

#### Scenario: Exported derivative imported

- **WHEN** an `Export/` HEIC has a `derivative` row with no `photos_uuid` and is not yet in Apple Photos
- **THEN** the HEIC is imported into Apple Photos with no album and its `photos_uuid` and `published_at` are recorded on the `derivative` row

### Requirement: Skip our own prior pushes

The system SHALL skip importing any derivative whose `photos_uuid` is already set on its
`derivative` row, so a re-run imports nothing already pushed to Apple Photos.

#### Scenario: Re-run imports nothing new

- **WHEN** publish runs again over derivatives whose `photos_uuid` is already recorded
- **THEN** no HEIC is re-imported into Apple Photos

### Requirement: Skip pre-existing overlap by natural key

The system SHALL, before importing, build a manifest of current Apple Photos assets via
`osxphotos` into the `photos_manifest` cache, and SHALL match each archive frame on the natural
key `DateTimeOriginal` + original filename. When a frame matches an existing asset, the system
SHALL skip the import and record the association rather than creating a duplicate. This SHALL also
leave pulled iPhone-origin frames — already present in Photos — untouched.

#### Scenario: Manually added photo is not duplicated

- **WHEN** a frame's `DateTimeOriginal` and original filename match an asset already in the Photos manifest
- **THEN** the frame is not imported and the association is recorded

#### Scenario: iPhone-origin frame already in Photos is left alone

- **WHEN** a pulled iPhone-origin frame matches an existing Photos asset on the natural key
- **THEN** no import occurs for that frame

### Requirement: Edits import as new assets, never supersede

The system SHALL import an edit's derivative as a new Apple Photos asset alongside existing
renders of the same frame, and SHALL NOT delete or replace any prior asset. No supersede or
replace state is tracked.

#### Scenario: Editing a published frame adds an asset

- **WHEN** a new edit of an already-published frame is published
- **THEN** its derivative is imported as a new asset and the frame's existing assets are left in place

#### Scenario: Nothing is ever deleted

- **WHEN** publish runs for any frame
- **THEN** no Apple Photos asset is deleted or replaced
