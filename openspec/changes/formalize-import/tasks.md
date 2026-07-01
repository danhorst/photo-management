## 1. Author the import spec

- [ ] 1.1 Write `specs/import/spec.md` capturing dedup, organization, filtering, move/copy, and dry-run

## 2. Behavior-match audit

- [ ] 2.1 Confirm dedup keys on BLAKE3 content hash and skips silently (`internal/hash`, `internal/index`, `cmdImport`)
- [ ] 2.2 Confirm the naming/date path: `YYYY/MM/YYYY-MM-DD--HH-MM-SS-<original>`, `DateTimeOriginal`→`CreateDate`→`Unsorted/` (`internal/exif`, `internal/organize`)
- [ ] 2.3 Confirm the managed-extension set and that `._` sidecars and the index db are ignored
- [ ] 2.4 Confirm move-when-same-volume / copy-when-cross-volume (`internal/volume`, `cmdImport`)
- [ ] 2.5 Confirm `--dry-run` writes nothing — no files moved, no index or volume records changed
- [ ] 2.6 File any behavior mismatch as a separate bug rather than adjusting the spec to match a defect

## 3. Verify

- [ ] 3.1 `openspec validate formalize-import --strict` passes
- [ ] 3.2 Cross-check the spec against `project.md`'s Critical behavior for consistency
