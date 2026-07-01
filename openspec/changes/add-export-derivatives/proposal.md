## Why

The archive holds full-resolution masters (RAF, camera JPEG, baked Capture One edits) that are
too large to browse or share from a phone. The presentation tier is small HEIC derivatives —
chronologically sortable and a faithful local mirror of what will live in the cloud catalog.
`export` is the outbound-to-disk hop: it turns archive frames into `Export/` HEICs and records
them, so republish stays incremental. It stops at disk; `publish` pushes those HEICs into Apple
Photos as a separate verb.

## What Changes

- New `pm export` command that, for each archive frame, generates presentation HEICs into
  `Export/YYYY/MM` and records them in the index.
- Frame grouping and stem/suffix parsing keyed off the known canonical stem
  (`YYYY-MM-DD--HH-MM-SS-<original>`), not naive hyphen-splitting: `<stem>.<raw>` is the master,
  `<stem>.JPG` the camera JPEG, `<stem>.HEIC` an iPhone-origin frame, `<stem>-<suffix>.<img>` an
  edit.
- Source resolution: one **base** derivative (from the sibling camera JPEG, or the embedded
  `JpgFromRaw` when RAW-only) plus one derivative **per baked edit**; edits are additive and
  never suppress the base; iPhone-origin frames yield no derivative.
- HEIC generation via `sips` (long edge 4096 px default, configurable; quality ~70) with
  `exiftool` carrying `DateTimeOriginal`/GPS/orientation and stamping `catalogKey` (version id)
  and `catalogStem` (frame id).
- Persistent `Export/` naming: `<stem>.heic` from the base, `<stem>-<suffix>.heic` per edit; the
  tree persists as a regenerable, backup-excluded mirror.
- New `derivative` index table making export incremental: a source content hash already generated
  is skipped. `export` writes `source_hash`, `stem`, `source_kind`, `heic_path`, `generated_at`;
  the `photos_uuid` / `published_at` columns are left null for `publish` to fill.
- `--since <date>` to scope a run; `--dry-run` to write nothing.

## Capabilities

### New Capabilities
- `export`: generating presentation-tier HEIC derivatives from archive frames into `Export/`,
  with per-derivative version identity and incremental, idempotent regeneration.

### Modified Capabilities
<!-- None: export is new. `publish` (Export/ → Apple Photos) is a separate capability. -->

## Impact

- `internal/index`: new `derivative` table and put/lookup methods (`source_hash` unique, `stem`,
  `source_kind`, `heic_path`, `generated_at`, plus nullable `photos_uuid` / `published_at` filled
  by `add-publish`).
- New `internal/export` (or similar) package: frame grouping / stem parsing, source resolution,
  and the sips+exiftool generation pipeline.
- `cmd/pm`: new `export` command and usage text.
- Depends on `rename-to-photo-management` (binary `pm`, module path). Precedes `add-publish`,
  which reads the `derivative` rows and `Export/` HEICs this change writes.
- New runtime tool: `sips` (macOS built-in). `exiftool` is already a dependency.
