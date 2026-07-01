## 1. Index: derivative table

- [ ] 1.1 Add `derivative(source_hash PRIMARY KEY, stem, source_kind, heic_path, generated_at, photos_uuid NULL, published_at NULL)` to the schema in `internal/index`
- [ ] 1.2 Add `PutDerivative` (upsert by `source_hash`, setting `generated_at`) and `HasDerivative(sourceHash)` / lookup methods
- [ ] 1.3 Unit-test derivative round-trip and that a repeat `source_hash` is skipped, not duplicated

## 2. Frame grouping and source resolution

- [ ] 2.1 Add a stem parser that, given the canonical stem, classifies siblings: master `<stem>.<raw>`, camera JPEG `<stem>.JPG`, iPhone-origin `<stem>.HEIC`, edit `<stem>-<suffix>.<img>`
- [ ] 2.2 Resolve base source: sibling camera JPEG, else embedded `JpgFromRaw` extracted from the RAF via exiftool
- [ ] 2.3 Resolve edit sources: every `<stem>-<suffix>` baked file; base is always included and never suppressed
- [ ] 2.4 Skip iPhone-origin frames (a `<stem>.HEIC` with no camera JPEG) — no derivative
- [ ] 2.5 Unit-test the parser/resolver: RAF+JPEG, RAW-only, multiple edits, iPhone-origin, and stems that themselves contain hyphens

## 3. HEIC generation pipeline

- [ ] 3.1 Compute the version id (BLAKE3 of the source file) per base/edit source
- [ ] 3.2 Transcode/resize to HEIC via `sips` — long edge 4096 (configurable), quality ~70
- [ ] 3.3 Carry `DateTimeOriginal`/GPS/orientation and stamp `catalogKey` (version id) + `catalogStem` (frame id) via exiftool
- [ ] 3.4 Write to `Export/YYYY/MM/<stem>.heic` (base) or `<stem>-<suffix>.heic` (edit); record a `derivative` row
- [ ] 3.5 Skip any source whose `source_hash` is already in `derivative` (incremental export)

## 4. CLI

- [ ] 4.1 Add `export` to the dispatch in `cmd/pm` with `--since <date>`, `--dry-run`, and shared `--db`/`-L`
- [ ] 4.2 `cmdExport` walks archive frames (optionally filtered by `--since`), runs generation, prints a summary of HEICs written / skipped
- [ ] 4.3 `--dry-run` reports intended derivatives and writes no files or rows

## 5. Config and docs

- [ ] 5.1 Add configurable long-edge and quality (default 4096 / ~70) to the config surface
- [ ] 5.2 Document `pm export`, `Export/` layout, and backup-exclusion in `README.md`
- [ ] 5.3 `CHANGELOG.md` entry

## 6. Verify

- [ ] 6.1 `go test ./...` passes
- [ ] 6.2 `openspec validate add-export-derivatives --strict` passes
- [ ] 6.3 Manual: against a throwaway `--db`/`-L` sandbox, export a frame group (RAF+JPEG plus an edit), confirm `<stem>.heic` and `<stem>-<suffix>.heic` land in `Export/` with `catalogKey`/`catalogStem` stamped, and a re-run skips both
