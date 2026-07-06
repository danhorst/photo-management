## Why

A whole-library `pm publish` (tens of thousands of derivatives) overwhelms Apple Photos: after a couple thousand imports Photos degrades and rejects every remaining file with a generic "Cannot Import Item" dialog.
Two things make this worse than it should be.
`publish` calls `osxphotos import` once per file, and `osxphotos` drives Photos over AppleScript (it imports via the `photoscript` bridge and enumerates the library to read back each new uuid), so a large run is thousands of rapid AppleScript sessions against a growing library — the worst case for Photos.
And `Import` records a derivative as published whenever the osxphotos report carries a uuid, without checking the report's `imported`/`error` status, so a rejected import can still be marked published — leaving the index claiming assets Photos never kept.

Deleting assets in Photos (or those phantom rows) also silently drifts the index: `publish` skips anything already marked published, so a deleted frame is never re-imported, and there is no way to reconcile the index back to what Photos actually holds.

## What Changes

- `publish` imports in **batches** — many files per `osxphotos import` call — cutting a large run from thousands of AppleScript sessions to a handful. It keeps Photos.app **running and warm** across the whole run, pausing between batches to let its background queue drain, rather than quitting it (a restart cold-launches the next batch into a still-loading library, which hangs). Batch size and the pause are configurable.
- `Import` becomes `ImportBatch` and records a derivative as published **only** when the osxphotos report marks that file `imported` with no `error` and a uuid; a rejected file is left unpublished for retry, never falsely marked.
- A new `pm reconcile` verb re-syncs the index's published state to the live Photos manifest: any derivative whose recorded uuid is no longer present in Photos is un-marked so the next publish re-imports it. It refuses to run against an empty manifest (which would wrongly un-mark everything).

## Capabilities

### New Capabilities

- `bulk-publish`: publishing a large set of derivatives into Apple Photos reliably — batched imports that keep Photos warm across the run, and published state recorded only on a verified successful import.
- `photos-reconcile`: reconciling the index's published state against the live Photos library, so assets deleted (or never actually kept) are re-queued for publish.

### Modified Capabilities

<!-- None: publish/photos behavior is not yet formalized in openspec/specs/, so these are introduced as new capabilities. -->

## Impact

- `internal/photos/photos.go`: `Library.Import` → `ImportBatch([]string)`; report parsing checks `imported`/`error` per record.
- `cmd/pm/publish.go`: phase-2 loop batches through `ImportBatch`, marking only verified successes, pausing between batches; new `--batch-size` and `--settle` flags.
- `cmd/pm/reconcile.go` (new): the `pm reconcile` verb; wired into `main.go` dispatch and usage.
- `internal/index/index.go`: `PublishedDerivatives()` and `ClearPublished(sourceHash)` to support reconcile.
- `cmd/pm/publish_test.go`: `fakeLibrary` implements `ImportBatch`; new reconcile tests.
- Unchanged: `export`, `pull`, `import`, and the natural-key/`--since` publish logic.
