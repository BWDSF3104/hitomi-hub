# UI priority queue and thumbnail placeholder implementation

**Status**: 進行中
**Date**: 2026-11-01

## 意図 / 背景

- 操作・表示を画像 download 待ちにしない。
- gallery switch 時に old strip thumbnails を残さず immediate placeholder へ。
- current displayed full image を priority で fetch。
- strip/search card は full cache objectURL を優先、未 cache は placeholder。search card TN は low priority で残す。

## 調査結果 / 変更内容

- CP1: `openGallery()` の uncached pathで `renderStripLoading(id)` を immediate表示。
- CP2: `startFullPrefetch()` の background worker を current displayed priority promise まで wait。reader placeholder は TN fetchせず empty placeholder。`storeFullCache()` が strip placeholder を objectURL で update。
- CP3: search card は metadata-first placeholder、cover TN は `currentPriorityDone` の後、full cache objectURLがあればそちら優先。`thumbProxy()` は full cache hitをTN remoteより前へ。

## Verification

### CP1 immediate strip placeholder
- `go vet ./...` pass.
- `go test .` pass `23.715s` with new strip loading assertion.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp1`: uncached `openGallery(4295772,3)` before `await api` showed reader `読込中`, strip `strip loading`, status `gallery read load: 4295772`.
- Screenshot: `scratch\hitomi-usage\cp1_strip_loading_immediate.png`

### CP2 reader empty placeholder, strip full-cache priority, current image priority
- `go vet ./...` pass.
- `go test .` pass `23.496s` with new strip/full-cache/priority assertions.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp2verify`:
  - full card data pathで await前に reader src empty, strip placeholder empty with `data-key='4295774|entryhash|webp'`.
  - `currentPriorityDone` was pending before background prefetch.
  - `storeFullCache()` updated strip img src.
- Screenshot: `scratch\hitomi-usage\cp2_strip_placeholder_priority_pending.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
