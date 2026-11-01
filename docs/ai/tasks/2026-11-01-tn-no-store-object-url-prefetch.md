# TN no-store display and objectURL prefetch checkpoint

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: implement, test, screen verify, commit

## 意図 / 背景
- 改修後に初めて開く gallery は `loadFullImage()` が現行挙動で動作するが、すでに描画済みで TN に固定された items は browser cache の古い TN response を返しやすく、フル image が取り直せない。
- `loadFullImage()` は `x-cache` を見るために fetch した後、同じ URL を `img.src` に再設定するため、display path で二重 fetch になりやすい。

## 調査結果 / 変更内容
- `imageProxy` の TN response は `cache-control: no-store` にする。
- local / persistent full cache / full miss は long cache を維持する。
- UI の `loadFullImage()` と `prefetchPageImage()` は `fetch(..., { cache: "no-store", signal })` を使う。
- `loadFullImage()` は response body を blob で受け取り、`URL.createObjectURL(blob)` で表示する。
- display objectURL は `fullObjectUrls` に登録し、`renderReader()` で revoke する。

## Verification
- `go vet ./...`: pass.
- `go test .`: pass, `31.082s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`: pass.
- Screen verify objectURL display at `http://127.0.0.1:8731/?x=tnendpoint2#/g/4295693/page/1-2`:
  - status `full prefetch 15/15 tn 0/15` for this gallery after prior cache was already present.
  - displayed `img.dataset.fullCache`: `cache`
  - displayed `img.src`: `blob:...`
  - `fullObjectUrls` count: `2`
- Screen verify TN response header at `http://127.0.0.1:8731/?x=tnendpoint2#/g/4295693/page/1-2`:
  - fetch `/api/image/6cf66a705f1c527c499dbd5b66fb07e506ed72cdd723b7747f1161517828667e?format=png&gallery=4295693` with `cache: "no-store"`
  - `x-cache`: `tn`
  - `cache-control`: `no-store`
  - `content-type`: `image/webp`
- Screenshot: `scratch\hitomi-usage\tn_no_store_object_url.png`.

## 完了時 Memory Bank 操作
- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
