## ADDED Requirements

### Requirement: Export iPhone-origin assets by device allowlist

The system SHALL export assets from Apple Photos via `osxphotos export` into a queue directory,
filtered to a device model allowlist (initially `Apple iPhone 13 mini`, extensible later), using
`--update` so already-exported assets are not re-exported.

#### Scenario: Only allowlisted device models are exported

- **WHEN** `pm pull` runs and Photos holds assets from an allowlisted iPhone model and from other devices
- **THEN** only the allowlisted-model assets are exported to the queue directory

#### Scenario: Already-exported assets are not re-exported

- **WHEN** `pm pull` runs again over assets already exported on a prior run
- **THEN** `osxphotos --update` skips re-exporting them

### Requirement: Exclude our own published derivatives

The system SHALL exclude from the export any asset carrying our `catalogKey`, so the tool never
re-ingests a derivative it published, whether a base or an edit render.

#### Scenario: A published derivative is not pulled back

- **WHEN** an Apple Photos asset carries a `catalogKey` we stamped when exporting
- **THEN** it is excluded from the pull export and never imported into the archive

### Requirement: Import through the existing pipeline

The system SHALL run the exported queue directory through the existing import pipeline, so BLAKE3
content dedup and `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original>` organizing apply to pulled frames
unchanged. A frame whose content hash is already in the index SHALL be skipped.

#### Scenario: Pulled frame is organized like any import

- **WHEN** a newly exported iPhone frame is imported
- **THEN** it is organized into `YYYY/MM` under the canonical name and recorded in the index

#### Scenario: Duplicate pulled content is skipped

- **WHEN** a pulled frame's BLAKE3 hash is already in the index
- **THEN** it is skipped by the import pipeline

### Requirement: Live Photos import the still only

The system SHALL import the still component of a Live Photo like any other frame and SHALL ignore
the motion `.mov` component.

#### Scenario: Live Photo still is imported, motion ignored

- **WHEN** a pulled asset is a Live Photo
- **THEN** its still is imported and organized, and its motion `.mov` component is not imported

### Requirement: Pull scoping and dry-run

The system SHALL accept `--since <date>` to limit a pull to assets captured on or after that date,
and SHALL, under `--dry-run`, report what it would export and import while writing nothing.

#### Scenario: Since filters the pull

- **WHEN** `pm pull` runs with `--since 2026-06-01`
- **THEN** only assets captured on or after that date are exported and considered

#### Scenario: Dry-run writes nothing

- **WHEN** `pm pull` runs with `--dry-run`
- **THEN** the intended export and import are reported and no file or index row is written
