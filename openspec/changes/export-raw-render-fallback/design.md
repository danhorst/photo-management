## Context

`Generator.Generate` in `internal/export/generate.go` handles the `embedded` source kind by
calling `extractEmbedded`, which tries `PreviewImage` then `JpgFromRaw` via exiftool and returns
`no embedded JPEG in <path>` when both are empty. `sips` then transcodes the extracted JPEG to HEIC.

Two file classes in the live archive have no embedded JPEG and so fail export outright:

- Linear Raw DNGs from external tools (Photomatix HDR, Lightroom/Fuji panoramas, Topaz upscales) —
  real RAW-ish data, no camera preview.
- Picasa-edited JPEGs saved with a `.CR2` extension — the whole file is already JPEG bytes.

Both transcode cleanly when handed straight to `sips`: the macOS RAW pipeline renders the DNGs, and
the misnamed CR2s are ordinary JPEGs. The embedded-JPEG path is kept as the preferred route because
it is faster (no demosaic) and carries the camera's baked color rather than a generic render.

## Goals / Non-Goals

**Goals:**

- RAW-only frames whose master has no embedded JPEG export successfully instead of erroring.
- The embedded-JPEG path stays the default; direct render is fallback-only.
- No change to version-id hashing or `source_kind` recording — the fallback is invisible to the
  incremental-regeneration contract.

**Non-Goals:**

- No fix for the underlying data-hygiene problem that the `.CR2`-named files are JPEGs.
- No new source kind, config flag, or CLI surface.
- No attempt to match camera color science on the directly rendered DNGs.

## Decisions

- **Fallback lives in `Generate`, keyed on `extractEmbedded` finding no bytes.** `extractEmbedded`
  distinguishes "no embedded JPEG" (both tags empty) from a genuine exiftool failure. On the former,
  `Generate` sets `input = src.Path` and lets `sips` transcode the master directly; on the latter it
  still errors. Alternative — branch in `Sources()` by inspecting each master for a preview — was
  rejected: it needs an extra exiftool probe per RAW-only frame at planning time and duplicates the
  extraction that `Generate` already does.
- **Signal the empty case with a sentinel, not a magic string.** `extractEmbedded` returns a typed
  sentinel (`errNoEmbeddedJPEG`) for the both-empty case so `Generate` can branch with `errors.Is`
  rather than string-matching the message. The `no embedded JPEG` text is preserved as the sentinel's
  message so any surfaced error reads unchanged.
- **Identity unchanged.** `versionID` is already computed by the caller from the master's content
  hash and passed in; the fallback reuses the same `src` so nothing about identity or the recorded
  `embedded` kind moves.

## Risks / Trade-offs

- **Generic render color on Linear Raw DNGs** → `sips` applies no camera tone curve, so a directly
  rendered DNG can look flatter or differently toned than a camera JPEG would. Mitigation: eyeball a
  sample HEIC before a bulk run; the path only triggers when there is no embedded JPEG to prefer, so
  it never degrades a frame that had one.
- **Masks the misnamed-CR2 data bug** → making them exportable removes the pressure to rename them to
  `.jpg`. Mitigation: called out explicitly in the proposal Impact as out of scope, not silently
  absorbed.
- **A corrupt master that `sips` also cannot read** → now fails at the `sips` step with its stderr
  instead of at extraction, which is still a clear per-file error and does not abort the run.
