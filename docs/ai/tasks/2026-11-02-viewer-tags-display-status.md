# Viewer tags display area and display/status separation

**Status**: 進行中
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

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
