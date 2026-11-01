# UI priority queue and thumbnail placeholder implementation

**Status**: 進行中
**Date**: 2026-11-01

## 意図 / 背景

- 操作・表示を画像 download 待ちにしない。
- gallery switch 時に old strip thumbnails を残さず immediate placeholder へ。
- current displayed full image を priority で fetch。
- strip/search card は full cache objectURL を優先、未 cache は placeholder。search card TN は low priority で残す。

## 必要機能 / 確認手順

- Required functionality: actual viewer image pixels must be visible when an image exists. Search-card TN cover alone is not enough. Status text, generation guard, metadata, alt text, empty `src`, or broken placeholder is not completion.
- Verification must use the actual running server at `http://127.0.0.1:8731`, not mocked tests.
- Pass criteria: `#reader img`, `#thumbs img`, and `#results img` must have `img.src` present and `naturalWidth > 0` for expected displayed images. A `naturalWidth=0` reader/strip display is abnormal/fail.
- Code tests must use mocked `Client.fetch` and cannot substitute actual screen verification.
- Current conflict: the current implementation disables TN display fallback. If viewer completion requires actual image pixels, current cacheless-screen result remains fail until display behavior or completion definition is clarified.
- Loading-navigation behavior must also be confirmed: during pending loads, page movement, page input, gallery switch, or search replacement must immediately reflect the new target, and stale async writes must be ignored.
- Confirm that `openGeneration`, `showGeneration`, `renderGeneration`, `searchGeneration`, `prefetchAbort`, render/page/hash guards, and per-gallery counts prevent stale route, title, reader, strip, cover, status, or priority writes.
- Expected result after loading-time switch: current DOM, status, priority, and route correspond only to the latest target. If a stale load writes any of these, verification fails.
- Decision entry: `docs/ai/decisions/2026-11-01-required-function-verification.md`.

## 運用決定（セッション横断で維持）

- Server EXE must remain running for user operation verification. Do not stop it unless the server EXE is being updated. Stop/rebuild/restart is allowed only for EXE updates.
- Canonical decision entry: `docs/ai/decisions/2026-11-01-server-exe-keep-running.md`.
- Current runtime policy as of 2026-11-01: keep `hub.exe view` on `http://127.0.0.1:8731`. Current process ID `bgp_1959f6bef001P6cmdcPdsA5W3y`. If replaced after an EXE update, record the new process ID, URL, command, and lifetime here.
- Screen verification must check actual rendered image pixels, not only status text or generation guards. A reader/strip `<img>` with `src=''` or `naturalWidth=0` is not considered displayed.

## 調査結果 / 変更内容

- CP1: `openGallery()` の uncached pathで `renderStripLoading(id)` を immediate表示。
- CP2: `startFullPrefetch()` の background worker を current displayed priority promise まで wait。reader placeholder は TN fetchせず empty placeholder。`storeFullCache()` が strip placeholder を objectURL で update。
- CP3: search card は metadata-first placeholder、cover TN は `currentPriorityDone` の後、full cache objectURLがあればそちら優先。`thumbProxy()` は full cache hitをTN remoteより前へ。
- CP4: viewer display は TN fallback を表示しない。`/api/image` は local / full cache / remote full だけへ変更。UI の `cachedFullSrc()` は `tn` と `tn-cache` を display source にしない。
- CP5: TN thumbnails を persistent cache へ保存。`thumbProxy()` は full cache hit → TN variant cache hit → remote TN の順。remote TN 成功時は `cache/images/thumbs/<hash>.<format>.<variant>` へ保存し、cache hit は `x-cache: tn-cache` で返す。
- CP6: top status は display intent に合わせる。`prefetch-status` を `full display cache X/Y TN display fallback disabled` に変更し、viewer display TN count を出さない。
- CP7: cache-less actual screen verify revealed that `/api/image` 404/no-full was being counted as `image load failed` error. `imageProxy()` returns `x-cache: no_full` for storage 404, UI counts `no_full` separately, note shows `no full image`, and status shows `full display cache X/Y | no full Z/Y | TN display fallback disabled`.

## CP9 fast non-blocking viewer plan

Status: active implementation plan.
Decision entry: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`.

Plan:

1. Keep TN display fallback forbidden for `#reader` and `#thumbs`.
2. `showPage()` becomes lightweight:
   - update current page/route,
   - render reader/strip from existing full cache,
   - start single current target priority only if full image is not already cached,
   - keep existing background queue when target remains in the same gallery,
   - update the queue only when target changes.
3. Remove `await post("/api/read")` from the display path. Read update is background/debounced and guarded.
4. Remove `await showLists()` from frequent page movement. Lists update is background/debounced or called when lists actually need refresh.
5. Search-card TN remains separate low-priority. It is not viewer display fallback.
6. `loadFullImage()` and `prefetchPageImage()` continue to fetch full image only. They must not call TN display helper for viewer.
7. `no_full` and true `error` remain separate. Viewer `no_full` is explicit status, not actual image pixels.
8. Stale async writes must be prevented by generation guards, `signal.aborted`, render/page/hash checks, and per-gallery counts.
9. Go tests must assert:
   - `showPage()` does not block display on `post read` or `showLists()`,
   - background queue remains stable on next-page movement,
   - no TN viewer display fallback,
   - `no_full` remains separate from true `error`,
   - stale writes after page/gallery/search navigation are ignored.
10. Actual screen verification must confirm route/title/reader/strip/status/cover correspond only to latest target, and full actual pixels are shown only when full image exists.

## CP8 revised actual viewer display policy

Recorded policy decision:

- TN display fallback for viewer display is absolutely forbidden.
- `ensureTNDisplay()` must not be used for `#reader` or `#thumbs` display. It may exist only as a search-card TN helper, or be removed.
- Viewer completion is satisfied only by actual full image pixels from `fullCache.objectUrl` / local file. Search-card TN cover is separate and does not satisfy viewer display.
- If the full image is unavailable, UI must show `no full`, not TN pixels. A `no_full` viewer state is correct status behavior, but it is not an actual image-display pass.
- Search-card TN may remain under the existing search-card condition. It must be handled separately from viewer full-image requests.
- Page movement should be lightweight. Frequent next-page movement should not recompute the whole background order every time.

CP8 revised plan:

1. `displayPlaceholder(entry)` returns `""` for viewer placeholders.
2. `cachedFullSrc()` returns `fullCache.objectUrl` only for actual full image objectURLs. TN objectURLs must not be used as viewer display sources.
3. `loadFullImage()` fetches full image only. Full success creates `objectUrl`. `no_full` counts `no_full`. True network failure counts `error`. No TN fetch/display is allowed in `loadFullImage()`.
4. `prefetchPageImage()` background prefetches full image only. It must not call TN display fallback.
5. Search-card TN remains low-priority. It uses the existing search-card TN path. This path is separate from viewer display.
6. `showPage()` should be lightweight for page movement: update route/current page, render from cache, set current target priority, keep existing background queue unless the target changes, and run read/lists as background/debounced work.
7. Loading-time navigation must remain current-target-only with generation guards. Stale async writes must not modify current DOM, route, title, reader, strip, cover, status, or priority.
8. Go tests must assert no TN display fallback for viewer, `no_full` handling, search-card TN separation, lightweight page movement, and stale-write prevention. Tests must not call external APIs.
9. Actual screen verification must check:
   - Search-card TN cover is displayed when expected.
   - Viewer shows actual full pixels only when full image exists.
   - Viewer `no_full` entries must not be treated as image pixels.
   - Stale loads after page movement do not write DOM/status.
10. Build EXE, restart server, actual screen verify.

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

### Current actual screen verification

- URL: `http://127.0.0.1:8731/?x=current#/g/4296091/page/1-2`
- Search-card cover: actual TN images are displayed (`img src` present, `naturalWidth > 0`).
- Viewer display: abnormal. Reader/strip `<img>` have `src=''` and `naturalWidth=0`; actual image pixels are not displayed.
- Status: `full display cache 0/40 | no full 39/40 | TN display fallback disabled`
- Screenshot: `scratch\hitomi-usage\current_after_load_20261101.png`
- Known issue: `docs/ai/known-issues/2026-11-01-viewer-display-broken-placeholder.md`

This result is not accepted as completion if the completion requires actual viewer image display. Future screen verification must inspect actual rendered pixels, not only status/generation guard behavior.

### CP8 lightweight non-blocking viewer verification

- `go vet ./...` pass.
- `go test .` pass `21.354s` with updated assertions for lightweight `showPage()`, stable background queue, no TN viewer display fallback, `no_full` handling, and stale-write prevention.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Active persistent process started: `bgp_1964cf66b0011stNFl6vNPrs5F`, `http://127.0.0.1:8731`, lifetime persistent.
- Actual screen URL: `http://127.0.0.1:8731/?x=cp7lightweight#/g/4295779/page/3-4`
- Search-card TN covers have `img src` and `naturalWidth > 0`.
- Full image display pass: reader `#reader img` has blob objectURL and `naturalWidth > 0` for `4295779` page 3-4.
- Strip display pass: `#thumbs img` has blob objectURL and `naturalWidth > 0` for cached full entries.
- `no_full` gallery `4296091`: reader/strip have no TN objectURL; `no_full` status is explicit, and no TN viewer fallback is present.
- Lightweight page movement check:
  - `showPage()` contains no `await post("/api/read")` and no `await showLists()`.
  - `showPage()` calls `ensureBackgroundPrefetch(displayed)`.
  - `runBackgroundReadAndLists()` is generation-guarded background/debounce.
  - `showPage()` returned in `2ms` during page movement.
  - background queue was not recomputed for the same gallery; `prefetchQueueKeys` remained stable.
- Screenshot: `scratch\hitomi-usage\cp8_lightweight_actual_full_pixels.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
