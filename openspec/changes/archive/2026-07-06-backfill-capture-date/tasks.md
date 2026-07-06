## 1. Export derivative fallback

- [x] 1.1 In `internal/export/generate.go` `Generate`, after the existing `-tagsFromFile` exiftool call, parse the capture date from `stem` with `organize.ParseStem` and format it as `YYYY:MM:DD HH:MM:SS`.
- [x] 1.2 Add a second exiftool call `exiftool -overwrite_original -q -wm cg -DateTimeOriginal=<stem> -CreateDate=<stem> dst` so the stem date is written only when the source carried none (never overwriting a real date).
- [x] 1.3 Handle exiftool's exit status when the guarded write is a no-op (source already had the tags) so it is not treated as a failure. (Verified: `-wm cg` exits 0 whether it writes or leaves unchanged, so the existing `err != nil` check suffices.)

## 2. Recanon sidecar backfill

- [x] 2.1 In `cmd/pm/recanon.go`, after each frame file is renamed to its canonical name, detect whether the file embeds a capture date (reuse `internal/exif`).
- [x] 2.2 For files with no embedded date, write `<canonical-basename>.xmp` beside the file via exiftool, carrying the stem date as `XMP-exif:DateTimeOriginal`, `XMP-photoshop:DateCreated`, and `XMP-xmp:CreateDate`.
- [x] 2.3 Skip the sidecar write under `--dry-run`, and skip files that already embed a date.
- [x] 2.4 Derive the sidecar's date from the same `organize.ParseStem` result used for the rename, so sidecar and stem always agree.

## 3. Tests

- [x] 3.1 In `internal/export/generate_test.go` (reuse `writeJPEG`/`requireBinary`): assert a no-EXIF source + canonical stem yields a HEIC whose `DateTimeOriginal` equals the stem date.
- [x] 3.2 Assert a source that already embeds a date keeps its own date (fallback does not clobber).
- [x] 3.3 Add a `recanon` test: a no-EXIF file gets a sidecar with the stem date and the original bytes are unchanged; a file that embeds a date gets none; `--dry-run` writes nothing.

## 4. End-to-end verification

- [x] 4.1 Against a sandbox library only (`-L`/`--db`, never `/Volumes/Photos`): recanon a no-EXIF JPEG, export it, and assert the derivative's date via exiftool and that the sidecar exists. (Verified: sidecar and HEIC both carry `2005:06:01 00:00:00`; original bytes stay undated.)
- [x] 4.2 Update `README.md`/`CHANGELOG.md` per repo conventions to note the capture-date guarantee and sidecar behavior.
