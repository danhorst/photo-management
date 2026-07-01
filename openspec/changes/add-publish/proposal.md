## Why

`export` gets presentation HEICs onto disk in `Export/`, but the point is to browse and share
them from the phone and sync onward to Google. `publish` is the last outbound hop: it pushes the
`Export/` HEICs into Apple Photos — without creating duplicates, either of our own prior pushes
or of frames already present (manually added, or iPhone-origin frames pulled into the archive).
It is one discrete step; `export` precedes it, and a later `sync` can chain the two.

## What Changes

- New `pm publish` command that imports `Export/` HEICs (those recorded by `export` but not yet
  pushed) into Apple Photos via `osxphotos`, as a flat import with no album creation.
- Dedup layer 1 (our pushes): skip any derivative whose `photos_uuid` is already set; on a
  successful import, record the returned `photos_uuid` and `published_at` on the `derivative` row.
- Dedup layer 2 (pre-existing overlap): before importing, build a manifest of current Photos
  assets via `osxphotos` and match each frame on a natural key (`DateTimeOriginal` + original
  filename, both encoded in the archive filename). A match means the frame is already in Photos —
  skip the import and record the association. This also covers iPhone-origin frames.
- New `photos_manifest` cache table backing layer 2.
- No-supersede: an edit imports as a new asset; the tool never deletes or replaces prior assets.

## Capabilities

### New Capabilities
- `publish`: pushing exported `Export/` HEICs into Apple Photos, idempotently, via two dedup
  layers (our own version-id pushes, and natural-key overlap with pre-existing assets), never
  superseding or deleting.

### Modified Capabilities
<!-- None: publish is a distinct verb from export. -->

## Impact

- `internal/index`: new `photos_manifest` table (`uuid`, `original_filename`, `capture_time`,
  `catalog_key`, `last_synced`); `derivative.photos_uuid` / `published_at` written on push.
- New `internal/photos` (or similar): osxphotos manifest query, natural-key matcher, and Photos
  import wrapper.
- `cmd/pm`: new `publish` command reading the `derivative` rows / `Export/` HEICs that `export`
  produced.
- New runtime dependency: `osxphotos`.
- Depends on `add-export-derivatives` (reads its `derivative` rows and `Export/` HEICs) and on
  `rename-to-photo-management` (binary `pm`).
