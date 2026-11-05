# 検索表示高速化（推奨案: data100ms+worker8+eager cover）の実装

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: commit implementation and docs updates; Memory Bank close

## 意図 / 背景

ユーザーが「もっと高速化が必要」と指示。既存項目3では data `250ms` / image `125ms` / reader prefetch `8` / CLI download `8` に変更済みだが、search page の card worker と prefetch は変更していなかったため、検索サムネイル表示の体感速度が改善しない。ユーザーが推奨案を選択: data interval `100ms`, search card worker `8`, search prefetch `4`, first visible cover eager/high priority, image `125ms`維持。block時の挙動は追加検証保留。

## 調査結果 / 変更内容

- 実装前 decision: `docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md`
- `app/hitomi.go` `defaultHitomiConfig()` data lane: `250ms -> 100ms`。image lane は `125ms`維持。
- `config.example.json` data key `min_interval_ms`: `250 -> 100`。
- `app/search.html`:
  - `SEARCH_CARD_CONCURRENCY`: `4 -> 8`
  - `PREFETCH_CONCURRENCY`: `2 -> 4`
  - first visible search card cover に `fetchpriority="high"` / `loading="eager"`。
- tests re-anchor: `main_test.go`, `hitomi_test.go`, `test_search_prefetch.py`, `test_search_visible_priority.py`, relevant harness checks。
- actual served UI first-pass exposed an async visibility race: `IntersectionObserver` can populate `visibleIds` after worker processing starts, especially on saved-state restore. `processCard` was re-anchored to use `distanceToVisibleArea(id, visibleIds, results, visibleRoot) === 0` as the initial display-area check for first visible eager covers.

## Verification

- `go vet . && go test -count=1 -timeout 300s .` in `app`: passed, `ok hitomi-hub/app 24.356s` after re-anchor.
- `python -m pytest tests/ -v` in project root: collected 53 items, `53 passed in 9.83s`.
- canonical build from `app`: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`: passed. `tests/test_canonical_build_freshness.py` passed after rebuild.
- canonical served UI confirmation used canonical server from the canonical exe: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`.
  - actual window size/content viewport: inner `1920x897`, `devicePixelRatio: 2`
  - Playwright screenshot: `docs/ai/tmp/2026-11-05-search-speedup-actual-window.png`
  - served UI `GET /search` loaded; `search-status`: `検索完了・カード表示済み 9件`
  - network response for served search: `GET http://127.0.0.1:8731/api/search?query=female%3Ayuri%20language%3Apolish&lang=all&page=3&limit=25` -> `200`, `cache-control: no-store`, `x-search-timing: search=140;record=29;json=1`
  - first actual visible card high/eager: first six cards in the actual viewport had `loading="eager"` and `fetchPriority="high"`; `FIRST_VISIBLE_EAGER_COVER_COUNT` is a cap of `8`, and the actual window displayed six first cards.
  - first visible cover network response: `GET /api/thumb/78a2c688096c34e56680ec2eee30b4e3d20a15f054c8165312cae8e0374a8bfb?format=webp&gallery=296403&variant=bigtn` -> `200`, `x-cache: tn-cache`, `content-length: 34818`
- docs numeric mirror pointers updated to the new decision for current data and search page values.
- block behavior remains intentionally pending: block signal policy and cooldown values were not changed in this implementation, and live block generation was not tested.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
