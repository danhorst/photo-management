## Why

The tool is no longer just an importer.
It is becoming bidirectional syndication around a canonical archive — `import`, `publish`,
`pull` — so the name `photo-import` and binary `photo-import` misdescribe it.
The repo directory is already `photo-management`, but the binary, Go module path, Homebrew
formula, GitHub repo, and every usage string still say `photo-import`.
Renaming first, before the new capabilities land, keeps each feature change's diff clean and
isolates the cross-cutting infra/tap/CI churn into one reviewable change.

## What Changes

- Binary renamed `photo-import` → `pm`; command directory `cmd/photo-import` → `cmd/pm`.
- Go module path `github.com/dbh/photo-import` → `github.com/dbh/photo-management`; all imports
  updated.
- Build/release infra: `Makefile` binary name, `scripts/release` compare URLs,
  `.github/workflows/release.yml` archive URL and formula handling.
- Homebrew: formula `photo-import.rb` → `pm.rb` (or `photo-management.rb`) in `danhorst/tap`;
  install becomes `brew install danhorst/tap/pm`.
- GitHub repo `danhorst/photo-import` → `danhorst/photo-management` (rename, keeping the
  redirect GitHub provides).
- All usage/help text, README, and CHANGELOG references to `photo-import` become `pm`.
- The on-disk names are renamed too: the config directory/file
  `~/.config/photo-import/photo-import.toml` → `~/.config/photo-management/photo-management.toml`,
  and the on-card volume marker `.photo-import.toml` → `.photo-management.toml`. A fallback-read
  shim reads the old name when the new one is absent and writes the new name going forward, so
  existing cards and configs are migrated rather than orphaned.

## Capabilities

### New Capabilities
<!-- None: the rename introduces no new behavior. -->

### Modified Capabilities
- `media-cache`: the `media list` / `media clear` command invocations are shown under the new
  binary name `pm` instead of `photo-import`.

## Impact

- `cmd/photo-import` → `cmd/pm`; `go.mod` module path and every internal import.
- `Makefile`, `scripts/release`, `.github/workflows/release.yml`, Homebrew tap formula.
- `README.md`, `CHANGELOG.md`, `AGENTS.md`/`CLAUDE.md` references.
- `internal/config` (config dir/file name + default-template comment) and `internal/volume`
  (`markerName`) gain the new names plus an old-name fallback read.
- No index/database schema change: existing indexes keep working; only the config and card-marker
  filenames move, and the fallback read makes that migration transparent.
- Sequencing: this change lands first; `add-export-derivatives`, `add-publish`, and `add-pull`
  assume the `pm` binary and `github.com/dbh/photo-management` module.
