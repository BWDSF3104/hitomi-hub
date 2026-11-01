# Fast non-blocking viewer policy

Date: 2026-11-01

## Required functionality

- Viewer display must show actual full image pixels when a full image exists.
- TN display fallback is absolutely forbidden for `#reader` and `#thumbs`.
- Viewer actual pixels are allowed only from full image objectURL or local file.
- If full image is unavailable, UI must show explicit `no full`, not TN pixels.
- Search-card TN cover may remain as a separate low-priority search-card path.
- Page movement must remain lightweight and fast. It must not block on image download, read update, or lists update.
- Stale async writes must be ignored by generation guards and per-gallery status counts.

## Fast processing rules

1. `showPage()` must do only lightweight display work:
   - update current page and route,
   - render reader/strip from existing full cache,
   - start single current target priority if the target image is not cached,
   - update existing background queue only when target changes.
2. `showPage()` must not await `post("/api/read")`.
3. `showPage()` must not await `showLists()`.
4. `post("/api/read")` and `showLists()` must be background/debounced and guarded by generation checks.
5. Background prefetch should keep a stable queue per gallery and should not recompute all items for every page movement.
6. The only blocking waits allowed are intentional priority waits:
   - current displayed full image priority,
   - search-card TN after current priority,
   - no TN viewer display fallback.

## Verification procedure

1. Use actual running server `http://127.0.0.1:8731`.
2. Perform actual search, open gallery, page movement, page input, gallery switch, and loading-time navigation.
3. Confirm route, title, reader, strip, status, and cover always correspond only to the latest target.
4. Confirm full image actual pixels when full image exists:
   - `img.src` present,
   - `naturalWidth > 0`.
5. Confirm `no_full` entries are shown as `no full` and are not treated as actual image pixels.
6. Confirm search-card TN is displayed only as search-card cover, not as viewer display.
7. Confirm page movement does not wait for `post read` or `showLists()` before display.
8. Confirm background queue is not recomputed for every next-page movement unless target changed.
9. Go tests must use mocked `Client.fetch` and must not call external APIs.
10. Go tests must assert lightweight `showPage()`, background/debounce for read/lists, no TN viewer fallback, no full handling, stale-write prevention, and current priority behavior.
