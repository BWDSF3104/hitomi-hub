# UI priority queue and thumbnail placeholder implementation

**Status**: 完了
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
- Current runtime policy as of 2026-11-01: keep `hub.exe view` on `http://127.0.0.1:8731`. Current process ID `bgp_196b07014001jT62QJx8infSQG`. If replaced after an EXE update, record the new process ID, URL, command, and lifetime here.
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

## CP10 root policy correction plan

Status: active implementation plan.

Decision entry: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`.

Plan:

1. Remove the `nearest .git parent` ancestor search from `resolveRoot()`.
2. Default root is cwd. `--root` remains explicit override.
3. Keep copied-project usage as the assumption. The project folder is usable as root without requiring `.git` discovery.
4. Add a Go test proving `resolveRoot()` returns cwd even when a `.git` directory exists in a parent path.
5. Keep TN images limited to search-card/result covers. TN must not be used as `#reader` or `#thumbs` viewer display source.
6. Do not add TN viewer display exception, because TN viewer fallback increases rate-limit pressure and conflicts with the user-required actual full image pixels criterion.
7. Viewer display pass remains valid only when actual full image pixels are visible from local full file, persistent full cache, or remote full fetch. `no_full` is not an image-display pass.
8. Rebuild canonical EXE, restart canonical server, and perform actual screen DOM verification for `img.src` present and `naturalWidth > 0` for actual full image display.

### CP10 verification

- `go vet ./...` pass.
- `go test .` pass `21.876s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Code change:
  - `resolveRoot()` now uses explicit `--root` > cwd only.
  - `.git` parent ancestor search was removed.
  - CLI help was updated from `workspace root (default: cwd, or nearest .git parent)` to `workspace root (default: cwd)`.
- Active persistent process after restart:
  - ID: `bgp_196838ff3001dgM7X0U7rMv3jI`
  - URL: `http://127.0.0.1:8731`
  - Command: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root scratch/live`
  - Root: `scratch/live`
- Actual screen URL: `http://127.0.0.1:8731/?x=cp10root#/g/4252697/page/1-2`
- Search-card `#results img` actual TN covers:
  - `img src` present and `naturalWidth > 0`.
  - These remain search-card cover behavior, not viewer display.
- Actual full image display pass:
  - `#reader img`:
    - `src` is objectURL.
    - `naturalWidth` values: `2880`, `1813`.
  - `#thumbs img` for displayed cached full entries:
    - `src` is objectURL.
    - `naturalWidth > 0` for the actual displayed cached entries.
- Status at actual screen:
  - `full display cache 21/203 | no full 0/203 | TN display fallback disabled`
- Actual stale navigation test:
  - Before stale operation: route `#/g/4252697/page/1-2`, note `local`.
  - A stale `loadFullImage()` was started for page 1 with `oldShow=3`, `oldRender=3`.
  - Latest target moved to page 4.
  - After stale async completion, current route became `#/g/4252697/page/4-5`.
  - Current reader images were page 4 and page 5, both `naturalWidth = 1813`.
  - Status did not change from the pre-move status.
  - Note reflected the latest target `cdn/TN（未ダウンロード）`, not the stale old target.
  - Stale async writes did not pollute current route, reader DOM, status, or priority.
- Screenshot: `S:\Documents\Kilo_Commons\hitomi-hub\scratch\hitomi-usage\cp10_root_actual_full_pixels.png`
- Pass judgment for this step: actual full image pixels are displayed when an actual full image is available through the runtime root. `no_full` remains not an image-display pass.

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

- [x] `current-task.tsv` の参照行を削除
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 記録決定 / 元メモ保存（2026-11-01）

- 決定: 重要な情報（元メモ）は消えないように本タスク内へ逐一記録する。この「記録を残す」という決定自体も本タスク内に記録する。

### 元メモ（ユーザー提示 2026-11-01 09:29 UTC）

```text
元のメモを提示します。
重要な情報が決して消えないように逐一記録は残してください。
記録を残すという決定もタスク内に記録してください。
確認を継続

先ほど挙げた **`Hitorus`** や **`hitomi_search_plus`** をはじめ、このジャンルのツールやアプリは基本的にすべてオープンソース（ソースコードがGitHub等で公開されているフリーソフト）です。

その中でも、「本家の検索・閲覧を大きく超える機能（高度な検索、高速化、管理・同期など）を持っている」という点で、**特に機能が優れていて評価されているオープンソースツール**をいくつか厳選して比較・紹介します。

---

### 1. 最も高機能・多機能な検索・閲覧クライアント

#### **`hitomi_search_plus` (GitHub: `h-akatsuki/hitomi_search_plus`)**

* **オープンソース:** はい（Apache License 2.0 / Dart, Rust）
* **対応プラットフォーム:** Android / Windows
* **何が優れているか（高度な機能）:**
* **高度な検索クエリとグループ化:** 本家サイトではやりにくい `((tag:A | tag:B) -tag:C) | tag:D` のような、**括弧を使った複雑なAND/OR/NOT条件の組み合わせ**が可能です。
* **スマートブックマーク（クエリ自体の保存）:** 単に「お気に入りタグ」を登録するだけでなく、**「自分が組んだ複雑な検索条件そのもの」をブックマーク**でき、背景で自動更新させることで新着を追えます。
* **Rustによる爆速サジェスト・検索:** アプリ内に軽量なインデックスとRust製エンジンを内蔵しており、数百万件規模のデータからでもラグなく一瞬で候補や検索結果を出力します。
* **デバイス間データ同期:** 閲覧履歴やお気に入りを別デバイス間で同期する仕組みも備わっています。

### 2. デスクトップでローカル一元管理したい場合

#### **`Hitorus` (GitHub: `kaismic/Hitorus`)**

* **オープンソース:** はい
* **対応プラットフォーム:** Windows / macOS / Linux (デスクトップWebアプリ形式)
* **何が優れているか（高度な機能）:**
* **完全なローカルDB（SQLite）管理:** ダウンロード、お気に入り、既読ステータスがすべてローカルのファイルとして保存されるため、Webサービスと違ってアカウントBANやサーバー閉鎖のリスクがなく、自分の手元で半永久的に管理できます。
* **強力なフィルターとビューアー統合:** ブラウザ感覚でサクサク動くUIを持ちながら、ローカルのファイル群とメタデータをシームレスに紐付けて一括管理できます。

### 3. ダウンロード・保存・整理に特化したツール

#### **`hitomi-downloader` (Python/Tauri版などの派生・関連ツール群)**

* **オープンソース:** はい
* **何が優れているか（高度な機能）:**
* 複数スレッドによる超高速一括ダウンロード。
* ダウンロードした作品を、作者名やカテゴリ別に**自動でフォルダ分け・リネーム**して保存。
* 取得したデータをそのまま **PDF や CBZ（電子書籍フォーマット）に一撃で変換**して出力する機能を持ちます。

### 💡 結論：どれを選ぶべきか？

* 「スマホやPCで、快適な検索・既読・スマートなお気に入り管理をトータルでやりたい」なら、機能面で頭一つ抜けている **`hitomi_search_plus`** が最も洗練されています。
* 「PCの大きな画面で、自分のローカル環境にデータを溜め込み、自分好みに管理・閲覧したい」なら **`Hitorus`** や、定番のダウンローダーと自炊マンガ用リーダー（Komga等）の組み合わせが強力です。
```

### 確認継続（2026-11-01）

- 元メモの `Hitorus` は GitHub で確認済み:
  - Repository: `https://github.com/kaismic/Hitorus`
  - License: GNU General Public License v3.0 (GPL-3.0)
  - Primary language: C#
  - Nature: desktop web app for Hitomi.la search/download/view
  - Main storage: local SQLite + `Galleries` folder
- 元メモの `hitomi_search_plus` は GitHub で確認済み:
  - Repository: `https://github.com/h-akatsuki/hitomi_search_plus`
  - License: Apache License 2.0, README に記載
  - Primary stack: Flutter/Dart with Rust suggestion/search support
  - Key confirmed features from README: advanced grouped search, per-query bookmarks, fast Rust suggestion, device data sync, viewer with caching
- 元メモの `hitomi-downloader` は GitHub 上では代表的に `https://github.com/lanyeeee/hitomi-downloader` を確認:
  - License: MIT license, GitHub repository resources に記載
  - Primary stack: Tauri + Rust + Vue frontend
  - Key confirmed features: GUI, multi-threaded download, export to PDF/CBZ, customizable directory structure
  - Note: Python/Tauri 派生関連ツール群という表現は広義。今回「Tauri版」として確認できたのは `lanyeeee/hitomi-downloader`。Python版特定リポジトリは未確定。
- GitHub repos の image handling 確認:
  - `h-akatsuki/hitomi_search_plus` viewer uses full image bytes from `imageDownloadStatusProvider` and DB `image` table. Display is `Image.memory(data)` only after download/status done. Search/top card uses `smallImageProvider` TN thumbnail separately; TN is not the viewer display fallback.
  - `h-akatsuki/hitomi_search_plus` full image download constructs URL via `GGJS.imgUrl(ImgType.webp)` and sends `referer: https://hitomi.la/...`; search/top card uses `smallSmallThumb` TN webp.
  - `kaismic/Hitorus` viewer displays local downloaded file through `api/image`. Missing local image file returns not-found/HTTP error. It does not use TN as viewer display fallback.
  - `kaismic/Hitorus` download tries image formats `avif` and `webp`, with preferred format first; if all formats 404, it can update live server info and retry; failure is counted when non-404 or repeated failure occurs.
  - `lanyeeee/hitomi-downloader` separates cover TN and full image:
    - Cover uses `url_from_url_from_hash(..., base="tn")`, specifically `webpbigtn`.
    - Full download uses `image_url_from_image(...)` for `webp` or `avif` based on `haswebp`/`hasavif`.
    - Uses `gg.js` dynamic `GG` logic for `b`, hash segment `s`, and subdomain calculation.
    - Sends `referer: https://hitomi.la/` for CDN requests.
- Current project policy remains consistent with these upstream examples on viewer display: TN is cover/search-card behavior, not `#reader`/`#thumbs` viewer fallback. Actual full image pixels require local file, persistent full cache, or successful full image fetch/cache.
- Current unresolved point remains: actual cache-less screen result showed `no_full` for `4296091`, but `4296091` was not confirmed as downloaded in the current runtime root. The active process root mismatch hypothesis remains open and requires runtime root actual screen confirmation.
- This confirmation step did not change code. No implementation change was made from this record/verification pass.

### CP11 cache-less gg cause investigation

- Fresh cache-less server `http://127.0.0.1:8734/api/image/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=avif` returned `404`, `x-cache: no_full`, `naturalWidth=0` behavior via actual screen status.
- Direct live `gg.js` at `2026-11-01 10:23Z` has this live `m()` shape:
  - `var gg={ m: function(g){ var o=0; switch(g){ case ...; o=1; break; } return o; }, s: ..., b:'1793527201/' }`
- For hash `899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9`, `gg.s(hash)=2335`.
- `case 2335` is not present in the live switch at this time, so live `m(2335)=0`.
- Direct request with app UA and `Referer: https://hitomi.la/`:
  - `a1` URL for `2335` returned `200`
  - `w1` URL for `2335` returned `200`
  - `a2` URL for `2335` returned `404`
  - `w2` URL for `2335` returned `404`
- The app builds `a2`/`w2` for this hash because `parseGG()` stores live `case` values as `ZeroSet` and returns `0` only for values inside `ZeroSet`. This is opposite to the live `gg.js` shape observed at this time.
- Recorded known issue: `docs/ai/known-issues/2026-11-01-parsegg-switch-value-inversion.md`.
- Implementation was approved for this cause correction. Implementation scope was limited to `parseGG`, `gen_ggset.py`, tests, canonical EXE rebuild, canonical server restart, and cache-less actual screen verification.

### CP11 implementation verification

- Code changes:
  - `app/gg.go`: `GG.ZeroSet` → `GG.CaseSet`; `M(value)` now returns `1` for live `case` values and `0` for values outside cases.
  - `tools/gen_ggset.py`: generates `ggCaseSet` and `ggM` using the observed live shape `var o = 0; switch (g) { ... o = 1; break; }`.
  - `app/ggset.go` was regenerated from live `gg.js` using the canonical generator.
  - `app/hitomi_test.go`, `app/searchhtml_test.go`, and `app/viewer_test.go` fixture assumptions were updated to the live switch polarity.
  - `newTestViewer()` uses `viewerFixtureGG()` so viewer tests verify the rule by fixture instead of rotating live snapshot values.
- Verification:
  - `go vet ./...` pass.
  - `go test .` pass `22.533s`.
  - `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
  - Canonical server was restarted after rebuild with process ID `bgp_196b07014001jT62QJx8infSQG`, URL `http://127.0.0.1:8731`.
- Cache-less actual screen pass:
  - Fresh cache-less server process `bgp_196b0a6c0001oVuL1sQU6jAF0j`, root `scratch/hitomi-usage-fixed`, port `8735`, stopped after verification.
  - `http://127.0.0.1:8735/?x=cp12fixed#/g/4252697/page/1-2`
  - API check: `/api/image/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=avif` returned `200`, `x-cache: miss`, `content-type: image/avif`.
  - Reader `#reader img`: blob `src`, `naturalWidth=3746`, `naturalHeight=2525`.
  - Status at first actual display: `full display cache 1/203 | no full 0/203 | TN display fallback disabled`.
  - `#thumbs img` displayed actual full objectURLs with `naturalWidth > 0`; no TN viewer display source was used.
  - Screenshot: `scratch/hitomi-usage-fixed/cp12_fixed_cacheless_actual_full_pixels.png`
- Recorded future verification policy: screen verification must be performed from a cache-less runtime root. Cache-backed actual screen passes are not treated as cache-less passes. Decision entry: `docs/ai/decisions/2026-11-01-required-function-verification.md`.
- Post-fix cache-less actual screen pass was reconfirmed from a cache-less root and delivered to the user's Kilo app as `cp13_fixed_cacheless_actual_full_pixels_pass.png`.
  - Server process `bgp_196b7e737001xfLhcaCpEXzbF1` used cache-less root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-verify`, port `8736`, and was stopped after screenshot confirmation.
  - Screenshot path used for delivery: `C:\Users\fujim\AppData\Local\Temp\kilo\cp13_fixed_cacheless_actual_full_pixels_pass.png`.
  - Actual screen URL: `http://127.0.0.1:8736/?x=cp13cachelesspass#/g/4252697/page/1-2`
  - Status: `full display cache 1/203 | no full 0/203 | TN display fallback disabled`
  - Reader: blob `src`, `naturalWidth=3746`, `naturalHeight=2525`.
- Runtime-server note: the canonical server `http://127.0.0.1:8731` was not stopped except for the approved EXE update restart. The cache-less verification server was stopped after screenshot confirmation, then a fresh persistent cache-less verification server was restarted at `http://127.0.0.1:8736` with root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, process ID `bgp_196bb0154001aophdh5vu5ya8s`. This server is to be kept running for future cache-less verification.

### CP14 strip group controls and already-landscape display

- Completed in task `docs/ai/tasks/archive/2026-11-01-strip-group-and-landscape-double.md`.
- Resolved known issues archived:
  - `docs/ai/known-issues/archive/2026-11-01-strip-group-no-user-control.md`
  - `docs/ai/known-issues/archive/2026-11-01-landscape-entry-forced-double.md`

### CP15 stale async status/strip guard

Code changes:
- `updatePrefetchCounts()` returns before status writes when `galleryId !== current.id || backgroundGalleryId !== galleryId`.
- `prefetchPageImage()` returns before `storeFullCache()` / `updatePrefetchCounts()` for stale background targets.
- `storeFullCache()` updates strip DOM only when `galleryId === current.id && backgroundGalleryId === galleryId`.
- `loadFullImage()` returns before status/no_full/error/TN writes when `loadShowGen !== showGeneration || loadOpenGen !== openGeneration || loadRender !== renderGeneration`.
- `viewer_test.go` added `TestEmbeddedUIStaleAsyncWritesDoNotPolluteStatusOrStrip`.

Verification:
- `go vet .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `21.395s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp16.exe .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- Cache-less ongoing server process for this checkpoint: `bgp_1979b7059001gPUXQ2KGDvqhCb`, root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, URL `http://127.0.0.1:8736`.

Actual screen and stale verification:
- Initial actual screen URL: `http://127.0.0.1:8736/?x=cp16-stale-search#/g/4252697/page/1`
  - `readerClass`: `reader single landscape`
  - `readerImageCount`: `1`
  - `#results img`: `25/25` actual pixel display ok
  - `#thumbs img`: `50/50` actual pixel display ok
  - `stripControlsCount`: `4`
  - `stripGroupStatus`: `strip group 1/5`
  - `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`
- Stale `loadFullImage()` was simulated with old show/render while `showGeneration` was advanced. It did not add the stale key to `prefetchedEntries`, did not change status, and did not change counts.
- `updatePrefetchCounts("full", entry, otherGalleryId)` did not change status, counts, or `prefetchedEntries`.
- User UI click `#next` moved to `#/g/4252697/page/2-3` and preserved actual display pass:
  - `readerClass`: `reader double`
  - `readerImageCount`: `2`
  - `readerPages`: `2`, `3`
  - `#reader img`: `2/2`
  - `#results img`: `25/25`
  - `#thumbs img`: `50/50`

Stale known issue `docs/ai/known-issues/2026-11-01-stale-async-status-strip-writes.md` is resolved.

### CP16 independent scroll panels

Implementation:
- `body` constrains the viewport: `height: calc(100vh - 2rem); overflow: hidden; display: flex; flex-direction: column;`.
- `.side` and `.main` fill `.cols` height and scroll independently with `height: 100%; min-height: 0; overflow-y: auto;`.

Verification:
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `21.563s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp18.exe .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- Cache-less ongoing server process: `bgp_197a46b53001g5Ml5Id720ruZ4`, root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, URL `http://127.0.0.1:8736`.
- Actual screen URL: `http://127.0.0.1:8736/?x=cp18-scroll-fixed#/g/4252697/page/1`
  - `bodyOverflow`: `hidden`
  - `sideOverflowY`: `auto`
  - `mainOverflowY`: `auto`
  - `sideScrollable`: `true`
  - `mainScrollable`: `true`
  - `bodyScrollTop`: `0`
  - `sideScrollTop`: `3762`, `mainScrollTop`: `0` after user-driven scrolling, proving side and main scroll independently.
  - Actual display pass remained true: `#reader img` `1/1`, `#results img` `25/25`, `#thumbs img` `50/50`.
  - `readerClass`: `reader single landscape`
  - `stripControlsCount`: `4`
  - `stripGroupStatus`: `strip group 1/5`
  - `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`

Backlog `#10` is completed and removed.

### CP17 query-param new-tab policy

Implementation:
- Added `const newTabMode = new URLSearchParams(window.location.search).get("newtab") === "1";`.
- Added `targetAttr()` and applied it to search-card, saved-search, favorites, unread, and strip links.
- Same-tab `hashchange` routing remains unchanged; `newtab=1` only sets `target="_blank"` on rendered internal links.

Verification:
- `go test .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass, `21.345s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.cp19.exe .` in `S:\Documents\Kilo_Commons\hitomi-hub\app`: pass.
- Cache-less ongoing server process: `bgp_197a83dc2001ZSqyKL15EaAVR9`, root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`, URL `http://127.0.0.1:8736`.

Actual screen pass with `newtab=1`:
- URL before hashchange: `http://127.0.0.1:8736/?newtab=1&x=cp19-newtab-policy#/q/furry/page/1`
- `newTabMode`: `true`
- `targetAttr`: ` target='_blank'`
- `#results a` count: `50`, `targetCount`: `50`, sample target: `_blank`
- `#results img`: `25/25` actual pixel display ok
- After same-tab hashchange to `#/g/4252697/page/1-2`, current route became `#/g/4252697/page/1` because the landscape entry is already-double, `readerClass`: `reader single landscape`
- `#reader img`: `1/1`
- `#thumbs img`: `50/50`
- `stripControlsCount`: `4`
- `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`

Actual screen pass without query param:
- URL: `http://127.0.0.1:8736/?x=cp19-newtab-normal#/q/furry/page/1`
- `newTabMode`: `false`
- `targetAttr`: `""`
- `#results a` count: `50`, `targetCount`: `0`
- `#results img`: `25/25` actual pixel display ok

Backlog `#11` is completed and removed.
