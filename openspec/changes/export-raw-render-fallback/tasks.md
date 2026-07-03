## 1. Fallback render

- [x] 1.1 In `internal/export/generate.go`, add a sentinel `errNoEmbeddedJPEG` and return it from `extractEmbedded` for the both-tags-empty case (preserving the `no embedded JPEG in <path>` message)
- [x] 1.2 In `Generate`, when `extractEmbedded` returns `errNoEmbeddedJPEG`, fall back to `input = src.Path` (direct `sips` render of the master) instead of returning the error; propagate any other extraction error unchanged

## 2. Tests

- [x] 2.1 Unit-test that `extractEmbedded` returns `errNoEmbeddedJPEG` (via `errors.Is`) for a master with no `PreviewImage`/`JpgFromRaw`
- [x] 2.2 Test that `Generate` produces a HEIC for a RAW-only frame whose master has no embedded JPEG, and still prefers the embedded JPEG when one is present

## 3. Verify

- [x] 3.1 Run `go test ./internal/export/...`
- [x] 3.2 Against a sandbox library (`-L`/`--db`), export one previously-failing DNG and one misnamed `.CR2`, and eyeball the resulting HEIC for reasonable tone before any bulk run
