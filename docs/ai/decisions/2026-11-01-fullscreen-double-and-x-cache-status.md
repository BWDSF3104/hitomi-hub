# 2026-11-01 fullscreen double and x-cache TN status separation

**Superseded by**: `2026-11-01-tn-endpoint-display-replacement-nearest-order.md`

## Decision
- Fullscreen can also be double when the viewer is not in single mode and `window.innerWidth >= 720`.
- Reader double items should be determined by `doubleActive()`, not only by `displayMode()`, so fullscreen double remains active.
- `prefetch-status` must count full, TN fallback, and errors separately. TN fallback bytes must not be counted as full prefetch.
- `loadFullImage()` should keep placeholder thumbnail when the image source's `x-cache` is `tn`.

## Reason
- The server can return TN bytes for `/api/image/` when full image bytes are unavailable. Previously, `prefetch-status` counted that as full while the visual result was still a thumbnail.
- Fullscreen should reproduce hitomi-like double pages when there is enough width.
