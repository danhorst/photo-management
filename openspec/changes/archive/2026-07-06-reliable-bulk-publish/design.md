## Context

`publish` selects unpublished derivatives and, per derivative, runs `osxphotos import <file> --report <tmp>`, reading the first uuid out of the report. At whole-library scale this fails two ways: Photos degrades under thousands of back-to-back import sessions and rejects the rest, and a rejected import can still leave a uuid in the report, so `MarkPublished` fires on a file Photos never kept. The index then disagrees with Photos, and there is no path back.

`osxphotos import` is **AppleScript-driven**: it imports through the `photoscript` bridge (`import_cli.py:1361`, `PhotosLibrary().import_photos(...)`) and enumerates the library to read back each new uuid. A crash report from a failed shakedown run showed Photos hung 52s in `NSCountCommand` (an AppleScript `count`) on a process only 58s old — i.e. AppleScript against a freshly launched, still-loading library hangs. So the degradation is driven by AppleScript churn against Photos, and it is worst when Photos is cold. Keeping Photos warm — never quitting it mid-run — is the mitigation, not restarting it.

osxphotos's import report (JSON) carries one record per file with `imported: bool`, `error: bool`, `uuid: str`, and `filename` (the basename). That is enough to tell a real import from a rejection.

## Goals / Non-Goals

**Goals:**
- Push tens of thousands of derivatives without Photos degrading.
- Never mark a derivative published unless osxphotos reports it actually imported.
- Give a way to re-sync the index to what Photos actually holds after deletions or phantom rows.

**Non-Goals:**
- Changing export, pull, import, or the natural-key / `--since` selection logic.
- Deleting Photos assets or derivative files (reconcile only clears the published marker).
- Fixing the capture date of assets already imported (that is a Photos-side timewarp, out of scope here).

## Decisions

**Batch the import; keep Photos warm, pause between batches.**
`ImportBatch(paths []string)` runs one `osxphotos import path1 path2 …` per batch, cutting thousands of AppleScript sessions to a handful and letting osxphotos import a group as it is designed to. Photos.app stays running for the whole run; between batches publish pauses (`--settle`, default 2s) so Photos' background import queue drains while the app stays warm and responsive to AppleScript. Batch size is a `--batch-size` flag (default a few hundred) so both can be tuned without a rebuild.
Alternative considered and rejected: quit/relaunch Photos between batches. A shakedown showed this is actively harmful — the next batch cold-launches Photos and fires AppleScript at a still-loading library, which hangs (see Context). Warmth, not freshness, is what keeps Photos responsive.

**Trust the report's `imported` flag, not any uuid.**
`ImportBatch` returns a per-path result; a path is a success only when its record has `imported` true, `error` false, and a uuid. Everything else (error record, or missing from the report) is a failure the caller leaves unpublished. This is strictly safer than the current "first uuid wins" and closes the false-published class. Report records map back to input paths by basename, which is unique across a batch (canonical stems are unique).

**Reconcile is a separate, guarded verb.**
`pm reconcile` is deliberate and explicit rather than folded into publish, because it clears state based on the live manifest and a bad manifest is dangerous. It refuses an empty manifest outright. Because it only clears the published marker (never deletes anything), the worst case of over-clearing is a redundant re-import, not data loss — so the empty-manifest guard is sufficient without a fuzzy fractional threshold.

## Risks / Trade-offs

- Publish relies on Photos.app already being open on the target library and warm; a cold or wrong-library Photos still misbehaves → documented as a precondition, and the existing `--photos-library` verify guard catches the wrong-library case before any import.
- A very long run could still let Photos' background queue outpace a fixed `--settle` pause → the pause is tunable, and each batch's failures stay unpublished for a resumable re-run, so a stall costs a retry, not progress.
- A partially-truncated (non-empty) manifest could still over-clear → mitigated by reconcile only clearing the (recoverable) published marker, plus the empty guard for the catastrophic case.
- Batch import means a single `osxphotos` failure can affect a whole batch → per-file report parsing still records the successes in the batch, and failures stay unpublished for the next run, so no progress is lost.
- osxphotos report schema could change across versions → the fields used (`imported`, `error`, `uuid`, `filename`) are stable public report columns; parsing tolerates missing optional fields.
