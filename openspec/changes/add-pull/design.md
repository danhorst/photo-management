## Context

The archive is authoritative for camera files but not yet for iPhone captures, which live in
Apple Photos / iCloud. Pull reuses the import core rather than building a second ingest path:
export the right assets to a queue directory, then let the existing pipeline hash, dedup, and
organize them.

## Goals / Non-Goals

**Goals:**
- Bring iPhone-origin photos into the archive canonically, reusing import unchanged.
- Never re-ingest our own published derivatives.

**Non-Goals:**
- Special handling for Live Photos motion components.
- Any new organize/dedup logic — import's BLAKE3 + `YYYY/MM` path is reused as-is.
- Deleting anything from Apple Photos.

## Decisions

- **Thin wrapper over the import core.** Pull is `osxphotos export` into a queue directory
  followed by the existing import pipeline; BLAKE3 dedup and `YYYY/MM` organizing apply unchanged.
  This is why pull is the thinnest capability.
- **Device model allowlist, initially `Apple iPhone 13 mini`.** An allowlist (extensible later)
  scopes the export to genuinely phone-origin frames rather than everything in Photos.
- **Exclude our own `catalogKey`.** Assets carrying the `catalogKey` we stamp when exporting are
  our own derivatives; excluding them stops the tool from re-ingesting what it published, edited or
  not.
- **Two independent dedup layers.** `osxphotos --update` avoids re-exporting already-exported
  assets; the BLAKE3 index avoids re-importing. Either alone would suffice; together they keep
  repeated pulls cheap and safe.
- **Live Photos treated as ordinary stills.** The still is imported like any frame and the motion
  `.mov` is ignored (only ~36 in the library) — not worth special handling.

## Risks / Trade-offs

- Allowlist starts narrow (one model) → intentional; extending it is a config/list change, and a
  too-broad filter would pull non-phone assets.
- Relies on `osxphotos export` fidelity for `DateTimeOriginal`/original filename so the import
  path names frames correctly → the same metadata publish's natural key depends on; acceptable.
- Ignoring Live Photos motion loses the video component → accepted given the tiny count and the
  still being the archival unit.

## Migration Plan

Additive and behavior-preserving for import. No schema change — pull writes through the existing
import path and its `media_files`/index tables. The queue directory is a scratch export location,
not persistent state.
