## 1. Index: photos manifest cache

- [ ] 1.1 Add `photos_manifest(uuid PRIMARY KEY, original_filename, capture_time, catalog_key, last_synced)` to the schema in `internal/index`
- [ ] 1.2 Add put/replace and query methods (upsert a manifest row; look up by natural key `original_filename` + `capture_time`)
- [ ] 1.3 Add a method to set `photos_uuid` + `published_at` on a `derivative` row, and a query for derivatives not yet pushed (`photos_uuid IS NULL`)
- [ ] 1.4 Unit-test manifest upsert and natural-key lookup

## 2. Apple Photos integration

- [ ] 2.1 Build a Photos manifest via `osxphotos` and refresh `photos_manifest` at the start of a publish run
- [ ] 2.2 Implement the natural-key matcher (`DateTimeOriginal` + original filename from the archive filename) against the manifest
- [ ] 2.3 Implement the `osxphotos` import wrapper (flat import, no album), returning the new asset's `photos_uuid`
- [ ] 2.4 Unit-test the matcher (match, no-match, iPhone-origin already-present) with a plain function separate from the osxphotos shell-out

## 3. Publish path

- [ ] 3.1 Add `publish` to the dispatch in `cmd/pm` with `--dry-run` and shared `--db`/`-L`; `cmdPublish` selects `derivative` rows with `photos_uuid IS NULL`
- [ ] 3.2 For each such derivative: skip if `photos_uuid` is already set (layer 1)
- [ ] 3.3 Else if the frame matches the manifest natural key, skip the import and record the association (layer 2)
- [ ] 3.4 Else import the HEIC into Apple Photos and record the returned `photos_uuid` + `published_at` on the derivative row
- [ ] 3.5 Never delete or replace an existing asset; an edit imports as a new asset
- [ ] 3.6 `--dry-run` reports intended imports/skips and writes nothing

## 4. Docs

- [ ] 4.1 Document the Apple Photos import, the two dedup layers, and no-supersede in `README.md`
- [ ] 4.2 Document the `osxphotos` dependency and that no delete is possible by Apple design
- [ ] 4.3 `CHANGELOG.md` entry

## 5. Verify

- [ ] 5.1 `go test ./...` passes (matcher and index covered; osxphotos calls behind an interface)
- [ ] 5.2 `openspec validate add-publish --strict` passes
- [ ] 5.3 Manual: against a throwaway `--db`/`-L` sandbox and a test Photos library, publish new derivatives (imported once), re-run (skipped by layer 1), and confirm a frame already in Photos is skipped by layer 2
