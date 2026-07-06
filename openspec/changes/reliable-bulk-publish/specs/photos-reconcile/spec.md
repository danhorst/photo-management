## ADDED Requirements

### Requirement: Reconcile re-syncs published state to the live library

`pm reconcile` SHALL query the live Photos manifest and, for every derivative whose recorded `photos_uuid` is not present in that manifest, clear its published state (`photos_uuid` and `published_at`) so the next `publish` re-imports it.
It SHALL report how many rows were cleared, and under `--dry-run` SHALL change nothing.
It SHALL NOT delete derivative rows, HEIC files, or Photos assets — it only clears the published marker.

#### Scenario: A deleted asset is re-queued

- **WHEN** a derivative is marked published with a uuid that is no longer in the Photos manifest
- **THEN** `pm reconcile` clears that derivative's published state, and a subsequent `publish` selects it for import again

#### Scenario: A present asset is left alone

- **WHEN** a derivative's recorded uuid is present in the live manifest
- **THEN** `pm reconcile` leaves that derivative's published state unchanged

#### Scenario: Dry run

- **WHEN** `pm reconcile --dry-run` finds derivatives whose uuid is missing from the manifest
- **THEN** it reports the count and clears nothing

### Requirement: Reconcile refuses an empty manifest

`pm reconcile` SHALL abort without changing anything when the live Photos manifest is empty, since an empty result (a failed query or the wrong library) would otherwise un-mark every published derivative.

#### Scenario: Empty manifest aborts

- **WHEN** the live manifest query returns zero assets
- **THEN** `pm reconcile` aborts with an error and clears no published state
