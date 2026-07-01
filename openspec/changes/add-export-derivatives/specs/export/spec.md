## ADDED Requirements

### Requirement: Frame grouping keyed off the canonical stem

The system SHALL group archive files into frames using the known canonical stem
(`YYYY-MM-DD--HH-MM-SS-<original>`), not naive hyphen-splitting, and SHALL classify each file in
a group as the master (`<stem>.<raw>`), the camera JPEG (`<stem>.JPG`), an iPhone-origin frame
(`<stem>.HEIC`), or a baked edit (`<stem>-<suffix>.<img>`).

#### Scenario: Edit distinguished from a hyphenated stem

- **WHEN** a group holds `2026-06-01--12-00-00-DSCF1234.RAF` and `2026-06-01--12-00-00-DSCF1234-bw.jpg`
- **THEN** the `.RAF` is classified as the master and the `-bw.jpg` as an edit with suffix `bw`, and the hyphens inside the stem are not mistaken for a suffix

#### Scenario: Camera JPEG and master share a frame

- **WHEN** a group holds `<stem>.RAF` and `<stem>.JPG`
- **THEN** both are recognized as the same frame, with the `.JPG` as the camera JPEG

### Requirement: Derivative source resolution

The system SHALL resolve, for each frame, one base derivative source plus one source per baked
edit. The base source SHALL be the sibling camera JPEG, or the embedded `JpgFromRaw` extracted
from the RAF when the frame is RAW-only. Edit sources SHALL be every `<stem>-<suffix>` baked
file. Edits are additive and SHALL NOT suppress the base. An iPhone-origin frame (a `<stem>.HEIC`
with no camera JPEG) SHALL yield no derivative.

#### Scenario: RAF+JPEG frame with two edits

- **WHEN** a frame has a camera JPEG and two baked edits `-edit` and `-bw`
- **THEN** three sources resolve: the camera JPEG (base) plus the `-edit` and `-bw` files

#### Scenario: RAW-only frame uses embedded JPEG

- **WHEN** a frame has a RAF but no sibling camera JPEG
- **THEN** the base source is the `JpgFromRaw` embedded in the RAF

#### Scenario: iPhone-origin frame is left alone

- **WHEN** a frame is a `<stem>.HEIC` with no camera JPEG
- **THEN** no derivative source is resolved and the frame is skipped

### Requirement: Presentation HEIC generation

The system SHALL generate each derivative as a HEIC via `sips`, resized to a configurable long
edge (default 4096 px) at a configurable quality (default ~70), and SHALL carry
`DateTimeOriginal`, GPS, and orientation into the HEIC. Each HEIC SHALL be stamped with
`catalogKey` (the version id — BLAKE3 of the source file) and `catalogStem` (the frame id) in XMP
that survives renaming.

#### Scenario: HEIC generated with carried metadata and identity

- **WHEN** a base source is exported
- **THEN** a HEIC is produced with long edge 4096 px, its `DateTimeOriginal`/GPS/orientation carried over, and `catalogKey` and `catalogStem` stamped in XMP

#### Scenario: Configured long edge is honored

- **WHEN** the configured long edge is set to a non-default value
- **THEN** the generated HEIC is resized to that long edge

### Requirement: Persistent Export mirror naming

The system SHALL write derivatives to `Export/YYYY/MM`, naming the base `<stem>.heic` and each
edit `<stem>-<suffix>.heic`, so multiple derivatives of one frame coexist without collision. The
`Export/` tree SHALL persist as a regenerable local mirror and SHALL contain only generated
presentation HEICs — never full-res baked edits.

#### Scenario: Base and edit derivatives coexist

- **WHEN** a frame exports a base and a `-bw` edit
- **THEN** `Export/YYYY/MM/<stem>.heic` and `Export/YYYY/MM/<stem>-bw.heic` are both written

#### Scenario: Export holds only presentation HEICs

- **WHEN** an edit is exported
- **THEN** only the generated `<stem>-<suffix>.heic` is placed under `Export/`; the full-res baked edit stays beside the master in the archive

### Requirement: Incremental, idempotent export

The system SHALL record each generated derivative in a `derivative` table keyed by a unique
`source_hash` (version id), with `stem`, `source_kind` (edit|jpeg|embedded), `heic_path`, and
`generated_at`; the `photos_uuid` and `published_at` columns are left for the `publish` verb. On
export the system SHALL skip any source whose `source_hash` is already recorded, regenerating
nothing that already exists.

#### Scenario: Already-generated source is skipped

- **WHEN** export encounters a source whose `source_hash` is already in the `derivative` table
- **THEN** the source is skipped and its HEIC is not regenerated

#### Scenario: New edit of an exported frame is generated

- **WHEN** a new baked edit appears for a frame whose base is already exported
- **THEN** the edit is a new `source_hash`, so its `<stem>-<suffix>.heic` is generated and recorded alongside the existing base row

### Requirement: Export scoping and dry-run

The system SHALL accept `--since <date>` to limit an export run to frames captured on or after
that date, and SHALL, under `--dry-run`, report the derivatives it would generate while writing
no files and no index rows.

#### Scenario: Since filters the run

- **WHEN** export runs with `--since 2026-06-01`
- **THEN** only frames captured on or after that date are considered

#### Scenario: Dry-run writes nothing

- **WHEN** export runs with `--dry-run`
- **THEN** the intended derivatives are reported and no HEIC or `derivative` row is written
