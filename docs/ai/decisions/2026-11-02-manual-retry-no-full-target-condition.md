# Manual full-image retry no_full target condition (2026-11-02)

Status: active.

## Correction method

- Existing task: `docs/ai/tasks/2026-11-02-manual-full-image-retry.md`.
- `manualRetryFailedFullImages()` must process entries whose `sourceStatusByEntry` status is `error` or `no_full`.
- Do not use the inverted guard form that skips `error` or `no_full`.
- The button must use `current.id` and `manualRetryRunning` guards. The exact pass condition is `button.disabled = manualRetryRunning || !current.id || !(Number(counts.errorDone) > 0 || Number(counts.noFullDone) > 0);`; therefore it is enabled only when `current.id` exists, manual retry is not running, and either `errorDone > 0` or `noFullDone > 0`.
- Existing generation guards, `manualRetryingKeys`, `manualRetryRunning`, `allowRecount = true`, `prefetchPageImage()`, and `loadFullImage()` behavior remain unchanged.

## Impact range

- `app/ui.html`: only the manual retry target condition inside `manualRetryFailedFullImages()`.
- `app/viewer_test.go`: replace the exact-string assertion that preserves the wrong guard with the correct combined target condition.
- No server-side cache, rate gate, TN display fallback, route, or page semantics change.
- `note: "no full image: <page>"` remains the target page label. The failed-entry count remains `no full <count>/<total>` in `prefetch-status`.

## Objective pass criteria

1. A failing test-first assertion must require:
   ```js
   if (sourceStatusByEntry.get(key) !== "error" && sourceStatusByEntry.get(key) !== "no_full") continue;
   ```
2. `go vet . && go test -count=1 -timeout 300s .` in `app` passes.
3. Canonical build freshness rule passes from `S:\Documents\Kilo_Commons\hitomi-hub`:
   - `python -m pytest tests -v`
4. Canonical `output/hitomi-hub/hub.exe` is rebuilt and the canonical server is restarted.
5. Fresh isolated browser tab confirms, for a `no_full` gallery state:
   - direct server baseline for the target hash returns `404` with `x-cache: no_full`,
   - `passCount` for the correct combined condition is greater than zero,
   - actual UI click on `#retry-failed` enters `manualRetryFailedFullImages()`,
   - UI network requests for `/api/image/` are made for the processed `no_full` entries,
   - live `manualRetryingKeys` and `manualRetryRunning` transition correctly during and after the click.
6. A normal OS screen capture is copied to the Google Drive confirmation date folder and recorded in the task. If the environment cannot provide a normal OS physical screen capture, that strict screen condition remains a blocker.

## Remaining unresolved criteria

Status: active. The target condition is implemented and verified for `no_full`; actual full-image pixels, production `errorDone`, and race detector remain open.

### Full-image actual pixels

- `no_full` display failure cannot be counted as a full-image visual pass because `statusIsViewerDisplaySource("no_full")` is false and no objectURL is created.
- Existing cache entries can satisfy actual pixels only when a UI-openable gallery contains one of the cached hashes.
- Recommended pass criteria:
  - actual UI opens the gallery through a real UI click/route, not source-string tests only;
  - server response for the displayed entry is `cache`, `local`, or successful remote full fetch;
  - UI display source is `objectURL` or `local-direct`;
  - displayed `img.naturalWidth > 0`;
  - normal OS physical screen capture shows the displayed image.
- If only controlled cache/test fixtures are used, record them as controlled confirmation, not production actual display.

### Reproducible `errorDone`

- Natural production `error` is not guaranteed when storage is missing, because server intentionally returns `404 no_full` with `x-cache: no_full`.
- UI `error` requires either:
  - `!r.ok` without `x-cache`, or
  - a fetch exception caught by `prefetchPageImage()` / `manualRetryFailedFullImages()`.
- Recommended reproducible method:
  - fresh isolated UI tab,
  - controlled actual UI network response for `/api/image/...` returning `500` without `x-cache`,
  - actual UI click on `#retry-failed`,
  - live UI state must show `errorDone > 0`, button enabled, `manualRetryingKeys` active during click, then empty after completion.
- This satisfies the UI error behavior path but does not satisfy a direct server natural-failure baseline. It should be recorded separately as a controlled UI test unless a natural backend failure occurs.

### Race verification

- Shared cache fields are protected by mutexes in the current implementation.
- Existing concurrent handler/cache tests pass under normal `go test -count=1 .`.
- `go test -race -count=1 .` remains blocked locally because cgo/gcc is unavailable.
- Recommended criteria:
  - keep existing mutex-based protection tests as the local pass,
  - run `-race` on CI or a machine with cgo/gcc,
  - do not claim data-race proof from normal tests alone.
