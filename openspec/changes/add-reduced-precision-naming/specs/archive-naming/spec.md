## ADDED Requirements

### Requirement: Reduced-precision canonical stems

The archive SHALL support three date precisions for a canonical stem: second (`YYYY-MM-DD--HH-MM-SS-<original>`), day (`YYYY-MM-DD-<original>`), and month (`YYYY-MM-<original>`).
A reduced-precision stem SHALL drop the unknown trailing date fields rather than fabricate them.

#### Scenario: Day precision stem

- **WHEN** a frame's capture date is known only to the day
- **THEN** its canonical stem SHALL be `YYYY-MM-DD-<original>` with no time component

#### Scenario: Month precision stem

- **WHEN** a frame's capture date is known only to the month
- **THEN** its canonical stem SHALL be `YYYY-MM-<original>` with no day or time component

### Requirement: Single stem parser owns the grammar

The system SHALL parse a canonical stem through one shared parser that returns the capture time, the detected precision, and the original name.
Precedence SHALL be second, then day, then month, so the longest matching form wins.

#### Scenario: Full stem parses at second precision

- **WHEN** a stem is `2021-12-05--14-03-22-IMG_0003`
- **THEN** the parser SHALL return time `2021-12-05 14:03:22`, precision second, and original `IMG_0003`

#### Scenario: Day stem parses at day precision

- **WHEN** a stem is `2021-12-05-IMG_0003` (date parses, char 10 is `-`, char 11 is not `-`)
- **THEN** the parser SHALL return time `2021-12-05 00:00:00`, precision day, and original `IMG_0003`

#### Scenario: Month stem parses at month precision

- **WHEN** a stem is `2021-12-IMG_0003` (first ten chars do not parse as a date, first seven parse as `YYYY-MM`)
- **THEN** the parser SHALL return time `2021-12-01 00:00:00`, precision month, and original `IMG_0003`

#### Scenario: Non-canonical stem is rejected

- **WHEN** a stem is `IMG_0003-ZF-9821` with no leading date
- **THEN** the parser SHALL report failure

### Requirement: Reduced-precision frames are exportable and matchable

Export, publish, and pull SHALL accept day- and month-precision frames rather than skipping or silently rejecting them.
Overlap matching for reduced-precision frames MAY be best-effort, since the exact time is unknown.

#### Scenario: Export accepts a reduced-precision frame

- **WHEN** `pm export` encounters a frame whose stem is `2021-12-IMG_0003`
- **THEN** it SHALL treat the frame as canonical and derive its capture date, not count it as non-canonical

#### Scenario: Publish accepts a reduced-precision stem

- **WHEN** publish or pull evaluates a frame whose stem is day or month precision
- **THEN** the natural-key matcher SHALL parse the stem instead of rejecting it
