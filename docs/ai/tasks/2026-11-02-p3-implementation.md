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

## Verification

No verification yet.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
