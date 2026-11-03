# B-02: objectURL 2000 件上限で 2001 個目が壊れる

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（実装・テスト・実機確認済み）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`MAX_FULL_CACHE_ENTRIES = 2000`（`app/ui.html`）。`storeFullCache`（`app/ui.html:768-785`）は `fullObjectUrls` 追加後に size > MAX で `revokeFullObjectUrls()` を呼び、active を含む全 URL を revoke する。実測（fresh tab, `#reader` に真の cached full を objectURL で表示 → `fullObjectUrls` を 2000 へ填充 → 2001 件目を `storeFullCache`）: size 2000 → store 後 size 0、`activeObjectUrlKeys` 0、`fullCache.get(key).objectUrl` null、`displayViewerSource` ""、img `naturalWidth` 0（表示が空白化）。`prefetchedEntries` は statuses を保持したまま（count は objectURL cache 0/8 だが entry は残る）。スクショ: `docs/ai/tmp/assess-20261103-1220/03-b02-objecturl-revoke-all.png`（Drive へコピー済み）。対応: per-entry LRU eviction（revoke するのは最も古い key だけ）+ `prefetchedEntries`/`activeObjectUrlKeys` の同期削除。合格条件: 上限超過時に表示中の entry の objectURL が有効なまま、LRU 外の entry だけ revoke。推定規模: 中。

実装: `app/ui.html` に `fullObjectUrlLru`（`key -> objectUrl` の Map insertion order）を追加し、`storeFullCache` が objectURL を追加時に LRU へ `delete(key)` 後に `set(key, objectUrl)` して newest へ更新。上限超過時は `evictFullObjectUrls(skipKey)` が `currentDisplayObjectUrlKeys` / `currentSearchObjectUrlKeys` / `currentGalleryObjectUrlKeys` で保護される key を skip し、LRU 内の最も古い unprotected key だけ `revokeFullObjectUrl(key)` で revoke する。`revokeFullObjectUrl` は `URL.revokeObjectURL(objectUrl)` 後、`fullObjectUrls`、`fullObjectUrlLru`、`activeObjectUrlKeys`、`sourceStatusByEntry`、`sourceTypeByEntry`、`prefetchedEntries` から該当 key を削除し、`fullCache` entry の `objectUrl` を null にする。`renderReader` / `renderStrip` / `renderSearchCard` / `loadSearchCardCover` / `recordImageDisplayPassFromImg` で current display/search/gallery objectURL key を保護集合へ登録。`storeFullCache` の all-revoke call は削除し、`revokeFullObjectUrls()` は legacy clear 関数としてのみ残す。

## Verification

- `go vet .`（`app`）: clean.
- `go test -count=1 -timeout 300s .`（`app`）: `ok hitomi-hub/app 36.656s`. New static test `TestEmbeddedUIObjectUrlLruEvictsOnlyOldestUnprotected` passes; updated objectURL tests no longer require all-revoke in `storeFullCache`.
- Canonical build from `app`: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded.
- `python -m pytest tests/ -v`: 25 passed, including `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui`.
- CLI download from canonical exe: `.\hub.exe download --id 4300588 --limit 1` returned `attempted=1`, `written=1`.
- Canonical server: stopped old server, rebuilt, started `bgp_1a03e5684001ov2tTmZ25jIlGp` PID `4768`, `http://127.0.0.1:8731/`.
- Actual UI/network confirmation (Playwright, viewport `1280x900`, fresh tab, initial display before actual click):
  - Before actual click, initial served UI state: `perf: /api/search 920ms・server search=230;record=685;json=0・cards 25 106934ms・cache miss=25`, first card `4300644`, first cover `naturalWidth=640`, `currentSearchObjectUrlKeys.size=25`.
  - Actual UI action: actual click on first search card cover link, route `#/g/4300644/page/1`.
  - Actual reader display after click: `actual pixels 31/57 | local direct 0/57 | objectURL cache 31/57 | no full 0/57 | TN display fallback disabled`; first displayed `#reader img[data-viewer-display='1']` key `4300644|a178e5238ef3480ec3f6677e264978924dc089d6391958a67bd8fc12a392e3fb|avif`, `naturalWidth=1200`, `currentDisplayObjectUrlKeys` protected current key.
  - Served UI network for current objectURL source: `GET /api/image/a178e5238ef3480ec3f6677e264978924dc089d6391958a67bd8fc12a392e3fb?format=avif&gallery=4300644 => 200 OK` (request #48).
  - LRU actual UI test on served UI: fill `fullObjectUrls` to exactly `MAX_FULL_CACHE_ENTRIES = 2000` with unprotected fake keys, then `storeFullCache({hash:"protectedclean", hasavif:false}, "cache", "blob:protectedclean", "99999995")` for protected key `99999995|protectedclean|webp`.
  - Result: protected current display key remained valid (`currentKeyLruAfter=true`, `currentKeyActiveAfter=true`, `currentKeyFullCacheObjectUrl=blob:http://127.0.0.1:8731/96cafb0e-3dc9-4d41-9a87-e61483de280b`, `naturalWidth=1200`); protected new key remained (`protectedKeyLruAfter=true`, `protectedKeyActiveAfter=true`, `protectedKeyFullCacheObjectUrl=blob:protectedclean`); the oldest unprotected LRU key `99999996|fake0|avif` was evicted (`firstFakeEvicted=true`, `firstFakePrefetchedAfter=false`, `firstFakeFullCacheObjectUrl=null`, `firstFakeActiveAfter=false`).
  - `prefetchedEntries` for evicted key was removed, satisfying the requirement that LRU eviction removes `prefetchedEntries`/`activeObjectUrlKeys` in sync.
- Visual confirmation screenshot: `docs/ai/tmp/assess-20261103-1220/b02-objecturl-lru-current-display.png`, copied to `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\b02-objecturl-lru-current-display.png`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
