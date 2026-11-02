# Manual full image retry correction method and pass criteria

Status: active. This decision sets how manual full-image retry should be added without changing the current automatic behavior.

## Current behavior

- Full image fetch failure is recorded as `error` or `no_full`.
- No automatic retry is performed.
- `prefetchedEntries` prevents background requeue for an already processed key.
- `statusIsViewerDisplaySource()` rejects `error`, `no_full`, and `tn` as display sources.
- TN display fallback is disabled.

## Correction method

1. Add a manual retry button to the UI.
   - Recommended placement: near `#prefetch-status`, not inside `form#search`.
   - Example UI:
     - `#retry-failed`
     - button label: `retry failed`
   - The button must be visible, actionable, and must not change the strict initial display criteria for `form#search`, `.side`, or `#results`.

2. Retry must be manual only.
   - No automatic retry on failure.
   - No `setTimeout`, no backoff, no background requeue.
   - The button click is the only trigger.

3. Add a manual retry function.
   - Recommended function name: `manualRetryFailedFullImages()`
   - It should target only entries whose current status is `error`.
   - Do not retry `tn` or `no_full` unless a separate explicit policy is added later.

4. Add re-entrancy protection.
   - Use `manualRetryingKeys`.
   - If a key is already being manually retried, skip it.
   - Existing `prefetchQueueKeys` and background worker behavior remain unchanged.

5. Add optional status recount support.
   - Extend `updatePrefetchCounts(status, entry, galleryId = current.id, allowRecount = false)`.
   - Default `allowRecount = false`, so existing automatic behavior remains unchanged.
   - Manual retry calls `updatePrefetchCounts()` with `allowRecount = true`.
   - When `allowRecount` is true:
     - decrement the old status count before setting the new status;
     - set the new status in `prefetchedEntries`;
     - increment the new status count.
   - This prevents manual retry from being silently ignored because `prefetchedEntries.has(key)` already returns early.

6. Preserve existing generation and abort guards.
   - `showGeneration`, `openGeneration`, `renderGeneration`, `backgroundGalleryId`, and `AbortSignal` checks must remain.
   - If the gallery, page, render, or signal changes during a manual retry, the stale retry must not update UI, display, or status counts.

7. Preserve existing display-source behavior.
   - `statusIsViewerDisplaySource()` remains the source of truth.
   - `displayPassByEntry` is recorded only when an img actually has natural pixels.
   - TN display fallback remains disabled.

8. Manual retry execution scope:
   - For currently displayed failed entries, call `loadFullImage()` with the current generation values and `allowRecount = true`.
   - For non-displayed failed entries, call `prefetchPageImage()` with `allowRecount = true`.
   - If a currently displayed img exists for the retried entry, update its `src` through `loadFullImage()`.
   - If only background entries are retried, update cache/status, but `actual pixels` must not increase until the image is actually displayed.

## Pass criteria

### Existing behavior preservation

1. Before a manual button click, no automatic retry is performed.
2. A failed entry remains `error` until manual retry is explicitly clicked.
3. `tn` and `no_full` are not automatically retried.
4. `prefetchedEntries` continues to block automatic background requeue for already processed keys.
5. `statusIsViewerDisplaySource()` continues to reject `error`, `no_full`, and `tn`.
6. TN display fallback remains disabled.

### Manual retry behavior

7. `#retry-failed` exists in the UI and is actionable.
8. `#retry-failed` is disabled when there is no current gallery or neither `errorDone > 0` nor `noFullDone > 0`.
9. `#retry-failed` is enabled when `current.id` exists, `manualRetryRunning` is false, and either `Number(counts.errorDone) > 0` or `Number(counts.noFullDone) > 0`.
10. Clicking `#retry-failed` calls `manualRetryFailedFullImages()`.
11. Manual retry targets entries with `sourceStatusByEntry.get(key) === "error"` or `sourceStatusByEntry.get(key) === "no_full"`.
12. Manual retry does not double-fetch the same key because `manualRetryingKeys` is used.
13. Manual retry updates `prefetchedEntries` only through the recount path.

### Status correctness

14. When a manually retried entry fails again:
   - old error count is removed;
   - new error count is added;
   - net `errorDone` remains unchanged.
15. When a manually retried entry succeeds as full/cache:
   - `errorDone` decreases by 1;
   - `fullDone` or objectURL cache count increases by 1;
   - `prefetchedEntries` becomes the success status;
   - `sourceStatusByEntry` becomes the success status;
   - `displayPassByEntry` becomes true only if the displayed img actually loads.
16. When a currently displayed failed entry succeeds through manual retry:
   - `actual pixels` increases.
   - The reader img `src` becomes the object URL or local direct source.
17. When a non-displayed failed entry succeeds through manual retry:
   - status count changes correctly;
   - `actual pixels` must not increase until the entry is displayed.
18. If a retried entry returns `tn` or `no_full`, the status count must move from `errorDone` to `tnDone` or `noFullDone` through the recount path. TN display fallback remains disabled.

### Stale response guard

19. A manual retry response must not update UI or status if the current `showGeneration`, `openGeneration`, `renderGeneration`, `backgroundGalleryId`, or `AbortSignal` no longer matches.
20. A stale manual retry response must not double-count.

### Strict UI visual confirmation

21. Use a fresh isolated browser tab or fresh profile.
22. Use actual UI click, not only handler existence or DOM state.
23. Capture visual confirmation with a normal OS screen capture, not a Playwright/headless screenshot.
24. The confirmation must compare before and after:
   - before: `#retry-failed` exists and reflects the actual `errorDone` state;
   - after: the actual click or actual UI button action changes status as described by the pass criteria.
25. Initial display regression remains required:
   - direct server baseline passes;
   - UI network response matches direct server baseline;
   - `#results` card count matches actual server `ids.length`;
   - `form#search` does not wrap at the target viewport;
   - `.side` remains `480px` at `1280x900` or the documented target viewport;
   - no measurement may depend on resize.
26. The manual retry button must not change `form#search` height, `.side` width, or `#results` card height.
27. Fullscreen regression remains required if the visual confirmation uses fullscreen:
   - actual Fullscreen state is active;
   - browser chrome is hidden in normal screen capture;
   - double pages have no visual center seam gap.

### Code/test criteria

28. `ui.html` must contain:
   - `id="retry-failed"`
   - `manualRetryFailedFullImages()`
   - `manualRetryingKeys`
   - `allowRecount`
   - a click handler for `#retry-failed`
   - `sourceStatusByEntry.get(key) === "error"`
29. `updatePrefetchCounts()` must keep the existing default behavior when `allowRecount` is omitted or false.
30. Existing tests must continue to pass.
31. New tests must assert manual retry status recount and re-entrancy protection.
32. `go vet .` and `go test .` must pass in `S:\Documents\Kilo_Commons\hitomi-hub\app`.
33. Canonical build and canonical server confirmation must be re-run after the implementation change.
