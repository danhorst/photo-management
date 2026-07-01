## 1. Code and module

- [x] 1.1 `git mv cmd/photo-import cmd/pm` and rename the package doc comment/binary references
- [x] 1.2 Change the module path in `go.mod` to `github.com/dbh/photo-management`
- [x] 1.3 Update every internal import path (`github.com/dbh/photo-import/...` → `.../photo-management/...`)
- [x] 1.4 Replace `photo-import` in usage/help strings with `pm` (dispatch, per-command `Usage`)
- [x] 1.5 `go build ./...` and `go test ./...` pass under the new module path

## 2. On-disk config and marker names

- [x] 2.1 `internal/config/config.go`: default path → `filepath.Join(base, "photo-management", "photo-management.toml")`; on read, fall back to the old `photo-import/photo-import.toml` when the new path is absent; update the `defaultTemplate` comment; fix `config_test.go`
- [x] 2.2 First save under the new path migrates: writing config creates the new file and supersedes the old one
- [x] 2.3 `internal/volume/volume.go`: `markerName` → `.photo-management.toml`; on read, fall back to `.photo-import.toml` when the new marker is absent; always stamp the new name
- [x] 2.4 Unit-test the fallback read for both config and marker (old name present → read; new name written on save/stamp)

## 3. Build and release infra

- [x] 3.1 `Makefile`: output binary `bin/pm`, build/clean targets, `./cmd/pm`
- [x] 3.2 `scripts/release`: update the `github.com/danhorst/photo-import/...` compare URLs
- [x] 3.3 `.github/workflows/release.yml`: archive URL, `formula_path`, formula url regex, commit message, `git -C tap add`
- [x] 3.4 Homebrew tap: add `Formula/pm.rb`; retire `photo-import.rb`

## 4. Repository

- [x] 4.1 Rename the GitHub repo `danhorst/photo-import` → `danhorst/photo-management` (settings; redirect stays)
- [x] 4.2 Confirm `git remote -v` / release URLs resolve after the redirect

## 5. Docs

- [x] 5.1 `README.md`: title, install command (`danhorst/tap/pm`), every `photo-import <cmd>` → `pm <cmd>`; note the new config path and card marker
- [x] 5.2 `CHANGELOG.md`: add a rename entry per release conventions, calling out the config/marker migration
- [x] 5.3 `AGENTS.md` (and the `CLAUDE.md` symlink) header references

## 6. Verify

- [x] 6.1 `go test ./...` passes; `make` produces `bin/pm`
- [x] 6.2 `openspec validate rename-to-photo-management --strict` passes
- [x] 6.3 Manual: `pm version`, `pm media list` against a throwaway `--db` (never the live library); confirm an old-named config/marker is read and a new-named one is written
