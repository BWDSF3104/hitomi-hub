# A-05/C-01: 検索カードの逐次取得

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了

## 意図 / 背景

- 検索カード取得が `loadSearchCards` で ids を 1 件ずつ `await api("/api/search-card/<id>")` する逐次経路。
- サーバの `MinIntervalMS = 1000ms` gate を消費し、cache hit であっても UI 側の逐次 `await` が体感遅延を生む。
- A-05/C-01/C-02 は同一項目。記録は A-05/C-01 に置き、C-02 へ相互参照する。

## 調査結果 / 変更内容

実装:
- `app/ui.html` に `const SEARCH_CARD_CONCURRENCY = 4;` を追加した。
- `loadSearchCards` を bounded worker pool へ変更した。worker は `searchGeneration` guard を維持し、`const workers = [];`、`const workerCount = Math.min(SEARCH_CARD_CONCURRENCY, ids.length);`、`await Promise.all(workers);` で最大 4 worker を回す。
- card fetch は full-id sequential `for (let i = 0; i < ids.length; i++) {` ではなく、queue から bounded worker が処理する。
- DOM selector と status update は worker 内で live DOM を参照する。stale search の async card write は `searchGen !== searchGeneration` で停止する。
- `const cardStart = performance.now();`、`perfStatus.cardTotalMs += performance.now() - cardStart;`、`recordPerfCache(card._x_cache);`、`renderPerfStatus();` を per-card API request ごとに維持した。
- `loadSearchCardCover` 内の `await currentPriorityDone;` を維持した。card fetch 直列化を除去し、cover load の current priority wait は C-02 の対象として分離した。
- server `MinIntervalMS = 1000ms` gate、full image guard、TN display source、objectURL actual pixels、`no_full`、`no_tn`、cache write singleflight は変更しなかった。

## Verification

- `go vet .` in `app`: clean。
- `go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 36.599s`。
- Canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded。
- `python -m pytest tests/ -v`: 25 passed, including `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui`。
- CLI download check from canonical server build path: `.\hub.exe download --id 4300588 --limit 1` returned `attempted=1`, `written=1`。
- Static UI test additions:
  - `TestEmbeddedUISearchCardLoadsCardsWithBoundedParallelWorkers` requires `SEARCH_CARD_CONCURRENCY = 4`, bounded worker, no full-id sequential fetch, per-card timing, `await loadSearchCardCover(card, searchGen);`, current-priority guarded cover load.
  - Existing selector test updated to bounded-worker `id` selector.
- Canonical UI confirmation from rebuilt canonical server, `http://127.0.0.1:8731/`, fresh tab, viewport `1280x900`:
  - Initial fresh tab state before actual search click: `perf: /api/search 1810ms・...・cards 1 38ms・cache cache=1`; status `カード読み込み 1/25: 4299791`。
  - Actual search click: query `female:yuri`, lang `english`, page `1`, clicked `検索` button。
  - First actual search completed 25 cards; UI status `検索完了・カード表示済み 25件`。TN cover source for first card `4300644` was `/api/thumb/a178e5238ef3480ec3f6677e264978924dc089d6391958a67bd8fc12a392e3fb?format=webp&gallery=4300644&variant=bigtn`, TN network request `#54` returned `200`, `x-cache: tn-cache`, and `statusIsViewerDisplaySource("tn")` was `false`。
  - Second actual click measured bounded parallel behavior after cache was populated: initial `cards 50 190198ms`, processed this click `cards 25 1340ms`; `perf-status` after second actual click: `cards 75 191539ms・cache cache=26 miss=49`; status `検索完了・カード表示済み 25件`。First card DOM `data-search-id='4300644'` matched network request `#90` (`x-cache: cache`, `129ms`)。
  - Screenshot: `docs/ai/tmp/assess-20261103-1220/a05-parallel-search-cards.png`
  - Google Drive copy: `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\a05-parallel-search-cards.png`

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
