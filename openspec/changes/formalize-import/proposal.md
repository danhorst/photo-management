## Why

Import is the tool's original, shipping capability, but it has no formal spec — its behavior
lives only in `project.md` prose. The reframe makes import one of four symmetrical operations
(`import` / `export` / `publish` / `pull`); the other three are getting `specs/` capabilities, so
import should sit beside them on equal footing. Capturing the current behavior as a spec now
gives the outbound verbs and `pull` a documented core to build on and a regression baseline, with
no code change.

## What Changes

- A new `import` capability spec records the already-shipping behavior: content-hash dedup,
  capture-date organization, managed-extension filtering, move-vs-copy by volume, and
  `--dry-run` semantics.
- No code changes. Each requirement is an audit of existing behavior in
  `internal/{hash,exif,organize,index,volume}` and `cmdImport`; any mismatch found during the
  audit is a bug filed separately, not silently "specced away."

## Capabilities

### New Capabilities
- `import`: pulling media off camera cards / queue directories into the archive by content-hash
  dedup and capture-date organization, moving on the same volume and copying across volumes.

### Modified Capabilities
<!-- None: this is the first formal capture of import behavior. -->

## Impact

- Docs/specs only: adds `specs/import/`. No `cmd/` or `internal/` change.
- The per-card skip cache is already specified in `specs/media-cache/`; this spec references it
  rather than restating it.
- Establishes the import core that `add-pull` reuses and that `add-export-derivatives` reads
  from.
