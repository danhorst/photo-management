## Context

`osxphotos import` is AppleScript-driven (`photoscript`: `PhotosLibrary().import_photos(...)`, plus a library enumeration to read back each new uuid). At whole-library scale Photos' import subsystem degrades after ~2,000–2,500 imports and rejects everything after, and the degradation only clears on a full Photos restart. A restart mid-run is itself unsafe: cold-launching Photos and firing AppleScript at a still-loading library hangs (the observed 52s `NSCountCommand` hang). So a reliable unattended `osxphotos` bulk import would need restart-with-warmup machinery — real, permanent complexity to survive a one-time event.

Photos' own native folder import is a separate first-party pipeline built for bulk (a full camera/SD-card dump) and does not go through the scripted path. It is the natural tool for the one-time seed. The only thing publish gives that native import does not is the index linkage (`derivative.photos_uuid`), and that is recoverable by filename: every asset this tool imported carries `original_filename == <stem>.heic` (verified 4,698/4,786; the 88 exceptions are layer-2 associations to pre-existing assets). Derivative HEIC stems are unique, so the filename is an exact, unambiguous key back to the row.

## Goals / Non-Goals

**Goals:**
- Seed the library with the full unpublished set without hitting the import wall.
- Avoid duplicating the derivatives already in Photos.
- Reconnect the index so steady-state `pm publish` skips the seeded assets.

**Non-Goals:**
- Changing the `osxphotos` publish path — it stays the incremental tool.
- Building restart-with-warmup into publish (explicitly dropped).
- Deleting or de-duplicating assets in Photos.

## Decisions

**Stage as a hardlink tree, reusing publish selection.**
`pm publish --stage DIR` runs publish's phase 1 unchanged (manifest query, `--since`, layer-2 association of frames already in Photos) and, in place of phase 2, hardlinks each `toImport` derivative into `DIR/YYYY/MM/<basename>`. Hardlinks are indistinguishable from real files to Photos and cost no data. Mirroring the `Export/` layout lets the user import year-by-year (natural chunking, and a boundary year like a half-done 2007 is handled per-file, not per-folder). Nothing is marked published — that is `pm link`'s job after the import. `DIR` must be on the library volume; a cross-volume `os.Link` fails with a clear message. Staging is idempotent (skip an existing target).
Alternative rejected: point native import at all of `Export/` and rely on Photos' duplicate detection to skip the ~4,800 already imported. Too risky — any miss means thousands of duplicates, and Photos deletion needs manual confirmation.

**Link by filename, mirroring reconcile.**
`pm link` is the inverse of `reconcile`: reconcile clears rows whose asset vanished; link sets rows whose asset appeared. It queries the manifest (refusing an empty one, same guard), builds `original_filename -> uuid`, and for each `UnpublishedDerivatives()` row marks published on a unique filename match via `MarkPublished`. Only unpublished rows are touched, so associations are never clobbered. A filename shared by two assets (a duplicate that slipped in) is ambiguous — skipped and counted, never guessed.

link needs only uuid and filename, so it uses a field-limited query (`ManifestNames`: `osxphotos query --field uuid --field original_filename`) rather than the full `Manifest`. A full `--json` manifest computes every field (exif, albums, persons) per photo, which after seeding a whole library (tens of thousands of assets) runs many minutes and gigabytes of RSS; the field-limited query skips that work. (publish/pull/reconcile still use the full manifest — slimming those is a separate follow-up, since their matchers need capture date and camera fields.)

## Risks / Trade-offs

- Photos' native importer could itself strain at tens of thousands in one action → mitigated by year-by-year import off the staged tree; verified on one year before the rest.
- A stale manifest could leave rows unlinked → link only *adds* published marks (never clears), so the worst case is an unlinked row that a later `pm publish` re-imports; the empty-manifest guard covers the catastrophic case.
- Native import loses layer-2 dedup against pre-existing assets → staging runs publish's layer-2 first, so overlapping frames are associated and never staged.
- Hardlink tree consumes inodes on the library volume → trivial (no data), and removed after the seed.
