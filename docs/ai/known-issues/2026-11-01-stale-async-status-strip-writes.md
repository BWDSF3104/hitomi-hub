# loading-time switch で stale async completion が status / strip を汚染する

**Date**: 2026-11-01
**Status**: 未解決

## 症状

- Page movement during pending load should affect only the latest target.
- Actual stale tests showed stale async completions could update status or strip images even when the latest route / reader remained correct.

## 実測

### Cache-less stale test

- Before stale operation: route `#/g/4252697/page/1-2`, status `full display cache 146/203 | no full 0/203 | TN display fallback disabled`.
- A stale `loadFullImage()` for page `1` was started with `oldShow=3`, `oldRender=3`.
- Latest target moved to page `4`.
- Latest route became `#/g/4252697/page/4-5`.
- After stale async completion, status changed to `full display cache 146/203 | no full 4/203 | TN display fallback disabled`.
- The stale completion updated status counts.

### Cache-backed full success stale test

- Latest reader remained correct.
- A stale completion for page `6` updated `#thumbs img[data-key='...page6...']` to an objectURL with `naturalWidth > 0`.
- `storeFullCache()` calls `updateThumbCache()` without checking whether the completion belongs to the latest render/page/hash generation.

## Code location

- `ui.html`: `updateThumbCache()` is called by `storeFullCache()` (`ui.html:429`).
- `ui.html`: `loadFullImage()` guard is present for DOM writes (`ui.html:465`, `ui.html:469`, `ui.html:478`, `ui.html:505`, `ui.html:511`), but status updates can occur before or outside the latest-target guard (`ui.html:444`, `ui.html:450`, `ui.html:482`, `ui.html:490`, `ui.html:497`, `ui.html:510`, `ui.html:519`).
- Background prefetch calls `updatePrefetchCounts()` without a full generation guard (`ui.html:616`, `ui.html:619`).

## Expected behavior

Status, strip images, priority, title, route, and counts should be updated only for the latest target, or should use per-gallery counts in a way that does not pollute the current display intent. A stale async completion must not update status text or strip DOM.

## Resolution scope

No implementation change was made in this record pass. Implementation approval is required before fixing stale status/strip writes.
