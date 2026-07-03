## 1. Stem grammar in organize

- [x] 1.1 Add `Precision` type (`Second`, `Day`, `Month`) to `internal/organize/organize.go`
- [x] 1.2 Add `ParseStem(stem) (t, prec, original, ok)` with precedence second → day → month and strict numeric guards
- [x] 1.3 Add `DestAt(library, t, prec, origName)`; keep `Dest` as the `Second` case
- [x] 1.4 Tests in `organize_test.go`: round-trip each precision, precedence disambiguation, the `NN-`/`HH-MM-SS-` ambiguity cases, junk rejection

## 2. Delegate existing parsers

- [x] 2.1 `internal/export/frame.go` — `Frame.CaptureDate` delegates to `organize.ParseStem`, accepting day/month
- [x] 2.2 `internal/photos/photos.go` — `ParseStem` delegates to `organize.ParseStem`; drop the local copy
- [x] 2.3 Confirm no `photos → organize` import cycle; if present, relocate the parser to a leaf package
- [x] 2.4 Tests: `frame_test.go` day/month `CaptureDate`; `photos_test.go` full stems still match and day/month now parse

## 3. pm recanon command

- [x] 3.1 Add `cmd/pm/recanon.go`: walk `YYYY/` via `collectArchive` + `export.Group`, select frames failing `organize.ParseStem`
- [x] 3.2 Derive month precision from the folder path; mint with `organize.DestAt(...Month...)`; rename via `organize.Place`
- [x] 3.3 Wire flags: honor global `--dry-run`; add `--match <substr>` and `--date YYYY-MM-DD` (day precision)
- [x] 3.4 Keep the index consistent on rename (update the path row via `internal/index`, or print a `pm index` reminder)
- [x] 3.5 Dispatch `recanon` and add usage text in `cmd/pm/main.go`
- [x] 3.6 Test with a temp library reproducing the `IMG_####-ZF-...` batch: dry-run lists, apply renames, second run is a no-op

## 4. Verification

- [x] 4.1 `make test` green
- [x] 4.2 End-to-end on a sandbox library via `-L`/`--db` (never `/Volumes/Photos`): seed a `2021/12/IMG_####-ZF-...` batch, `pm recanon --dry-run`, `pm recanon`, then `pm export` shows the non-canonical count at 0
- [ ] 4.3 Read-only re-check on the live library that the non-canonical count is still 106 before any real remediation (deferred to DBH — pairs with the actual live remediation; not auto-run against `/Volumes/Photos`)
