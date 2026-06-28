## Context

`media_files` keys the skip cache on `(volume_id, path)` storing `size`+`mtime`,
where `volume_id` is a random id stamped into a `.photo-import.toml` marker at
the card's volume root. The id is opaque and the marker's `Label` field is never
populated, so the cache cannot be presented to a human. Reformatting a card wipes
the marker, orphaning that id's rows with no way to find or remove them. The
index is single-writer SQLite (`internal/index`); the CLI is a non-interactive,
dependency-light Go tool (`cmd/photo-import`).

## Goals / Non-Goals

**Goals:**
- Give every cached volume a human-readable name and a last-seen time.
- Let the user list cached volumes and clear selected ones.
- Keep `media_files` lean and require no schema migration for existing databases.

**Non-Goals:**
- Automatic garbage collection of orphaned caches (a future change may add it).
- Touching the card or its marker when clearing.
- Changing the skip/dedup behavior itself.

## Decisions

- **New `volumes` table, not a column on `media_files`.** A
  `volumes(volume_id PRIMARY KEY, label, last_seen)` table keeps `media_files`
  trimmed to the size+mtime the skip path needs, and gives one row per card
  rather than repeating the label on every file row. Alternative considered:
  denormalize `label`/`last_seen` onto `media_files` — rejected as it bloats the
  hot skip table and muddies the deliberate trim. Every `volumes` column is read
  by `media list`, so nothing is write-only.
- **Name = volume mount-point basename** (`filepath.Base(root)`, e.g.
  `EOS_DIGITAL`), captured at import via `PutVolume`. It is the most meaningful
  name available without new user input. Labels collide across cards, so they
  are display-only; the `volume_id` remains the unique key.
- **Listing drives off `media_files`.** `Volumes()` selects distinct
  `volume_id` from `media_files` LEFT JOIN `volumes`, so caches created before
  this change (no `volumes` row) still appear and stay clearable; their label
  renders blank until the next import re-registers them.
- **Two selection paths for `clear`.** Non-interactive: a `volume_id` or
  unambiguous prefix resolved against `Volumes()` (ambiguous prefix or
  label-only input is an error). Interactive: when invoked with no selector on a
  TTY, an interactive multiselect. The resolver and clear logic stay as plain
  functions; the TUI is a thin shim so the logic is testable without a terminal.
- **One new dependency** for the multiselect: `github.com/charmbracelet/huh`,
  used via a single `huh.NewMultiSelect[string]` form. Chosen over the lighter
  `AlecAivazis/survey/v2` because huh is actively maintained and `media clear` is
  the only consumer, so its heavier tree (bubbletea/bubbles/lipgloss) stays off
  the import/index hot paths. The tool has no interactive dependency today, so
  this is a deliberate addition scoped to `media clear`.

## Risks / Trade-offs

- New runtime dependency for a small feature → keep it isolated to the `clear`
  command so the import/index hot paths stay dependency-free.
- A non-reformat workflow that leaves the marker intact but is treated as a new
  card elsewhere could still skip falsely → out of scope here; documented as the
  marker being the source of identity.
- `Volumes()` aggregates `media_files` on demand; for very large caches the
  GROUP BY cost is paid only when the user runs `media list`/`clear`, never on
  import.

## Migration Plan

Additive only. `volumes` is created via `CREATE TABLE IF NOT EXISTS`, so existing
databases gain an empty table with no migration step. Pre-existing caches list
(with a blank name) and clear; they regain a name on the next import. No
rollback concern — removing the feature leaves an unused table.
