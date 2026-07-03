## Why

The canonical stem `YYYY-MM-DD--HH-MM-SS-<original>` assumes every frame has a second-precise capture time, but some do not.
Film scans and hand-filed frames often have only an approximate date — a batch of 106 scan JPEGs sits in `2021/12/` with no timestamp prefix at all, and `pm export` skips them as "non-canonical."
The archive already holds 7 files with a bare `YYYY-MM-DD-<original>` day stem that `export` tolerates but `publish`/`pull` silently reject.
There is no honest way to name a frame whose date is known only to the day or the month, and no tool to catalogue the stragglers.

## What Changes

- Introduce two reduced date precisions for canonical stems: day (`YYYY-MM-DD-<original>`) and month (`YYYY-MM-<original>`), alongside the existing second precision.
- Give the archive one shared stem grammar that mints and parses all three precisions, so `export`, `publish`, and `pull` accept reduced-precision frames instead of skipping or rejecting them.
- Add a `pm recanon` subcommand that finds non-canonical frames in the `YYYY/` tree and renames them to a reduced-precision canonical name derived from their folder, honoring `--dry-run`.
- Scope is code-path A only: the `Unsorted/` metadata backlog is deferred to a separate change.

## Capabilities

### New Capabilities
- `archive-naming`: the canonical stem grammar — how a frame's date and original name are minted into and parsed from `YYYY-MM-DD--HH-MM-SS-<original>` and its day/month reduced-precision forms, and the precedence that disambiguates them.
- `recanon`: the straggler cataloguer — how `pm recanon` selects non-canonical frames under `YYYY/`, derives a fallback date, renames to a reduced-precision canonical name, and stays idempotent and index-consistent.

### Modified Capabilities
<!-- None: the canonical-naming behavior is not yet an archived spec in openspec/specs/ (it lives inside the unarchived formalize-import change), so this change introduces it fresh rather than modifying an existing spec. -->

## Impact

- `internal/organize` — gains the precision type, a `ParseStem` parser, and a precision-aware `DestAt`; existing `Dest` preserved as the second-precision case.
- `internal/export/frame.go` and `internal/photos/photos.go` — their local stem parsers delegate to `organize`, so reduced-precision frames export and match.
- `cmd/pm` — new `recanon.go` command plus dispatch and usage in `main.go`.
- `internal/index` — `recanon` keeps the BLAKE3 index path rows consistent on rename (or falls back to prompting `pm index`).
- No change to the Capture One catalog; `recanon` only moves and renames files on disk, like every other verb.
