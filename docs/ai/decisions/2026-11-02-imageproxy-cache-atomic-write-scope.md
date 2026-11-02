# imageProxy cache atomic write scope

Date: 2026-11-02
Superseded by: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope-correction.md`

## Accepted facts

- A missed shard time window for an unacquired full image bytes object cannot later be recovered as full actual pixels.
- Full image pass and TN display state must not be counted as the same pass.
- Persistent full image cache is already implemented in `imageProxy`.
- `imageProxy` uses local full, persistent full cache, then remote full StorageURL, and persists remote full success to `cachePath`.
- `cacheRoot` remains product-root relative `cache/images` through `app/viewer.go:41`.
- TN display fallback in `#reader` or `#thumbs` remains forbidden. TN is search-card cover only.
- `image_files` / `gallery_files` table additions are not accepted.
- File magic cannot distinguish TN from full when both are webp. Full/TN discrimination uses `x-cache` and actual dimensions against `galleryinfo` width/height.

## Rejected proposals

- Do not redesign image storage on the premise that full bytes are unsaved.
- Do not move cache root to `scratch/` or `output/`.
- Do not add DB tables for image cache state.
- Do not use TN visual fallback as reader display.
- Do not treat SQLite `WAL` / `busy_timeout` as part of image cache persistence.
- Do not change full actual pixels pass criteria in this task.

## User-confirmed proposal scope

Only this improvement is recorded as the separate proposal scope:

- Atomicize `imageProxy` full image cache `WriteFile` only.
- Use temp write + close + rename in the same cache directory.
- Include same-hash / same-format concurrent write protection, keyed by `cachePath`.
- Keep existing response behavior. Remote full success remains `x-cache: miss`; cache hit remains `x-cache: cache`.
- Do not change `tn` / `tn-cache` handling, TN display policy, UI pass criteria, manual retry pass criteria, DB schema, cache root, or acquisition write behavior.

## Implementation guard

This decision records a proposal. It does not authorize implementation. A separate explicit implementation-start instruction is required before changing code.
