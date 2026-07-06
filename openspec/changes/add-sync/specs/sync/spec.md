## ADDED Requirements

### Requirement: `sync` chains export and publish behind one command

`pm sync` SHALL run the equivalent of `export` followed by `publish` in-process, sharing one index and config, without requiring two separate invocations.
It SHALL accept the flags that matter for a recurring run (`--dry-run`, `--since`, `--photos-library`, `--batch-size`, `--settle`) plus the common flags (`-L`/`--library`, `--db`, `--debug`).
It SHALL NOT expose `--stage`, since that is the one-time native-seed migration path, not the recurring workflow.

#### Scenario: A plain sync run exports then publishes

- **WHEN** `pm sync` runs with new files in the archive that have not yet been exported or published
- **THEN** it generates derivatives for those files and then publishes the newly generated (and any previously generated but unpublished) derivatives, in a single command invocation

### Requirement: The sync cutoff is remembered automatically between runs

`sync` SHALL resolve its capture-date cutoff in this order: an explicit `--since` flag on this invocation; otherwise a stored watermark from the last clean run; otherwise no cutoff (a full scan, matching today's default `export`/`publish` behavior with no `--since`).
The stored watermark SHALL persist in the index database so it survives across separate invocations of `pm sync`.

#### Scenario: First-ever sync does a full scan

- **WHEN** `pm sync` runs against an index with no stored watermark
- **THEN** it behaves as `export`/`publish` do today with no `--since`: every not-yet-exported or not-yet-published file is considered, regardless of capture date

#### Scenario: Later sync only considers files at or after the watermark

- **WHEN** `pm sync` runs against an index with a stored watermark of `2026-07-01`
- **THEN** it only considers frames captured on or after `2026-07-01`, the same as passing `--since 2026-07-01` to `export` and `publish` today

#### Scenario: An explicit `--since` overrides the stored watermark for that run

- **WHEN** `pm sync --since 2026-01-01` runs against an index with a stored watermark of `2026-07-01`
- **THEN** that run considers frames captured on or after `2026-01-01`, ignoring the stored watermark for cutoff purposes on this run only

### Requirement: The stored watermark only advances after a clean run

`sync` SHALL advance the stored watermark to the current date only when neither the export step nor the publish step reports any failures during that run.
`sync` SHALL leave the stored watermark unchanged when either step reports one or more failures, so the next `sync` run retries the same window.
`sync` SHALL NOT modify the stored watermark when run with `--dry-run`.

#### Scenario: A clean run advances the watermark

- **WHEN** `pm sync` runs on 2026-07-10 and every file it considers exports and publishes successfully
- **THEN** the stored watermark is updated to 2026-07-10, so the next automatic `sync` only considers frames captured on or after that date

#### Scenario: A failed item keeps the watermark from advancing

- **WHEN** `pm sync` runs and one derivative fails to publish (e.g. Photos rejects the file)
- **THEN** the stored watermark is left at its previous value, so the next `sync` run reconsiders the same window; already-succeeded files are skipped again via existing dedup and only the failed item is retried

#### Scenario: A dry run never changes the watermark

- **WHEN** `pm sync --dry-run` runs, regardless of what it reports it would do
- **THEN** the stored watermark is left exactly as it was before the run

### Requirement: The watermark can be seeded directly without running export or publish

`sync` SHALL accept a `--set-since` flag that writes the stored watermark to the given date and exits, without invoking export or publish.
This SHALL let a user who has already brought the archive and Photos library into agreement by other means (manual `export`/`publish` runs, or the native-seed workflow) start automatic `sync` runs from a chosen date instead of a full scan.
`--set-since` and `--since` SHALL be mutually exclusive on the same invocation.
`--set-since` combined with `--dry-run` SHALL report the date that would be set without writing it.

#### Scenario: Seeding the watermark before the first automatic sync

- **WHEN** `pm sync --set-since 2026-07-01` runs
- **THEN** the stored watermark is set to `2026-07-01` and the command exits without running export or publish; the next `pm sync` only considers frames captured on or after that date

#### Scenario: Dry-run seeding reports without writing

- **WHEN** `pm sync --set-since 2026-07-01 --dry-run` runs
- **THEN** the command reports the date it would set, and the stored watermark is left exactly as it was before the run

#### Scenario: `--since` and `--set-since` together are rejected

- **WHEN** `pm sync --since 2026-01-01 --set-since 2026-07-01` runs
- **THEN** the command returns an error and neither runs export/publish nor writes the watermark
