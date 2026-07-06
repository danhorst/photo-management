## Why

Backfill is done. Going forward, the recurring workflow is: export new derivatives, then publish them, repeated on a cadence — and today that means hand-typing a matching `--since YYYY-MM-DD` on both `pm export` and `pm publish` every time, with nothing remembering what was already covered. A `sync` verb chaining the two steps, with an automatically stored watermark, removes that manual bookkeeping.

## What Changes

- Add a new `pm sync` command that runs `export` then `publish` in-process, sharing one index/config, instead of requiring two separate invocations.
- Add a `sync_state` table to the index DB storing the capture-date cutoff (`since_date`) for the next automatic run.
- `sync` resolves its cutoff as: explicit `--since` override, else the stored watermark, else no cutoff (full scan, matching today's default `export`/`publish` behavior with no `--since`).
- The stored watermark only advances after a clean run (zero failures from both export and publish); a run with any failures leaves it untouched so the next `sync` retries the same window — cheap, since both steps already dedup on content hash / publish state.
- Extract `export`'s core logic out of `cmdExport` into a standalone `export(...)` function (mirroring the existing `cmdPublish`/`publish(...)` split), so `sync` can call both steps as functions rather than shelling out to `pm export && pm publish`.
- `publish(...)` gains a returned failure count (already computed internally, currently only printed) so `sync` can decide whether to advance the watermark.

## Capabilities

### New Capabilities
- `sync`: chains export and publish behind one command, using a stored, self-advancing capture-date watermark instead of a manually supplied `--since`.

### Modified Capabilities

(none — `export`'s and `publish`'s existing behavior and flags are unchanged; the internal split into callable functions is an implementation detail, not a requirements change.)

## Impact

- `internal/index/index.go`: new `sync_state` table, `LastSyncSince`/`SetSyncSince` methods.
- `cmd/pm/export.go`: extract core logic into a callable `export(...)` function; `cmdExport` becomes a thin flag-parsing wrapper.
- `cmd/pm/publish.go`: `publish(...)` returns a failure count.
- `cmd/pm/sync.go` (new): `cmdSync` and the sync orchestration.
- `cmd/pm/main.go`: dispatch `sync` subcommand, update `usage`.
- `README.md`: document the `pm sync` workflow.
