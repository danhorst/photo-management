## Context

Most frames are shot RAF+JPEG, so a presentation render rarely needs RAW development. Capture One
develops are non-destructive and live only in the C1 catalog — a single point of failure — so
each edit is baked to a full-res file beside its master (`<stem>-<suffix>.<ext>`) and re-enters
the archive as master-tier. Export reads whatever base and edit files exist on disk; it never
drives Capture One and never allocates edit suffixes.

## Goals / Non-Goals

**Goals:**
- Produce one base HEIC per frame plus one per baked edit, into a persistent `Export/` mirror.
- Make republish incremental and idempotent via per-derivative version identity.
- Keep the base path free of RAW rendering and C1 automation.

**Non-Goals:**
- Importing into Apple Photos or any dedup against Photos (that is the `publish` verb / `add-publish`).
- Deleting or superseding prior renders.
- Generating full-res baked edits — those are produced by a manual Capture One recipe, not this tool.

## Decisions

- **Two identities, two jobs.** Frame id = the stem (shared by master, JPEG, edits, derivatives);
  version id = BLAKE3 of the *source* file that produced a given HEIC. The base and each edit have
  distinct version ids. Version id drives per-derivative idempotency and marks the file as ours;
  the stem only groups versions of a frame after the fact. Both are stamped into the HEIC XMP as
  `catalogKey`/`catalogStem` so they survive renaming.
- **Stem-keyed suffix parsing.** Stems contain hyphens, so suffix detection keys off the known
  canonical stem, not naive hyphen-splitting: within a frame group `<stem>.<raw>` is the master,
  `<stem>.JPG` the camera JPEG, `<stem>.HEIC` an iPhone-origin frame, and `<stem>-<suffix>.<img>`
  an edit. Suffixes are free-form labels from the C1 export recipe (`-edit`, `-bw`, `-crop`, …).
- **Base source: camera JPEG, else the RAW's embedded JPEG.** Because most frames are RAF+JPEG the
  base needs no RAW rendering; a RAW-only frame falls back to the full-resolution JPEG exiftool
  extracts from the RAW — every body in this library exposes it as `PreviewImage`, with `JpgFromRaw`
  as a fallback. Edits are additive and never suppress the base.
- **iPhone-origin frames yield no derivative.** A `<stem>.HEIC` with no camera JPEG is already a
  presentation-grade file living in the catalog (pulled in), so export leaves it alone.
- **HEIC, 4096 px long edge, quality ~70, configurable.** 4096 px (~11 MP) fills a 4K display with
  crop headroom and prints to ~13×9″, and HEIC yields a 10–20× size cut over a 40 MP master.
  `sips` does the transcode/resize; exiftool carries `DateTimeOriginal`/GPS/orientation so Apple
  Photos sorts chronologically; other metadata may be dropped.
- **`Export/` kept on disk, not ephemeral.** Persisting the tree keeps republish truly incremental
  (skip regeneration), makes it a faithful local mirror of the cloud catalog, and lets Google be
  bulk-seeded from `Export/` instead of round-tripping the phone. Cost is ~1–2 MB/frame; the tree
  is regenerable and excluded from backup. Full-res baked edits never live in `Export/` — only
  generated presentation HEICs do.
- **`derivative` table, split-written, no supersede columns.** One row per derivative keyed by
  unique `source_hash` (version id), plus `stem`, `source_kind` (edit|jpeg|embedded), `heic_path`,
  and `generated_at`. `export` owns these columns; the nullable `photos_uuid` and `published_at`
  are left for `publish` to fill on the Apple Photos push. A `source_hash` already present (already
  generated) is skipped, which is what makes export incremental. No replace/supersede columns —
  editing a frame adds rows, never rewrites them.

## Risks / Trade-offs

- `sips` and exiftool are per-file subprocess calls → batch where possible; the incremental skip
  keeps steady-state runs cheap since only new sources are transcoded.
- Free-form suffixes mean the tool cannot validate an edit label → intentional; it exports
  whatever edit files exist and encodes the label in the derivative name.
- Persisting `Export/` trades disk for republish speed and a Google bulk-seed path → accepted;
  cost is negligible against the masters and the tree is backup-excluded.

## Migration Plan

Additive. The `derivative` table is created with `CREATE TABLE IF NOT EXISTS`; existing databases
gain an empty table. `Export/` is created on first export. No change to import, the index's
existing tables, or on-disk masters.
