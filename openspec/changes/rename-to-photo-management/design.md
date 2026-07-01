## Context

The repo directory was already renamed to `photo-management`, but the binary (`photo-import`),
Go module (`github.com/dbh/photo-import`), Homebrew formula, GitHub repo, and all help strings
still carry the old name. On-disk artifacts owned by the tool — the `.photo-import.toml` card
marker and the `~/.config/photo-import/` config — are read by live cards and the user's machine,
so renaming them needs an old-name fallback so existing state migrates instead of orphaning.

## Goals / Non-Goals

**Goals:**
- One clean, mechanical rename to binary `pm` and module `github.com/dbh/photo-management`.
- Keep release/tap distribution working under the new name.

**Non-Goals:**
- Changing the config or index file *format* — only the filenames move.
- Any behavior change; this is a refactor plus a transparent filename migration.

## Decisions

- **Binary `pm`, module `github.com/dbh/photo-management`.** `pm` matches the reframed CLI
  surface (`pm import` / `pm publish` / `pm pull`). The module path follows the repo name.
- **Rename the config and marker, with an old-name fallback read.** The config dir/file becomes
  `~/.config/photo-management/photo-management.toml` and the card marker becomes
  `.photo-management.toml`, so no `photo-import` name survives on disk. To avoid orphaning live
  cards (a card whose marker is unread mints a fresh volume id and re-processes its files) and
  the user's existing config, both readers try the new name and fall back to the old one when it
  is absent, and always **write** the new name. Config additionally migrates on write: the first
  save under the new path supersedes the old file. `photo-management` (not `pm`) is used for the
  on-disk names so they stay unambiguous even though the binary is `pm`.
- **No `photo-import` shim binary.** Distribution is a personal Homebrew tap with a single user;
  a compatibility alias is unnecessary. `brew install danhorst/tap/pm` replaces the old formula.
- **GitHub repo rename over redirect.** Renaming `danhorst/photo-import` →
  `danhorst/photo-management` leaves GitHub's automatic redirect in place, so old release URLs
  and tap history resolve; the release workflow is updated to emit the new URLs going forward.

## Risks / Trade-offs

- Tap formula rename is a breaking install-path change → acceptable for a single-user tap;
  `brew uninstall photo-import && brew install danhorst/tap/pm` once.
- Fallback-read adds a small permanent branch in the config and marker readers → kept minimal
  (try new path, then old); it can be dropped in a later change once no old-named files remain.
- Names are `photo-management` while the binary is `pm` → intentional; the binary is terse for
  the CLI, the on-disk names are explicit for grep-ability and to avoid a generic `pm` collision.

## Migration Plan

Rename in one change: code + module + imports, then infra + formula + docs, then the on-disk
config and marker names. The GitHub repo rename is a one-time repo-settings action with an
automatic redirect. The filename migration is transparent: the renamed binary reads an old-named
config or card marker if present, and writes the new name from then on, so existing libraries,
indexes, stamped cards, and configs keep working with no user action. The old-name fallback stays
until a later change retires it.
