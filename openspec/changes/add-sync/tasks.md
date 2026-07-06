## 1. Index: watermark storage

- [x] 1.1 Add `sync_state` table to the schema in `internal/index/index.go` (singleton row, `id INTEGER PRIMARY KEY CHECK (id = 1)`, `since_date TEXT NOT NULL`)
- [x] 1.2 Add `LastSyncSince() (time.Time, bool, error)` returning `ok = false` when the table is empty
- [x] 1.3 Add `SetSyncSince(time.Time) error` as an upsert (`INSERT ... ON CONFLICT(id) DO UPDATE`), storing the date as `YYYY-MM-DD`

## 2. Export: extract callable core

- [x] 2.1 Extract the body of `cmdExport` (`cmd/pm/export.go`, from config/index setup onward) into a standalone `runExport(idx *index.Index, cfg config.Config, sinceDate time.Time, dryRun bool, logf func(string, ...any), showProgress bool) (generated, skipped, failed int, err error)` — named `runExport` (not `export`) to avoid colliding with the imported `internal/export` package
- [x] 2.2 Reduce `cmdExport` to: parse flags, load config/idx, call `runExport(...)`, print the existing summary line (printing kept inside `runExport`, matching `publish`'s existing pattern)
- [x] 2.3 Confirm `pm export`'s CLI behavior and output are unchanged after the extraction (ran `pm sync --dry-run --debug` against a sandbox library/db with one canonical archive frame; export's log lines and summary format matched pre-extraction behavior exactly — "found 1 frame(s)...", "Would export...", "Would export 1 HEIC(s) in 0s; skipped 0 already generated.")

## 3. Publish: surface failure count

- [x] 3.1 Change `publish(...)` (`cmd/pm/publish.go:113`) to return its already-computed `failed` count alongside `error`
- [x] 3.2 Update `cmdPublish` to keep printing the same summary using the returned value (unchanged; it already prints internally, only the call site was adjusted for the new return arity)

## 4. New `sync` command

- [x] 4.1 Create `cmd/pm/sync.go` with `cmdSync(args []string) error`: flags for common (`-L`/`--library`, `--db`, `--debug`), `--dry-run`, `--since`, `--photos-library`, `--batch-size`, `--settle` (no `--stage`)
- [x] 4.2 Resolve `sinceDate`: explicit `--since` wins; else `idx.LastSyncSince()`; else zero value
- [x] 4.3 Call `runExport(...)` then `publish(...)` in-process, sharing the opened `idx`/`cfg`
- [x] 4.4 If `!dryRun` and both steps report zero failures, call `idx.SetSyncSince(today)`; otherwise leave the watermark untouched
- [x] 4.5 Print a combined summary: `runExport`/`publish` already print their own per-step summary lines; `sync` adds one final line reporting whether the watermark advanced (and to what date) or was left in place

## 5. Wiring and docs

- [x] 5.1 Add `case "sync": err = cmdSync(args[1:])` to the dispatch switch in `cmd/pm/main.go`
- [x] 5.2 Add a `pm sync [flags]` line and its flags to the `usage` const in `cmd/pm/main.go`
- [x] 5.3 Document `pm sync` in `README.md`'s workflow section (first run is a full backfill; later runs are automatic and incremental; `--since` still available as an override)

## 6. Verification

- [x] 6.1 `go build ./... && go vet ./...`
- [x] 6.2 Against a sandbox library/db (never `/Volumes/Photos`), ran `pm sync --dry-run --debug -L <sandbox> --db <sandbox.db>`; confirmed the first run full-scans (`sync: no stored watermark, full scan`) and that `--dry-run` wrote nothing at all (`sync_state`, `derivative`, and `photos_manifest` all empty afterward)
- [x] 6.3 / 6.4 — **deviation from the literal task**: driving a real non-dry-run `pm sync` all the way through `publish` would have imported a test file into the live Apple Photos library (no disposable Photos library was available to point `--photos-library` at, and the ambient-library fallback is the user's real one). Instead of touching it, extracted `runSync` (mirroring the existing `cmdX`/`x(...)` split) so it takes a `photos.Library` interface, and added `TestSyncAdvancesWatermarkOnCleanRun` / `TestSyncDryRunLeavesWatermarkUntouched` (`cmd/pm/sync_test.go`) plus `TestSyncSinceRoundTrip` (`internal/index/index_test.go`), using the existing `fakeLibrary` test double to verify the watermark writes and reads without any real Photos or filesystem I/O beyond a temp dir
- [x] 6.5 Forced a failure two ways: (a) real CLI run of `pm sync --photos-library /nonexistent/fake.photoslibrary` against the sandbox — failed cleanly before writing anything, `sync_state` confirmed empty afterward; (b) `TestSyncLeavesWatermarkOnPublishFailure`, using `fakeLibrary{rejects: ...}` to simulate Photos rejecting one file — confirms the watermark stays unset and the rejected derivative stays unpublished for the next `sync` to retry

## 7. `--set-since`: seed the watermark without running

- [x] 7.1 Add `--set-since DATE` to `cmd/pm/sync.go`: when set, parse the date, upsert it via `idx.SetSyncSince`, print confirmation, and return without calling `runExport`/`publish`; `--dry-run` reports the date without writing
- [x] 7.2 Reject `--since` and `--set-since` together with a clear error
- [x] 7.3 Document `--set-since` in `main.go`'s `usage` const and `README.md`'s Sync section
- [x] 7.4 Add tests: `TestSyncSetSinceSeedsWatermarkWithoutRunning`, `TestSyncSetSinceDryRunWritesNothing`, `TestSyncSinceAndSetSinceAreMutuallyExclusive` (`cmd/pm/sync_test.go`)
