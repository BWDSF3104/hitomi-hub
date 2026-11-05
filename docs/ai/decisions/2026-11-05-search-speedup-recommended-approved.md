# 2026-11-05 — 検索表示高速化（推奨案）の実装承認

Superseded by（部分・2026-11-06）: `docs/ai/decisions/2026-11-06-tn-image-lane-60ms-approved.md`（本 entry の「image lane 125 ms を維持」の点のみ。data lane 100 ms・search page 定数・block 系は有効）

**実行固有ID**: `assess-20261105-1210`
**Status**: 承認済み（ユーザー 2026-11-05 12:09 UTC。質問選択で「推奨案: data100ms + worker8」）

## 決定（本実行の正典値）

- data lane `min_interval_ms`: 250 ms → **100 ms**
- image lane `image_min_interval_ms`: **125 ms を維持**
- `app/search.html` の search card worker concurrency: 4 → **8**
- `app/search.html` の background prefetch concurrency: 2 → **4**
- 最初の可視 search card cover は `fetchpriority="high"` / `loading="eager"` を付与。可視 cover の eager priority は最大 **8件**。
- block signal、5分クールダウン、`Retry-After`、politeness退避、TN display policy、full image policyは変更しない。
- 正典単一出力元 `app/hitomi.go` の `defaultHitomiConfig()` の data lane 既定値を 100 msへ更新。`image_min_interval_ms`、`gg_force_min_interval_ms`、`bg_grace_ms`は変更しない。

## 根拠

- 2026-11-05実測で data 100 ms・6並行、100 ms・12並行、50 ms、25 msで 429・rate関連ヘッダ・`cf-mitigated` が観測されなかった。100 msは既存実測範囲内。
- 検索体感が改善しない主因は、search page側の worker定数が変更されていなかったこと。`app/search.html:116` の `SEARCH_CARD_CONCURRENCY = 4`、`app/search.html:572` の `PREFETCH_CONCURRENCY = 2` は、reader側 `PREFETCH_CONCURRENCY = 8` とは別。
- `/api/search` の initial remote latencyは単発 remote latencyが支配。worker上昇と可視 card priority、eager cover fetch priorityによって queue overlapと最初の表示を改善する。

## supersede する決定

- `docs/ai/decisions/2026-11-05-rate-values-and-concurrency-change-approved.md` の以下のみ supersede:
  - data lane `min_interval_ms` を 250 msとする点
  - prefetch並行度を 8とする点のうち search page prefetch 部分（search page prefetch は 4とする）
- 同 decision の以下は維持:
  - image lane 125 ms
  - CLI download parallel 8
  - block signal・5分クールダウン・`Retry-After`
  - `gg_force_min_interval_ms`・`bg_grace_ms`

## 未確認点

- block時の挙動はユーザー指示によりこの高速化実装では追加検証しない。block発生時動作の実機確認は引き続き未確認。
- 100 ms未満、持続呼出し、累計規模での block挙動は未測。
