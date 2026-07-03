## Why

Export of a RAW-only frame relies on extracting the master's embedded full-resolution JPEG (`PreviewImage`, else `JpgFromRaw`).
Some masters carry no embedded JPEG at all — Linear Raw DNGs written by external tools (Photomatix HDR merges, Lightroom/Fuji panoramas, Topaz denoise/upscale) and a handful of Picasa-edited JPEGs misnamed with a `.CR2` extension — so their frames fail export with `no embedded JPEG` and never reach `Export/`.

## What Changes

- Add a base-source fallback for RAW-only frames: when embedded-JPEG extraction yields no bytes, transcode the master directly with `sips` (the macOS RAW pipeline renders the DNGs; the misnamed CR2s are already JPEG bytes `sips` reads as-is).
- The embedded-JPEG path stays preferred; the direct render is fallback-only, so RAW-only frames that do embed a preview are unaffected.
- No new source kind is surfaced to the caller: the fallback happens inside derivative generation for the existing `embedded` source, keeping version-id hashing (still the master's content hash) and the `source_kind` record unchanged.

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `export`: the RAW-only base-source requirement gains a fallback — direct `sips` render of the master when it carries no embedded full-resolution JPEG.

## Impact

- `internal/export/generate.go` — `Generate` gains the fallback when `extractEmbedded` reports no embedded JPEG.
- `openspec/changes/add-export-derivatives/specs/export/spec.md` — the current (unarchived) source of truth for the export capability; the delta modifies its RAW-only base-source requirement and scenario.
- No new runtime dependency: `sips` already transcodes every derivative.
- Does not address the underlying data-hygiene issue that the `.CR2`-named files are actually JPEGs; it only makes them exportable.
