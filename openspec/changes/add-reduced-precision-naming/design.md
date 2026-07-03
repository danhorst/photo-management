## Context

Canonical stems are minted in `internal/organize/organize.go` (`tsLayout = "2006-01-02--15-04-05-"`) and parsed by two independent, differently-strict copies: `export/frame.go`'s `Frame.CaptureDate` (checks only the first ten chars) and `photos/photos.go`'s `ParseStem` (requires the full second layout).
That split is why a bare day stem exports but fails publish, and why a batch with no date prefix is skipped by export entirely.
The tool only ever moves and names files on disk; it never writes the Capture One catalog.

## Goals / Non-Goals

**Goals:**
- One grammar, one parser, three precisions (second, day, month), owned by `organize`.
- `export`, `publish`, and `pull` accept reduced-precision frames by delegating to that parser.
- A `pm recanon` command that renames non-canonical stragglers to reduced-precision names, safely and idempotently.

**Non-Goals:**
- The `Unsorted/` metadata backlog (code-path B) — a separate change.
- Extending `pm import`'s undated fallback to mint reduced-precision names — a listed follow-up, not this change.
- Year precision or any precision coarser than month.

## Decisions

**Naming format: drop unknown fields, not a sentinel time.**
Day is `YYYY-MM-DD-<original>`, month is `YYYY-MM-<original>`.
Chosen over a sentinel (`2021-12-01--00-00-00-<original>`) because a fabricated timestamp lies about precision and collides with real first-of-month/midnight captures, and because 7 files already use the bare-day form.
The cost is parser work and a narrow ambiguity (below) that the sentinel would have avoided.

**One parser in `organize`, precedence second → day → month.**
`ParseStem(stem) (t time.Time, prec Precision, original string, ok bool)`:
- second: `len >= 21`, `stem[10:12] == "--"`, parse `2006-01-02--15-04-05`.
- day: `stem[:10]` parses `2006-01-02`, `stem[10] == '-'`, `stem[11] != '-'`.
- month: `stem[:7]` parses `2006-01`, `stem[7] == '-'` (only reached after day fails).
Reduced forms return `t` at midnight / first-of-month.
`DestAt(library, t, prec, origName)` mints per precision; `Dest` stays as the second-precision case so existing callers are untouched.
`organize` is the right owner because it already mints names; making it also parse removes the duplicate grammars.

**Delegation, not reimplementation.**
`Frame.CaptureDate` and `photos.ParseStem` call `organize.ParseStem`.
`photos`'s `naturalKey` keeps formatting a full second timestamp, so reduced-precision overlap matching is best-effort — acceptable because these frames rarely pre-exist in Apple Photos.
If `photos` importing `organize` introduces a cycle, move the parser to a tiny leaf package instead; `organize` is the default.

**`pm recanon` reuses the export walker.**
It walks `YYYY/` via `collectArchive` and groups with `export.Group`, selects frames where `organize.ParseStem` fails, derives month precision from the folder, and renames in place with `organize.Place` (same-volume move).
Flags: global `--dry-run`; `--match <substr>` to scope a batch; `--date YYYY-MM-DD` to stamp day precision instead of month-from-folder.

**Index consistency on rename.**
A rename changes a file's path but not its BLAKE3 hash, so the index path row must follow.
Preferred: update the path row directly, reusing import's index-write path in `internal/index`.
Fallback if that is heavy for a first cut: print a "run `pm index`" reminder after applying, matching how the project already requires reindex after out-of-band changes.

## Risks / Trade-offs

- Format ambiguity: a month-precision original beginning `NN-` (e.g. `05-x.jpg`) parses as a day, and an original beginning `HH-MM-SS-` parses as second. → Accept it; real camera/scan names (`IMG_`, `DSCF`, `DSC`, the ZF batch) never start that way, and precedence tests pin the behavior.
- Weak overlap matching for reduced-precision frames (unknown exact time). → Best-effort by design; these frames are unlikely to already be in Apple Photos, and publish's second layer (version-id) still prevents our own re-pushes.
- Renaming the live archive is destructive. → `--dry-run` is the default rehearsal; `--match` scopes a batch; verification runs against a sandbox library via `-L`/`--db`, never `/Volumes/Photos`.
- Index drift if the path row is not updated. → Covered by updating the row, or by the explicit `pm index` reminder.
