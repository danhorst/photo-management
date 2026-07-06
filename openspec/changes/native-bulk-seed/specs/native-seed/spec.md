## ADDED Requirements

### Requirement: Stage unpublished derivatives for native import

`pm publish --stage DIR` SHALL, instead of importing into Photos, hardlink each derivative it would import into `DIR/YYYY/MM/<basename>`, mirroring the `Export/` layout, so the set can be seeded through Photos' native folder import.
It SHALL reuse publish's selection unchanged — `--since` scoping and layer-2 association of frames already in Photos — so a frame already present is associated and NOT staged.
It SHALL NOT mark any derivative published (linking happens after the import).
The stage directory MUST be on the library volume; a cross-volume link SHALL fail with a clear message. Staging SHALL be idempotent, skipping a target that already exists.

#### Scenario: Only the true import set is staged

- **WHEN** `publish --stage /Vol/.stage` runs with some derivatives already published and one frame already present in Photos by natural key
- **THEN** the already-published derivatives and the already-present frame are not staged, every remaining derivative is hardlinked under `/Vol/.stage/YYYY/MM/`, and no derivative is marked published

### Requirement: Link natively-imported assets into the index

`pm link` SHALL match each unpublished derivative to a live Photos asset by filename — the asset's original name equal to the derivative's HEIC stem (its basename without the `.heic` extension, since osxphotos' `{original_name}` strips the extension) — and mark it published with that asset's uuid.
It SHALL refuse an empty manifest, SHALL touch only unpublished derivatives (never clobbering an existing association), and SHALL skip a filename that maps to more than one asset as ambiguous.
`--dry-run` SHALL report the counts without writing.

#### Scenario: A natively-imported derivative is linked

- **WHEN** the live manifest holds an asset whose original name matches an unpublished derivative's HEIC stem
- **THEN** that derivative is marked published with the asset's uuid, and a derivative with no matching asset is left unpublished

#### Scenario: Empty manifest aborts

- **WHEN** `pm link` queries the manifest and it is empty
- **THEN** it aborts without changing any derivative
