# 即座読込表示と高速 gallery 切替

**Status**: 完了
**Date**: 2026-11-01

## 意図 / 背景

- Gallery 切替の体感速度を上げる。cache/full data がある場合は immediate render、未 data の場合は「読込中」を即表示する。

## 調査結果 / 変更内容

- 未 cache gallery の実際の page content は `/api/gallery/<id>` data（files・format・read_state・scene_indexes）を前提にするため、外部 data fetch gate を越えて完全な page 内容を即表示できない。
- `searchCard` は既に `v.info(id)` を呼ぶため、card response に `files`、`scene_indexes`、`format`、`read_state` を追加すれば、lazy card 読み込み済みの gallery からは data fetchを awaitせずに reader を描画できる。
- `viewer.go`: `searchResult()` に `files`、`scene_indexes`、`format`、`read_state`、`local_formats` を追加。mini page thumbnails は追加しない。
- `ui.html`: `galleryDataFromCard(card)` で search card の full data を gallery data shapeへ変換し、`openGallery()` の immediate pathで使う。
- `ui.html`: `renderGalleryLoading(id, page, second)` を追加し、full data が無い gallery では route/meta/status 表示後に reader area へ `読込中` と cover placeholder を即表示する。
- `openGallery()`: route update → full card data check → cached gallery path → loading UI → `await api("/api/gallery/" + id)` の順序を維持。

## Verification

- `go vet ./...` pass.
- `go test .` pass `23.825s` (new `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen`, `TestEmbeddedUIShowsImmediateReaderLoadingForUncachedGallery`).
- `go build -o output\hitomi-hub\hub.exe` pass.
- Active process `bgp_1959f6bef001P6cmdcPdsA5W3y` rebuilt to `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe` and restarted with new UI.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=newui2`:
  - uncached card click pathで `await api`前に hash `#/g/4295770/page/2`、reader area `読込中: loading verify card page 2`、`data-reader-loading` を確認。
  - full card data pathで `await`前に hash `#/g/4295771/page/1`、reader entry `data-page='1'`、gallery meta、pagenum を確認。
  - Screenshot: `scratch\hitomi-usage\immediate_reader_loading.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
