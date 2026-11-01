# Viewer strip group control and already-double aspect handling

**Status**: 完了
**Date**: 2026-11-01
**Backlog**: P2 #13 #14
**Next Action**: commit checkpoint.

## 意図 / 背景

ユーザー報告 2026-11-01:
- thumbnail strip の group/page selection は現状 current page に依存するだけで、任意 group を操作する UI が不足している。
- viewer reader で `double` 表示が view mode / window width によって決まるが、entry 自体がすでに横方向（見開き相当）の aspect ratio なのに、第二 page と pairing され `reader double` として表示されている。

これはユーザー報告に基づく確認作業であり、まだ実装開始指示は受けていない。記録のみ実施し、実装・ファイル変更は行わない。

## 調査結果 / 変更内容

- `ui.html` の `stripGroupPage(items)` は current page から group を選択し、`renderStrip()` がその current group の 50 件を render する。group/page selection を操作する UI control は存在しない。
- Playwright 実測 `http://127.0.0.1:8736/?x=cp14-strip-landscape-check#/g/4252697/page/1-2`:
  - `thumbsControlCount: 0`
  - `thumbsRenderedCount: 50`
  - `readerClass: "reader double"`
  - `readerImageCount: 2`
- 実測 gallery entry:
  - route: `#/g/4252697/page/1-2`
  - page1 entry hash: `899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9`
  - `width: 3746`, `height: 2525`
  - page2 portrait entry と隣接表示されている。
- `displayMode()` は window widthまたは `viewMode=double` で `double` を返す。`readerClass()` は `displayMode()` を反映する。`readerItems()` は `doubleActive() && first.entry` で第二 item を追加し、entry の `width` / `height` を見て「すでに横方向＝見開き」かどうかを判定していない。
- candidate design direction（未承認）:
  - strip group に明示的な group/page controls を追加する。
  - `entry.width > entry.height` の entry は already-double/single full-width 扱いし、`readerItems()` が pair しない方針を検討する。
  - `viewMode=double` は portrait page pairing の意味に限定し、already landscape entry を無理に half-width pair にしない方向を検討する。

- Known issues created:
  - `docs/ai/known-issues/2026-11-01-strip-group-no-user-control.md`
  - `docs/ai/known-issues/2026-11-01-landscape-entry-forced-double.md`

## 実装内容

- `app/ui.html`:
  - Added `#strip-controls` UI with previous group, group input, go, next group, and status.
  - Added `stripGroup`, `stripGroupTotal()`, and `stripGroupPage()` selectable group selection.
  - `renderStrip()` renders the selected group and updates controls.
  - `openGallery()` resets the selected group for a new gallery target.
  - Added `isAlreadyDoublePage(entry)` using `entry.width > entry.height`.
  - `readerItems()` does not pair an already landscape/spread entry.
  - `readerClass(firstEntry)` accepts reader items or entries and returns `reader single landscape` for already landscape entries.
  - Added `startFullPrefetch()` helper for `setViewMode()` so full prefetch is defined.
- `app/viewer_test.go`:
  - Added `TestEmbeddedUIStripControlsUseSelectableGroup`.
  - Added assertions for `isAlreadyDoublePage`, `readerClass(firstEntry)`, `reader single landscape`, `readerClass(items[0])`, and `startFullPrefetch()`.

## Verification

- `go vet .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `21.455s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp15.exe .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass. `hub.exe` was locked by canonical server, so this checkpoint used `hub.cp15.exe` for cache-less verification.

Actual display pass on cache-less runtime root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`:
- URL initial: `http://127.0.0.1:8736/?x=cp15-landscape-fixed#/g/4252697/page/1-2`
- `readerClass`: `reader single landscape`
- `readerImageCount`: `1`
- reader actual display ok: `true`
- `stripControlsCount`: `4`
- `stripGroupStatus`: `strip group 1/5`
- `stripRenderedCount`: `50`
- `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`

Strip group control actual screen operations:
- Click `#strip-next-group`: `stripGroup` became `2`, status `strip group 2/5`, rendered group pages `51` to `100`.
- Fill `#strip-group` with `5`, click `#strip-go-group`: `stripGroup` became `5`, status `strip group 5/5`, rendered group pages `201` to `203`.

Actual screen simultaneous display pass:
- URL: `http://127.0.0.1:8736/?x=cp15-results-search#/g/4252697/page/1`
- `readerClass`: `reader single landscape`
- `readerImageCount`: `1`
- reader actual display ok: `true`
- `#results img`: `25/25` actual pixel display ok
- `#thumbs img`: `50/50` actual pixel display ok
- `stripControlsCount`: `4`
- `stripGroupStatus`: `strip group 1/5`

Double portrait regression:
- URL: `http://127.0.0.1:8736/?x=cp15-results-search#/g/4252697/page/2-3`
- `readerClass`: `reader double`
- `readerImageCount`: `2`
- reader pages: `2`, `3`
- `#reader img`: `2/2` actual pixel display ok
- `#results img`: `25/25` actual pixel display ok
- `#thumbs img`: `50/50` actual pixel display ok
- `stripControlsCount`: `4`
- `stripGroupStatus`: `strip group 1/5`
- `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`

Known issues updated to resolved.

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [x] `backlog.md` の完了項目を削除（該当する場合）
