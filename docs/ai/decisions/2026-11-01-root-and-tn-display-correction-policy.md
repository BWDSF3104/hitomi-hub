# Root and TN display correction policy

Date: 2026-11-01

## Policy

- Root resolution must be copy-and-use. Default root is cwd; explicit `--root` overrides it. The `nearest .git parent` ancestor search is removed as unnecessary for this project.
- TN images are used only for search-card/result covers. TN must not be used as `#reader` or `#thumbs` viewer display source.
- TN viewer display exception is not accepted. TN display fallback remains forbidden.
- `tn` / `tn-cache` can be persisted for search-card cover use, but they must not become actual viewer display sources.
- Viewer actual image display pass requires actual full image pixels from local downloaded full file, persistent full cache, or remote full fetch.
- `no_full` is a source-side missing-full state, not an image-display pass. If no full image bytes are available, viewer display cannot satisfy the required actual pixels under this policy.

## Correction plan

- Change `resolveRoot()` to explicit `--root` > cwd.
- Add a Go test proving `resolveRoot()` returns cwd even when a `.git` directory exists in a parent path.
- Keep `imageProxy()` full-only order: local full, persistent full cache, remote full storage, `no_full`.
- Keep UI `cachedFullSrc()` returning full objectURL only. TN objectURL is not a viewer display source.
- Rebuild canonical EXE, restart canonical server with copied/runtime root, then verify actual screen DOM `naturalWidth > 0` for actual full image display.
