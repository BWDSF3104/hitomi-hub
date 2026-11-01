# Implement P3 adopted plans from lowest difficulty

**Status**: 進行中
**Date**: 2026-11-02
**Next Action**: Set tests for P3 adopted plans and implement from #16 #11 #13 #15 #18 #14 #10 #17 #12 with screen verify and checkpoint commits

## 意図 / 背景

P3 backlog の採用済み方針を実装する。難易度の低い項目から順に、項目別に tests を設定し、既存機能影響があれば既存機能確認 tests も追加する。実装ごとに画面確認を行い、合格まで修正を繰り返す。想定通り完了した checkpoint ごとに記録と commit を行う。

## 調査結果 / 変更内容

Planned order by difficulty:

1. `#16` tab title to current work
2. `#11` gallery/viewer position display only
3. `#13` search result prev/next page buttons
4. `#15` view mode buttons and state row
5. `#18` default RTL spread display order and toggle button
6. `#14` fullscreen state separation and resize/error handling
7. `#10` search latency measurement
8. `#17` same-title Japanese anchor on display cards
9. `#12` normalized cache acquisition by copy, page-number filenames, overwrite confirmation

## Checkpoint #16 tab title

Implemented in `app/ui.html`:

- `updateTabTitle(title)`
- called from `setGalleryMeta()`, `showGalleryMetaFromCache()`, `renderGalleryLoading()`
- sets `document.title` to `japanese_title || title || id`, with suffix ` - hitomi hub viewer`

Test added in `app/viewer_test.go`:

- `TestEmbeddedUIUpdatesTabTitleForCurrentWork`

Verification:

- `go test .`: pass
- `go vet .`: pass
- Screen verify:
  - URL: `http://127.0.0.1:8731/?x=tab#/g/4295693/page/1`
  - `document.title`: `ぱんぱんジャンヌちゃん - hitomi hub viewer`
  - Screenshot: `scratch/hitomi-usage/p3-16-tab-title-4295693.png`
  - Console: only image cache/no_full 404s on the open gallery; title behavior passed.

Canonical exe was rebuilt and canonical server restarted with the same test build.

## Checkpoint #11 gallery/viewer position

Implemented in `app/ui.html`:

- Added `#reader-position` inside `#reader-controls`.
- Added `renderPosition()` and `renderLoadingPosition()`.
- Search result position display is not added, matching the adopted scope decision.
- Calls position helpers from `setGalleryMeta()`, `renderGalleryLoading()`, `showPage()`, and `setViewMode()`.

Test added in `app/viewer_test.go`:

- `TestEmbeddedUIShowsGalleryViewerPositionOnly`

Verification:

- `go test .`: pass
- `go vet .`: pass
- Screen verify:
  - URL: `http://127.0.0.1:8731/?x=position#/g/4295693/page/5-6`
  - `#reader-position` text: `position: 5-6 / 15`
  - Screenshot: `scratch/hitomi-usage/p3-11-gallery-position-4295693-5-6.png`
  - Console: only image cache/no_full 404s on the open gallery; position display passed.

Canonical exe was rebuilt and canonical server restarted with the same test build.

## Checkpoint #13 search prev/next buttons

Implemented in `app/ui.html`:

- Added `#search-prev`, `#search-next`, `#search-page-status`.
- Added `searchTotalPages(total)`, `renderSearchPageStatus()`, `setSearchPage(delta)`.
- `runSearch()` stores `currentSearchTotal` and renders status without changing `search-status` semantics.
- `restoreSearchState()` renders saved total page status.
- Buttons clamp to page bounds and run search once.

Test added in `app/viewer_test.go`:

- `TestEmbeddedUIHasSearchPrevNextButtons`

Verification:

- `go test .`: pass
- `go vet .`: pass
- Screen verify:
  - URL: `http://127.0.0.1:8731/?x=searchpage#/q/furry/page/2`
  - Initial page status: `page 2 / 1316・total 32877`
  - Clicked next: page moved to `3`, status completed with 25 cards.
  - Clicked prev: page returned to `2`, status completed with 25 cards.
  - Screenshot: `scratch/hitomi-usage/p3-13-search-prev-next.png`

Canonical exe was rebuilt and canonical server restarted with the same test build.

## Checkpoint #15 view mode buttons and state

Implemented in `app/ui.html`:

- Added `#view-mode-buttons` with `data-mode="single|double|fullscreen"`.
- Added `#view-state` and `renderViewState()`.
- `setViewMode()`, `showPage()`, `setGalleryMeta()`, `renderGalleryLoading()`, `fullscreenchange`, and resize update view state.
- Button click uses `setViewMode(button.dataset.mode)` and `button.blur()` to keep keyboard `d/s/f` working.
- `f` became a fullscreen toggle when fullscreen state is active.

Test added in `app/viewer_test.go`:

- `TestEmbeddedUIHasViewModeButtonsAndState`

Verification:

- `go test .`: pass
- `go vet .`: pass
- Screen verify:
  - URL: `http://127.0.0.1:8731/?x=viewmode#/g/4295693/page/2-3`
  - Initial state: `view: double・display: single landscape・page 2-3 / 15・fullscreen off`
  - Clicked `single` button: state became `view: single・display: single landscape・page 2 / 15・fullscreen off`
  - Button focus cleared to `BODY` after `blur()`.
  - Pressed `d` after button click: state returned to `view: double` with pair `2-3`.
  - Screenshot: `scratch/hitomi-usage/p3-15-view-mode-buttons-state.png`

Canonical exe was rebuilt and canonical server restarted with the same test build.

## Checkpoint #18 RTL spread toggle

Implemented in `app/ui.html`:

- Added `spreadDirection = "rtl"` default.
- Added `SPREAD_DIRECTION_STORAGE_KEY`.
- Added `loadSpreadDirection()` and `setSpreadDirection(dir)`.
- Added `#spread-direction` button and key `r`.
- `renderReader()` reverses display order for RTL when multiple items are shown.
- Logical route/hash remains `page-second`.
- `setSpreadDirection()` and `resize` call `startFullPrefetch()` after render so full image display is re-evaluated.

Tests in `app/viewer_test.go`:

- `TestEmbeddedUIHasSpreadDirectionToggle`

Verification:

- `go test -count=1 .`: pass
- `go vet .`: pass
- Canonical exe rebuilt after test pass.
- Screen display check:
  - `http://127.0.0.1:8736/?x=displaycheck-cacheless#/g/4252697/page/2-3`
  - Current build showed `reader double`, `readerImageCount: 2`, objectURL cache, and display pass.
  - Full image display regression cannot currently be confirmed; per current instruction, it is lowest priority and handled only when an obvious full-image display failure is visible.

## Checkpoint #14 fullscreen state separation

Implemented in `app/ui.html`:

- `viewMode` is layout intent only (`single` / `double`).
- `fullscreenState` is presentation state (`off` / `on` / `fallback`).
- `lastViewMode` preserves layout mode across fullscreen exit.
- Fullscreen target prefers `#reader` so UI chrome can be separated.
- Fallback order is explicit:
  - `requestFullscreen`
  - `webkitRequestFullscreen`
  - CSS-only pseudo fullscreen
- Added `body.pseudo-fullscreen` reader overlay rules.
- `fullscreenchange` restores `lastViewMode`.
- `fullscreenerror` changes state to fallback and re-evaluates display/prefetch.
- `resize` re-renders reader, position, view state, and starts full prefetch.
- `f` toggles fullscreen through `setFullscreenFromGesture()` instead of making fullscreen a layout mode.

Tests in `app/viewer_test.go`:

- Extended `TestEmbeddedUIHasReaderSpreadAndFullscreen`
- `TestEmbeddedUISeparatesFullscreenStateAndRestoreLastViewMode`

Verification:

- `go test -count=1 .`: pass
- `go vet .`: pass
- Canonical exe rebuilt after test pass.
- Screen verify:
  - URL: `http://127.0.0.1:8736/?x=fullscreen#/g/4295693/page/2-3`
  - Initial state: `fullscreen off`
  - `setFullscreenFromGesture()`: `fullscreen on`, `last layout: double`, `body.fullscreen` applied.
  - Narrow viewport `600x800`: state showed `display: fullscreen` without fullscreen double fallback.
  - Exit fullscreen: `fullscreen off`, `last layout: double`, `body.fullscreen` removed.
  - Screenshot: `scratch/hitomi-usage/p3-14-fullscreen-state.png`
  - Full image display on this entry remained `no_full`; actual display pass was not available. This is not treated as a regression fix unless an obvious full-image display failure is confirmed.

## Checkpoint #10 search measurement

Implemented in `app/metrics.go`, `app/hitomi.go`, `app/viewer.go`, `app/ui.html`:

- Added `FetchMetric` records per URL kind: `calls`, `gate_wait_ms`, `network_ms`, `decode_ms`, `retry_sleep_ms`.
- `Client.getWithGate` records mock fetch calls, and `fetchRemoteWithGate` records gate wait, network, decode, and retry sleep separately.
- `classifyFetchURL` classifies version, index, nozomi, gg, image, gallery, and data URLs.
- `Viewer.search` adds `x-search-timing` header and JSON `timing` fields:
  - `search_ms`
  - `record_ms`
  - `json_ms`
  - `metrics`
- `Viewer.searchCard` sets `x-cache` as `miss` or `cache` based on `infoCache`.
- UI keeps `search-status` semantics unchanged and adds `#perf-status`.
- UI measures `/api/search` and each `/api/search-card/{id}` using `performance.now()`.
- UI displays cache distribution from `x-cache` response headers.

Tests in `app/viewer_test.go` and `app/hitomi_test.go`:

- `TestSearchHandlerIncludesMeasurementShape`
- `TestClientRecordsFetchMetricsForMockFetch`
- `TestEmbeddedUIHasSearchMeasurementStatus`
- Existing `TestViewerSearchCardHandlerReturnsResult` asserts search card `x-cache` cache hit.

Verification:

- `go test -count=1 .`: pass, `23.589s`
- `go vet .`: pass
- Canonical exe rebuilt after test pass.
- Screen cache-less measurement:
  - URL: `http://127.0.0.1:8736/?x=measurement-cacheless#/q/furry/page/1`
  - First run:
    - `perf: /api/search 12008ms・server search=9444;record=2559;json=0・cards 25 24061ms・cache cache=1 miss=24`
  - Second same query on the same server:
    - `perf: /api/search 9656ms・server search=7227;record=2425;json=0・cards 50 26659ms・cache cache=26 miss=24`
    - The second run confirmed `x-cache` cache distribution increased for cache hits.
- Warm root comparison for the same query:
  - `http://127.0.0.1:8731/?x=measurement-warm#/q/furry/page/1`
  - First warm run showed `/api/search 11833ms`, server `search=9478;record=2350`, cards `26496ms` before search-card `x-cache` response headers were added.
- Screenshot: `scratch/hitomi-usage/p3-10-search-measurement.png`

## Verification

No separate verification beyond checkpoint sections yet.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
