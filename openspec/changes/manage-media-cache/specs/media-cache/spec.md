## ADDED Requirements

### Requirement: Source volumes are named and tracked

On import the system SHALL record each source volume's human-readable name and
the time it was last seen, keyed by the volume's stable id, so the skip cache
can be presented and managed by name.

#### Scenario: Volume registered on import

- **WHEN** an import processes files from a card whose volume root basename is `EOS_DIGITAL`
- **THEN** the index holds a record for that volume id with name `EOS_DIGITAL` and a last-seen time set to the import time

#### Scenario: Re-import updates last seen

- **WHEN** a previously recorded card is imported again
- **THEN** the volume's last-seen time is updated to the new import time and a new record is not created

#### Scenario: Dry-run does not record

- **WHEN** an import runs with `--dry-run`
- **THEN** no volume record is written

### Requirement: List cached volumes

The system SHALL provide a `media list` command that prints every volume present
in the skip cache, showing its name, a short form of its id, its cached file
count, and its last-seen time.

#### Scenario: Listing cached volumes

- **WHEN** the user runs `photo-import media list` and the cache holds two volumes
- **THEN** both volumes are printed with name, short id, file count, and last-seen time

#### Scenario: Caches created before naming still appear

- **WHEN** the cache contains a volume that has no recorded name (cached before naming existed)
- **THEN** `media list` still lists that volume by id with an empty name and its file count

### Requirement: Clear a cached volume by id

The system SHALL provide a `media clear` command that, given a volume id or an
unambiguous id prefix, removes that volume's skip-cache entries and reports how
many were removed.

#### Scenario: Clear by unambiguous prefix

- **WHEN** the user runs `photo-import media clear <prefix>` and exactly one cached volume id starts with `<prefix>`
- **THEN** that volume's cache entries are removed and the count removed is reported

#### Scenario: Ambiguous prefix is rejected

- **WHEN** the given prefix matches more than one cached volume id
- **THEN** the command reports an error and removes nothing

#### Scenario: Label-only selector is rejected

- **WHEN** the given selector matches no volume id (e.g. a name like `EOS_DIGITAL`)
- **THEN** the command reports an error directing the user to `media list` and removes nothing

### Requirement: Clear cached volumes interactively

The system SHALL, when `media clear` is run with no selector on an interactive
terminal, present the cached volumes in an interactive multiselect and clear each
volume the user selects.

#### Scenario: Interactive multiselect on a terminal

- **WHEN** the user runs `photo-import media clear` with no arguments on a terminal
- **THEN** the cached volumes are shown for selection and each selected volume's cache entries are removed

#### Scenario: No selector without a terminal

- **WHEN** `media clear` is run with no arguments and no interactive terminal is attached
- **THEN** the command reports an error asking for a volume id and removes nothing

### Requirement: Clearing is index-only

Clearing a cached volume SHALL affect only the index and SHALL NOT modify the
card, its files, or its volume marker. A subsequent import of a cleared card
SHALL re-process its files as if newly seen.

#### Scenario: Card and marker untouched

- **WHEN** a cached volume is cleared
- **THEN** the card's marker file and contents are unchanged

#### Scenario: Re-import after clearing re-processes

- **WHEN** a card whose cache was cleared is imported again
- **THEN** its files are re-hashed and re-evaluated rather than skipped
