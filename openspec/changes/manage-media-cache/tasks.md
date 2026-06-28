## 1. Index: volume registry

- [ ] 1.1 Add `volumes(volume_id PRIMARY KEY, label, last_seen)` to `schema` in `internal/index/index.go`
- [ ] 1.2 Add `PutVolume(volumeID, label string) error` upserting `label` and `last_seen=now`
- [ ] 1.3 Add `VolumeInfo` struct and `Volumes() ([]VolumeInfo, error)` selecting distinct `media_files` ids LEFT JOIN `volumes`, with name, last-seen, and file count, ordered by last-seen desc
- [ ] 1.4 Add `ClearVolume(volumeID string) (int64, error)` deleting from `media_files` (return rows affected) and `volumes`
- [ ] 1.5 Unit-test `PutVolume`/`Volumes`/`ClearVolume` round-trip and that re-`PutVolume` updates last-seen without duplicating

## 2. Import: register the volume

- [ ] 2.1 In `cmdImport` (`cmd/photo-import/main.go`), after `volume.Stamp`, call `idx.PutVolume(volID, filepath.Base(root))` when not `--dry-run`

## 3. CLI: media subcommand

- [ ] 3.1 Add `case "media": err = cmdMedia(args[1:])` to the dispatch in `main` and extend the `usage` text with `media list` / `media clear`
- [ ] 3.2 Implement `cmdMedia` sub-dispatching `list` and `clear`, both taking shared `--db`/`-L` flags
- [ ] 3.3 Implement `media list`: open index, call `Volumes()`, print a table (name, short id, file count, last seen)
- [ ] 3.4 Add a plain (testable) resolver mapping a selector to a single volume id by exact match or unambiguous prefix; error on ambiguous prefix or label-only input
- [ ] 3.5 Implement `media clear` with id args: resolve each selector, `ClearVolume`, report rows cleared
- [ ] 3.6 Implement `media clear` with no args: on a TTY (`isatty`) show an interactive multiselect of `Volumes()` and clear selected; otherwise error asking for an id
- [ ] 3.7 Unit-test the selector resolver (exact, unambiguous prefix, ambiguous→error, label-only→error)

## 4. Dependency

- [ ] 4.1 Add the multiselect library (`github.com/charmbracelet/huh`) via `go get` and tidy modules; keep its use confined to `media clear`

## 5. Docs

- [ ] 5.1 Document `media list` / `media clear` and the reformat/orphaned-cache note in `README.md` (one-sentence-per-line)
- [ ] 5.2 Add a `CHANGELOG.md` entry following the existing release conventions

## 6. Verify

- [ ] 6.1 `go test ./...` passes
- [ ] 6.2 Manual: build, import a temp tree twice into a throwaway `--db` (never the live library), `media list`, `media clear <prefix>`, confirm re-import re-hashes
