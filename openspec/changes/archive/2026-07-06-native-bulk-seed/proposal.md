## Why

A whole-library `pm publish` cannot complete. `osxphotos import` drives Photos over AppleScript (the `photoscript` bridge), and after ~2,000–2,500 imports Photos' import subsystem degrades and rejects every remaining file — valid or not — with a generic PhotoKit "unrecognizable file format" error. A crash report confirmed the mechanism: Photos hung 52s in an AppleScript `count` on a freshly-forked process. The degradation is sticky (pausing iCloud, draining, and resuming published zero) and only a full Photos restart clears it. We verified the wall is positional, not file-related: within one series the cutoff was clean (files 001–035 imported, 036+ failed) and file 036 is byte-structurally identical to 001.

Building durable restart-with-warmup machinery into the tool to survive a **one-time** migration is the wrong trade — permanent complexity for a transient problem. `osxphotos`-based `pm publish` already works for the steady-state incremental case, where batches never approach the wall.

## What Changes

- `pm publish` gains a `--stage DIR` mode: instead of importing, it hardlinks each unpublished derivative into `DIR/YYYY/MM/<basename>` (mirroring `Export/`), so the user can seed Photos via its native folder import — a first-party bulk path that does not hit the AppleScript wall. It reuses publish's exact selection (so frames already in Photos are associated, not staged) and marks nothing published.
- A new `pm link` verb reconnects the index to a native import: it matches each unpublished derivative to a live Photos asset by filename (`original_filename == <stem>.heic`, an exact unique key) and marks it published. It refuses an empty manifest and never clobbers an existing association.

## Capabilities

### New Capabilities

- `native-seed`: seeding Apple Photos with a large derivative set via native folder import — staging the unpublished derivatives as a hardlink tree, then linking the resulting assets back into the index by filename so future publishes skip them.

### Modified Capabilities

<!-- None: publish/photos behavior is not yet formalized in openspec/specs/, so staging and linking are introduced under a new capability. -->

## Impact

- `cmd/pm/publish.go`: `--stage DIR` flag; phase-2 branches to a hardlink loop when set, skipping `ImportBatch` and the published marks.
- `cmd/pm/link.go` (new): the `pm link` verb; wired into `main.go` dispatch and usage.
- `cmd/pm/main.go`: `link` dispatch, usage line, `--stage` doc.
- `cmd/pm/link_test.go` (new), `cmd/pm/publish_test.go`: link tests and a stage-hardlink test.
- Unchanged: `export`, `pull`, `import`, `reconcile`, and the `osxphotos` publish path (batching, `--settle`, verified-import) from `reliable-bulk-publish`.
