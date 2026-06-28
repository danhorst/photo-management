## Why

The per-card skip cache (`media_files`) is invisible and unmanageable: there is
no way to see what cards it holds or to remove stale entries. When a card is
reformatted its on-card marker is wiped, so the next import mints a fresh
`volume_id` and the reformatted card's old rows are orphaned — read by nothing,
deleted by nothing. The leak is small but unbounded across reformats, and the
user has no tool to inspect or reclaim it.

## What Changes

- The importer records each source volume's human-readable name (its mount-point
  label) and last-seen time, so the cache can be presented by name rather than
  by opaque id.
- New `photo-import media list` prints every cached volume — name, short id,
  file count, last seen — including caches created before this change.
- New `photo-import media clear` removes a cached volume's skip entries:
  non-interactively by id (exact or unambiguous prefix), or via an interactive
  multiselect when run on a terminal. Clearing touches only the index, never the
  card or its marker; the next import re-hashes that card from scratch.
- Documentation explains the reformat behavior and points at `media clear` as
  the way to drop an orphaned cache entry.

## Capabilities

### New Capabilities
- `media-cache`: inspecting and clearing the per-volume skip cache that lets
  re-imports skip already-processed files, including how source volumes are
  named and tracked.

### Modified Capabilities
<!-- None: the existing skip behavior is not yet specced; this introduces the first media-cache spec. -->

## Impact

- `internal/index`: new `volumes` table and `PutVolume` / `Volumes` /
  `ClearVolume` methods; `media_files` unchanged.
- `cmd/photo-import`: register the volume on import; new `media` subcommand
  (`list`, `clear`) and usage text.
- New dependency: `github.com/charmbracelet/huh` for the interactive multiselect.
- Docs: README and CHANGELOG.
- No schema migration needed — `volumes` is additive; existing databases gain an
  empty table and pre-existing caches still list and clear.
