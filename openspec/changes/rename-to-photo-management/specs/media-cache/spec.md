## MODIFIED Requirements

### Requirement: List cached volumes

The system SHALL provide a `media list` command that prints every volume present
in the skip cache, showing its name, a short form of its id, its cached file
count, and its last-seen time.

#### Scenario: Listing cached volumes

- **WHEN** the user runs `pm media list` and the cache holds two volumes
- **THEN** both volumes are printed with name, short id, file count, and last-seen time

#### Scenario: Caches created before naming still appear

- **WHEN** the cache contains a volume that has no recorded name (cached before naming existed)
- **THEN** `media list` still lists that volume by id with an empty name and its file count

### Requirement: Clear a cached volume by id

The system SHALL provide a `media clear` command that, given a volume id or an
unambiguous id prefix, removes that volume's skip-cache entries and reports how
many were removed.

#### Scenario: Clear by unambiguous prefix

- **WHEN** the user runs `pm media clear <prefix>` and exactly one cached volume id starts with `<prefix>`
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

- **WHEN** the user runs `pm media clear` with no arguments on a terminal
- **THEN** the cached volumes are shown for selection and each selected volume's cache entries are removed

#### Scenario: No selector without a terminal

- **WHEN** `media clear` is run with no arguments and no interactive terminal is attached
- **THEN** the command reports an error asking for a volume id and removes nothing
