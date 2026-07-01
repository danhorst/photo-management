## Context

Apple Photos is a downstream, regenerable presentation catalog, but importing into it naively
would duplicate: on a re-run (our own prior pushes) and against frames already present — manually
added, or iPhone-origin frames that `pull` put into the archive and that already live in Photos.
Publish therefore needs two independent dedup layers. It also must accept that Apple Photos has
no unattended programmatic delete.

## Goals / Non-Goals

**Goals:**
- Import only genuinely new derivatives into Apple Photos, idempotently.
- Recognize both our own prior pushes and pre-existing overlap.

**Non-Goals:**
- Deleting, replacing, or superseding any asset.
- Album creation or organization inside Photos.
- Handling Google Photos directly — Google syncs from Apple Photos (or is bulk-seeded from
  `Export/`) independently.

## Decisions

- **Two independent dedup layers.** Layer 1 recognizes *our* pushes: a `derivative` row whose
  `photos_uuid` is already set has been pushed, so it is skipped (its version id is also stamped as
  `catalogKey` in the asset). Because `export` writes the `derivative` row before publish runs, the
  "already pushed" signal is the `photos_uuid`, not the mere existence of the row.
  Layer 2 recognizes *pre-existing* overlap: build an `osxphotos` manifest of current assets and
  match each frame on a natural key (`DateTimeOriginal` + original filename, both already encoded
  in the archive filename); a match skips the import and records the association. Layer 2 also
  covers pulled iPhone-origin frames, which are already in Photos and carry no camera JPEG.
- **`photos_manifest` as a cache.** `uuid`, `original_filename`, `capture_time`, `catalog_key`,
  `last_synced` — a rebuildable cache of the Photos manifest for overlap detection. Like the rest
  of the index it is authoritative of nothing; the archive and Photos are the real state.
- **Record `photos_uuid` and `published_at` on the derivative.** A successful import fills the
  nullable `photos_uuid` and `published_at` columns (which `export` left null) on the `derivative`
  row, tying our render to its Photos asset and marking it pushed for later grouping and audit. No
  supersede/replace columns are added.
- **No-supersede, grounded in three facts.** (1) Apple Photos has no unattended programmatic
  delete — `osxphotos` cannot delete, PhotoKit forces a user-confirmation prompt, and AppleScript
  never had a delete verb for media. (2) Deletion would not propagate: once a render syncs to
  Google, Google keeps an independent copy. (3) Deleting against the live library risks removing a
  real photo on any UUID drift from an iCloud re-sync or rebuild. So an edited frame appears as
  multiple assets (base + each edit); this is rare (developing is reserved for special cases) and
  the assets stay reconstructable by grouping on the shared stem. No supersede state is tracked.
- **Flat import.** No album creation — a plain import, matching the seed's locked decision.

## Risks / Trade-offs

- The `photos_manifest` cache can go stale between builds → rebuild it at the start of a publish
  run; staleness only risks a redundant skip/association, never a wrong delete (there are none).
- Natural-key matching (`DateTimeOriginal` + original filename) could theoretically collide →
  acceptable; both fields come straight from the archive filename and the cost of a false match is
  a skipped import, not data loss.
- Multiple assets per developed frame → accepted cost of no-supersede; reconstructable via the
  shared stem in the filename and stamped metadata.

## Migration Plan

Additive. `photos_manifest` is created with `CREATE TABLE IF NOT EXISTS`; `derivative.photos_uuid`
and `published_at` are already present (nullable) from `add-export-derivatives`. No change to
export, import, or on-disk state. Removing the feature leaves unused rows.
