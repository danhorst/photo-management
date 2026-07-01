## Why

iPhone-captured photos live in Apple Photos and iCloud but not in the archive, so the archive is
not yet canonical for phone photos. Pull closes that gap: it exports iPhone-origin assets out of
Apple Photos and runs them through the existing import pipeline, so BLAKE3 dedup and `YYYY/MM`
organizing bring them into the archive on the same terms as camera files. It is the thinnest of
the four verbs — mostly a wrapper around `osxphotos export` feeding the import core.

## What Changes

- New `pm pull` command that exports iPhone-origin assets from Apple Photos into a queue
  directory via `osxphotos export`, then runs the existing import pipeline over that directory.
- Device allowlist filter, initially model `Apple iPhone 13 mini` (extensible later).
- Exclude any asset carrying our `catalogKey`, so the tool never re-ingests its own published
  derivatives (edited or not).
- Two independent dedup layers: `osxphotos --update` avoids re-exporting, and the BLAKE3 index
  avoids re-importing.
- Live Photos: the still is imported like any other frame; the motion `.mov` component is
  ignored.
- `--since <date>` to scope a run.

## Capabilities

### New Capabilities
- `pull`: reverse syndication — exporting iPhone-origin assets from Apple Photos and importing
  them into the archive through the existing import core, so the archive is canonical for phone
  photos too.

### Modified Capabilities
<!-- None: pull is new; it reuses import behavior without changing it. -->

## Impact

- New thin `internal/pull` (or similar): the `osxphotos export` wrapper, device allowlist, and
  `catalogKey` exclusion.
- `cmd/pm`: new `pull` command reusing the import path from `cmdImport` over the queue directory.
- New runtime dependency: `osxphotos` (shared with `add-publish`).
- Depends on `rename-to-photo-management` (binary `pm`) and reuses the import core captured in
  `formalize-import`. Independent of the export and publish changes.
