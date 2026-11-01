# Immediate gallery route/cache during full prefetch

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: Record completed immediate gallery navigation checkpoint.

## 意図 / 背景

ユーザー報告: full prefetch中にほかの作品を押しても即座に作品ページへ移動しない。`openGallery()`はclick handlerで`preventDefault`し、gallery data awaitまで route/search meta を更新しないため、画面・URLが古いgalleryに残る問題があった。

## 調査結果 / 変更内容

- `openGallery()`の冒頭で `prefetchAbort.abort()` を即実行し、旧 full prefetch を停止する。
- `openGallery()`の冒頭で `history.replaceState(null, "", route(id, page, second))` を実行し、gallery data await前にrouteを作品ページへ更新する。
- `galleryCache` に開いた gallery data を保存し、cache済みのgalleryは `api("/api/gallery/")` await前に `setGalleryMeta()`/`showPage()` で即renderする。
- `searchCardCache` にlazy search card metadataを保存し、未cache galleryでは route/search card title/tagsと `gallery read load: <id>` statusを即表示する。

## Verification

- `go vet ./...` in `hitomi-hub/app`: pass.
- `go test .` in `hitomi-hub/app`: pass `23.304s`.
- `go build -o output\hitomi-hub\hub.exe .`: pass.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint`:
  - uncached search card click/open callで `openGallery()` await前に hashが `#/g/4295770/page/1` へ更新され、status `gallery read load: 4295770` を確認。
  - cache済み `galleryCache` の galleryで page3へ即移動し、`#/g/4295770/page/3-4` と reader objectURL表示を確認。
- Screenshots:
  - `scratch\hitomi-usage\immediate_gallery_route_search_card.png`
  - `scratch\hitomi-usage\immediate_cached_gallery_movement.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
