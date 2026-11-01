# Viewer tags display area and display/status separation

**Status**: 完了
**Date**: 2026-11-02
**Backlog**: P2 #13, #14
**Next Action**: Implement tags display area clamp, then implement displayPass/sourceStatus separation and page-switch direct local src; screen-confirm each checkpoint.

## 意図 / 背景

Search result tags display area が tag 数で card 全体の高さを長くしているため、thumbnail height に clamp する。View display の actual pixel truth と backend status truth を分離し、page switch で local full file を即表示できるようにする。Status line は actual pixels と source/cache counts を明確に分ける。

## 調査結果 / 変更内容

（記入）

## Verification

### CP18 search tags display area clamp

Implementation:
- `app/ui.html` CSS: `.card` defines `--search-card-height: 130px`, `.tags` clamps tags area to thumbnail height using `max-height`, `overflow: hidden`, `-webkit-line-clamp: 9`.
- `renderSearchCard(r)` adds `title` containing full tags and renders escaped tags.

Verification:
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `22.932s`.
- `go vet .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- Built `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp20.exe`.
- Cache-less ongoing server process: `bgp_197c2dbe2001Ml4n93T8LTyzjN`, root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, URL `http://127.0.0.1:8736`.

Actual screen pass:
- URL: `http://127.0.0.1:8736/?x=cp20-tags-clamp#/q/furry/page/1`
- Search results actual pixel display: `25/25` with `naturalWidth > 0`.
- Tags areas: `25` elements.
- Computed `.tags` max-height: `130px` for all tags areas.
- `-webkit-line-clamp`: `9`.
- Tags areas over `130px` scroll height are visually limited by `clientHeight: 130px`; full tags remain available through `title`.
- Sample clamped area:
  - `scrollHeight`: `972`
  - `clientHeight`: `130`
  - `title` contains full tags.
- Screenshot: `scratch/hitomi-usage-fixed-ongoing/cp20_tags_clamp_actual_display.png`

Backlog `#13` is completed and removed.

### CP19 viewer display/status separation

Implementation:
- `app/ui.html` separates `displayPassByEntry`, `sourceStatusByEntry`, `sourceTypeByEntry`, and `activeObjectUrlKeys`.
- `statusIsViewerDisplaySource()` excludes `tn`, `tn-cache`, `no_full`, and `error` from viewer display source truth.
- Viewer display source selection prefers active objectURL, then direct local `/files/...` for local entries.
- TN bytes are stored as `tn` source status only, not as viewer display objectURL.
- `renderReader()` and `renderStrip()` mark viewer-display images and attach `load` handlers.
- `recordImageDisplayPassFromImg()` records `displayPass` only when `naturalWidth > 0`.
- `prefetchPageImage()` uses direct local `/files/...` as source for already-downloaded local entries and avoids fetching when an active objectURL is already available.
- `prefetchStatus` reports `actual pixels`, `local direct`, `objectURL cache`, `no full`, and `TN display fallback disabled`.

Verification:
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `21.630s`.
- `go vet .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- Built `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp22.exe`.
- Cache-less ongoing server process: `bgp_197d0b346001TJA8l1bNZoWYPt`, root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, URL `http://127.0.0.1:8736`.

Actual screen pass:
- URL: `http://127.0.0.1:8736/?x=cp22-display-status-fix#/g/4252697/page/1`
- Status line:
  `actual pixels 50/203 | local direct 0/203 | objectURL cache 203/203 | no full 0/203 | TN display fallback disabled`
- Actual pixel panels:
  - `#reader img`: `1/1` with `naturalWidth > 0`
  - `#thumbs img`: `50/50` with `naturalWidth > 0`
  - `#results img`: `25/25` actual pixel display ok; these are search-card TN covers, not counted as viewer actual pixels
- Screenshot: `scratch/hitomi-usage-fixed-ongoing/cp22_display_status_actual_display.png`

Page switch timing after cache objectURL is active:
- `showPage()` to `#/g/4252697/page/2-3`: `8.8ms`
- `#reader img`: `2/2` actual pixels, `naturalWidth > 0`
- Reader source types on displayed pages: `objectURL`, `objectURL`
- Direct local source path is implemented for entries with `entry.local_formats`, but the current cache-less root showed `objectURL cache` entries rather than `local_formats` entries.

Backlog `#14` is completed and removed.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
