## Context

`pm`'s pipeline is import (card → archive) → export (archive → `Export/` HEIC) → publish (`Export/` → Apple Photos). `export` and `publish` already dedup correctly on their own: `export` skips any source whose content hash already has a `derivative` row (`idx.HasDerivative`), `publish` only ever looks at `idx.UnpublishedDerivatives()`. Nothing is ever double-processed regardless of what `--since` is passed. `--since YYYY-MM-DD` on each is a manually-typed capture-date cutoff (`cmd/pm/export.go:27`, `cmd/pm/publish.go:24`, both `time.ParseInLocation("2006-01-02", ...)`), with no stored memory of when either last ran.

Backfill into the archive/Photos is done. Going forward the only recurring need is: pick up whatever's new since last time, without hand-typing a date each run.

## Goals / Non-Goals

**Goals:**
- One command (`pm sync`) that runs export then publish without the user re-typing a matching `--since` on each.
- No argument needed in the common case: the cutoff is remembered automatically between runs.
- Safe under partial failure: a run that fails partway must not cause the next run to silently skip the failed items.

**Non-Goals:**
- `import` (card/queue → archive) and `pull` (iPhone → archive) are not part of `sync` — both require an argument or aim in a different direction, and were explicitly scoped out.
- Import-time tracking. The watermark is a capture-date cutoff, exactly like today's `--since`, not "time this file entered the archive." A photo imported late with an old capture date needs a manual `--since` or full rerun to pick up — this is unchanged from today's behavior, just no longer hand-typed.
- No change to `export`'s or `publish`'s dedup logic, flags, or output format; `--stage` (native-seed bulk migration) is intentionally not exposed through `sync`.

## Decisions

**Watermark storage: one singleton row in a new `sync_state` table**, rather than a generic key-value table. The existing schema (`internal/index/index.go`) has no KV table anywhere — `files`, `media_files`, `volumes`, `derivative`, `photos_manifest` are all explicit typed tables — so a dedicated `sync_state(id INTEGER PRIMARY KEY CHECK (id = 1), since_date TEXT NOT NULL)` matches the existing convention better than introducing a new generic pattern for one value.

**Watermark advances only on a clean run (zero failures in both steps).** Alternative considered: always advance to "today" regardless of failures, and rely on the user noticing failures in the printed summary and manually re-running with `--since`. Rejected — it silently drops failed items out of all future automatic runs once the watermark passes their capture date, with no forcing function to notice. Requiring zero failures to advance means a failure simply reappears in the *next* automatic `sync` too, at negligible cost (dedup skips everything that already succeeded; only the actual failures get retried).

**`export`'s core logic is extracted into a standalone `export(...)` function**, mirroring the split `publish.go` already has between `cmdPublish` (flag parsing) and `publish(...)` (the work, `publish.go:113`). `export.go` today has everything inlined in `cmdExport`. This lets `sync` call both steps as in-process Go functions sharing one `*index.Index` and config, rather than shelling out to `pm export && pm publish` as two OS processes — avoiding a second DB open/close cycle and keeping error handling in one place. Alternative considered: have `sync` exec itself as a subprocess twice; rejected as needless indirection when both are already in the same binary.

**`publish(...)` gains a returned failure count.** It already computes `failed` internally (`publish.go:140` onward) but only prints it; `sync` needs it to decide whether the run was clean enough to advance the watermark.

**`--since` on `sync` is an explicit override for that run only; a clean run still advances the stored watermark to today afterward**, regardless of whether `--since` was defaulted or explicitly passed. This keeps the automatic cadence moving forward from "now," rather than the watermark drifting to whatever date happened to be typed for an unrelated one-off run.

## Risks / Trade-offs

- **[Risk]** A photo captured before the current watermark but imported after it (recovered card, rescanned negative) is silently excluded from automatic `sync` runs. → **Mitigation**: identical to today's manual `--since` behavior; documented in README, and a manual `--since` or omitting the watermark (fresh sandbox/db) forces a full rescan when needed. Explicitly accepted trade-off, not a new regression.
- **[Risk]** Extracting `export`'s logic into a standalone function is a larger mechanical diff than the other pieces. → **Mitigation**: pure extraction, no behavior change — same parallel scan/memoize/generate structure, same progress-bar gating; `cmdExport` keeps its current CLI behavior byte-for-byte.
- **[Risk]** Defining "clean run" only by failure *count* (not identity) means if failures happen on both the very first sync and every run after, the watermark never advances at all. → **Mitigation**: acceptable — that's exactly the intended behavior (never skip past unresolved failures); the user sees the failures printed each time and can intervene (fix and rerun, or investigate).

## Migration Plan

Additive only: new table (`CREATE TABLE IF NOT EXISTS`, applied idempotently like the rest of the schema), new command, no changes to existing table schemas or CLI behavior of `export`/`publish` when invoked directly. No rollback concerns beyond reverting the code; existing indexes need no migration since the table starts empty (first `sync` behaves as a full scan, same as today's no-`--since` default).

## Open Questions

None outstanding — scope, watermark basis, and failure-safety were each explicitly decided with the user before this design was written.
