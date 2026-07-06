## 1. Batch import in the photos package

- [x] 1.1 Replace `Library.Import(path)` with `ImportBatch(paths []string) (map[string]ImportResult, error)`, where `ImportResult{UUID string; Err error}` is keyed by input path.
- [x] 1.2 Implement `OSXPhotos.ImportBatch`: one `osxphotos import <paths…> --report <tmp> --no-progress` call; parse the JSON report and, per record, treat a file as success only when `imported` is true, `error` is false, and `uuid` is non-empty. Map records to input paths by basename.
- [x] 1.3 Files absent from the report or marked `error` get an `ImportResult.Err`; a whole-invocation failure (osxphotos couldn't run) returns the top-level error.

## 2. Batched, verified publish loop

- [x] 2.1 Add `--batch-size` and `--settle` flags to `cmdPublish` (defaults a few hundred / 2s) and thread them into `publish`.
- [x] 2.2 Rewrite phase-2 to chunk `toImport` into batches, call `ImportBatch` per chunk, and `MarkPublished` only the verified successes; count `error`/missing as failed.
- [x] 2.3 Pause `--settle` between batches (never quitting Photos); keep the progress bar and the existing summary line accurate.

## 3. Reconcile verb

- [x] 3.1 Add `index.PublishedDerivatives()` (rows with a non-null `photos_uuid`) and `index.ClearPublished(sourceHash)` (null `photos_uuid`/`published_at`).
- [x] 3.2 Add `cmd/pm/reconcile.go` (`cmdReconcile`): query the manifest, abort on an empty manifest, and clear published state for every derivative whose uuid is absent from the manifest; support `--dry-run`.
- [x] 3.3 Wire `reconcile` into `main.go` dispatch and the usage text.

## 4. Tests

- [x] 4.1 Update `fakeLibrary` in `publish_test.go` to implement `ImportBatch`; keep existing publish tests green.
- [x] 4.2 Add a publish test: a batch where one file reports `error` leaves that derivative unpublished (retryable) while the rest are marked published.
- [x] 4.3 Add a reconcile test: a derivative whose uuid is absent from the manifest is cleared; one present is kept; an empty manifest aborts and changes nothing; `--dry-run` writes nothing.
- [x] 4.4 Add a `photos` test parsing a sample osxphotos report to confirm `imported`/`error`/uuid handling.

## 5. End-to-end and docs

- [x] 5.1 Build and drive `pm reconcile --dry-run` against a fake/sandbox setup to confirm counts; verify `publish --batch-size` batches via `--dry-run`/debug logs.
- [x] 5.2 Update `README.md` (publish batching, `pm reconcile`) and `CHANGELOG.md` per repo conventions.
