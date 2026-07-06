## 1. Stage mode in publish

- [x] 1.1 Add a `--stage DIR` flag to `cmdPublish` and thread it into `publish`.
- [x] 1.2 In `publish`, when `stageDir` is set, replace phase-2 with a hardlink loop: for each `toImport` derivative, `os.MkdirAll` the `DIR/YYYY/MM` parent and `os.Link` the HEIC to `DIR/<rel-under-Export>`; skip an existing target; mark nothing published.
- [x] 1.3 On a cross-volume `os.Link` error, fail with a message naming the library volume; keep the summary line honest (staged count, next-step hint).

## 2. Link verb

- [x] 2.1 Add `cmd/pm/link.go`: `cmdLink` + testable `link(idx, lib, logf, showProgress, dryRun)`, structured like `reconcile.go`.
- [x] 2.2 Query the manifest; abort on empty; build `original_filename -> uuid`, marking a filename shared by >1 asset ambiguous.
- [x] 2.3 Iterate `UnpublishedDerivatives()`; on a unique basename match call `MarkPublished`; count linked / unmatched / ambiguous; support `--dry-run`.
- [x] 2.4 Wire `link` into `main.go` dispatch and usage; add the `--stage` doc line.

## 3. Tests

- [x] 3.1 `cmd/pm/link_test.go` (mirror `reconcile_test.go`): match links; non-match stays unpublished; empty manifest aborts; `--dry-run` writes nothing; ambiguous filename skipped.
- [x] 3.2 `cmd/pm/publish_test.go`: `TestPublishStageHardlinks` — temp Export + stage dir on one volume; hardlinks for unpublished only, layout mirrored, nothing marked published.

## 4. Docs

- [x] 4.1 Update `README.md` (publish `--stage`, `pm link`, the native-seed flow) and `CHANGELOG.md` per repo conventions.
