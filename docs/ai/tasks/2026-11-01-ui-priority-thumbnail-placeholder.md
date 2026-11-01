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
- CP4: viewer display は TN fallback を表示しない。`/api/image` は local / full cache / remote full だけへ変更。UI の `cachedFullSrc()` は `tn` と `tn-cache` を display source にしない。
- CP5: TN thumbnails を persistent cache へ保存。`thumbProxy()` は full cache hit → TN variant cache hit → remote TN の順。remote TN 成功時は `cache/images/thumbs/<hash>.<format>.<variant>` へ保存し、cache hit は `x-cache: tn-cache` で返す。
- CP6: top status は display intent に合わせる。`prefetch-status` を `full display cache X/Y TN display fallback disabled` に変更し、viewer display TN count を出さない。
- CP7: cache-less actual screen verify revealed that `/api/image` 404/no-full was being counted as `image load failed` error. `imageProxy()` returns `x-cache: no_full` for storage 404, UI counts `no_full` separately, note shows `no full image`, and status shows `full display cache X/Y | no full Z/Y | TN display fallback disabled`.

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

### CP3 search card metadata-first, low-priority TN cover after priority
- `go vet ./...` pass.
- `go test .` pass `23.337s` after selector syntax fix; final `go test .` pass `23.408s` after `revokeFullObjectUrls()` Set fix.
- New Go test `TestViewerThumbProxyUsesFullCacheBeforeTN` proves `thumbProxy()` uses existing full cache before remote TN.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp3fix`:
  - `renderSearchCard()` renders metadata and cover `<img src=''>` before `currentPriorityDone` resolves.
  - With pending `currentPriorityDone`, `loadSearchCardCover()` did not set cover `src`.
  - `storeFullCache()` updated the actual cover `<img>` to `object-url-cp3` and `object-url-strip7`.
  - With no full cache and `currentPriorityDone` resolved, `loadSearchCardCover()` set the low-priority TN URL `/api/thumb/cp3hash?format=webp&gallery=4295779&variant=bigtn`.
- Fixed issue: `loadSearchCardCover()` had invalid `"]` selector due backslash and matched `<li>` not cover `<img>`; changed to `img[data-search-id="..."]`.
- Fixed issue: `revokeFullObjectUrls()` assigned `[]` to `fullObjectUrls` instead of clearing the `Set`.
- Screenshot: `scratch\hitomi-usage\cp3_metadata_first_priority_pending.png`

### CP4 viewer display no TN fallback
- `go vet ./...` pass.
- `go test .` pass `21.790s` with image proxy no-TN assertions and UI `cachedFullSrc()` / `loadFullImage()` TN exclusion assertions.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp4noTnDisplay`:
  - `prefetchedEntries` status `tn` makes `cachedFullSrc()` return empty placeholder.
  - `loadFullImage()` with mock `x-cache: tn` stores `tn` status but `objectUrl` is null, reader img `src` remains empty, note shows `tn display fallback disabled: 1`.
- Screenshot: `scratch\hitomi-usage\cp4_no_tn_display_fallback.png`

### CP5 TN thumbnail persistent cache
- `go vet ./...` pass.
- `go test .` pass `21.408s` with `TestSafeThumbnailCachePathUsesVariantAndStaysInsideThumbCache` and `TestViewerThumbProxyPersistsTNThumbnailCacheAndServesCacheHit`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp5tnCache` using a pre-created runtime TN cache file `cache/images/thumbs/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89.webp.bigtn`:
  - `/api/thumb/<hash>?format=webp&gallery=1&variant=bigtn` returned `x-cache: tn-cache`, `cache-control: private, max-age=604800`, `content-type: image/webp` without remote TN fetch.
- Screenshot: `scratch\hitomi-usage\cp5_tn_persistent_cache_hit.png`

### CP6 display status wording
- `go vet ./...` pass.
- `go test .` pass `21.588s` with status text assertions.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted with rebuilt `hub.exe`.
- Screen verify `http://127.0.0.1:8731/?x=tnendpoint&v=cp6status`:
  - `tnDone = 52`, `fullDone = 0`, `total = 580` produced `full display cache 0/580 TN display fallback disabled`.
  - reader img remained empty placeholder.
- Screenshot: `scratch\hitomi-usage\cp6_status_display_cache_no_tn.png`

### CP7 cache-less actual viewing screen verify
- `go vet ./...` pass.
- `go test .` pass `21.447s` with `imageProxy` `x-cache: no_full` for storage 404, UI `loadFullImage()` `no_full` handling, `!r.ok` error branch, and `no full` status assertions.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active process restarted. Cache-less verify process `bgp_19617f7ed001TCkkqGgQhwks3U` restarted on port `8732` using runtime root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage\cp7_cacheless`.
- Actual cache-less screen verify `http://127.0.0.1:8732/?x=cp7cachelessactual`:
  - search results loaded live (`furry`, first id `4296091`), search card TN cover displayed through `/api/thumb` TN cache path.
  - reader display did not use TN fallback. Reader `src` remained empty.
  - missing full storage entries returned `x-cache: no_full`, UI note `no full image`, status `full display cache 0/40 | no full 12/40 | TN display fallback disabled`.
- Before fix, actual cache-less screen showed `image load failed: 18` and `error 93/105`; this is why `no_full` was separated from true network errors.
- Screenshot: `scratch\hitomi-usage\cp7_cacheless_actual_no_full.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
