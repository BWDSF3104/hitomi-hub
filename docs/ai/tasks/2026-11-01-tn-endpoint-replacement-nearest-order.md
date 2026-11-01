# TN endpoint replacement and nearest-order prefetch fix

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: usage memo update and checkpoint commit

## 意図 / 背景
- x-cache 状態は full と TN fallback を正しく分ける必要があるが、表示上は TN fallback の endpoint bytes であっても placeholder から画像 endpoint へ置き換えるべきだとユーザーが指摘した。
- Background prefetch は現在閲覧中の gallery page 付近の full images を優先すべき。

## 調査結果 / 変更内容
- `loadFullImage()` は `x-cache` を `updatePrefetchCounts()` で full/tn/error として数え、表示置き換えは `cache !== "tn"` に依存しない。
- `loadFullImage()` は置き換え時に `img.dataset.fullCache = cache` を保持し、`img.src = fullSrc` で placeholder を画像 endpoint に置き換える。
- `startFullPrefetch()` は display items を先に fetch し、background order は `current.page` 付近の nearest-first にする。
- Nearest order は `currentIdx + distance` と `currentIdx - distance` を距離順で走査し、`displayedPages` を skip する。
- Test expectations を現行挙動に更新した。

## Verification
- `go vet ./...`: pass.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe`: pass.
- `go test ./app`: pass, `30.017s`.
- Screen verify at `http://127.0.0.1:8731/?x=tnendpoint#/g/4295693/page/5-6`:
  - status `full prefetch 0/15 tn 15/15`
  - `fullDone: 0`, `tnDone: 15`
  - displayed source becomes `/api/image/...`
  - `img.dataset.fullCache: "tn"`
  - natural size remains TN `211x300` because the server returned TN bytes.
  - Screenshot: `scratch\hitomi-usage\tn_endpoint_replaced_status.png`.
- Screen verify at `http://127.0.0.1:8731/?x=fullendpoint#/g/2648601/page/2-3`:
  - status `full prefetch 27/27 tn 0/27`
  - `fullDone: 27`, `tnDone: 0`
  - natural full size `2059x2880`
  - display size `500x700`

## 完了時 Memory Bank 操作
- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
