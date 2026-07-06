## Context

The canonical stem (`YYYY-MM-DD--HH-MM-SS-<original>`, with day/month fallbacks) is the authoritative capture date for a frame.
Today that date is used only for filing and matching — it is never written into a derivative or an original's metadata.
Two downstream catalogs read a date the pipeline does not supply:

- Apple Photos, on `publish`, reads only the HEIC's embedded EXIF (`osxphotos import` is called with no date flag). A derivative built from a source lacking EXIF is undated, so Photos stamps the import day.
- Capture One reads the original's embedded EXIF (or an XMP sidecar). A scanned original with no EXIF shows undated.

The project invariant (`openspec/project.md`) is that the tool only moves and names files on disk — it never rewrites file contents. That rules out stamping EXIF into originals in place.

## Goals / Non-Goals

**Goals:**
- Guarantee every exported HEIC carries a real capture date, so Apple Photos never files a frame under its import day.
- Give archived originals that lack EXIF a Capture-One-readable capture date, without touching their bytes.
- Source both from the same authority (the canonical stem), so all three representations agree.

**Non-Goals:**
- Changing `import` (an EXIF-dated file already embeds its date; a no-EXIF file has no date until recanon supplies one).
- Changing `publish` (it keeps reading the HEIC's embedded EXIF).
- Sidecars for edit files inside an already-canonical frame — recanon skips canonical frames; those edits' derivatives are still dated via the export fallback.
- Overriding a wrong-but-present EXIF date (e.g. an unset camera clock). The stem is a fallback only, never an override.

## Decisions

**Sidecar, not in-place EXIF, for originals.**
Writing EXIF into an original would break the "only moves and names files" invariant, change the file's BLAKE3 hash (so a later re-import of the untouched card file would fail dedup and duplicate), churn the backed-up master tier, and is unsafe on Fuji RAF and other proprietary RAW.
An XMP sidecar avoids all of this: the original stays byte-identical, and Capture One reads the sidecar's date automatically (verified against a no-EXIF JPEG whose sidecar dated it 2005-06-15).
RAWs, which virtually always embed a date, are skipped anyway by the "only when absent" guard — so the RAW-write hazard never arises.

**Export fallback via a second, guarded exiftool call.**
`Generate` already copies date/GPS/orientation with `exiftool -tagsFromFile src`. Adding a second call `exiftool -wm cg -DateTimeOriginal=<stem> -CreateDate=<stem>` writes those tags only when they are absent (`-wm cg` = create, never update), so a real source date is never clobbered.
The stem date is parsed with `organize.ParseStem`, which already yields midnight for day precision and first-of-month midnight for month precision.
Alternative considered: fold the fallback into the existing `-tagsFromFile` call. Rejected — mixing write-modes in one command is subtle and error-prone; a separate guarded call is obviously correct.

**Recanon is the sidecar write point, not import.**
An original first gains a canonical date it does not embed exactly when recanon assigns a stem to a previously non-canonical frame. Import never faces this: it either reads a date from EXIF (already embedded) or sends the file to `Unsorted/` with no date at all.

## Risks / Trade-offs

- Stray `.xmp` files in the archive → `media.IsMedia` already excludes them from import, export, and indexing, so they are inert to the pipeline.
- Capture One only picks up the sidecar when its XMP sync is set to load (or on a manual metadata sync) → documented operational note; not enforced by the tool.
- Extra exiftool spawn per derivative → negligible against the existing per-file exiftool and sips calls, and it does real work only when a source lacks a date.
- Sidecar and stem could drift if one is edited by hand → both derive from the same stem and are only written by recanon at rename time, so they start identical; hand edits are out of scope.
