# C-05 の上限実装は承認範囲 4 cache のみ（残 cache は unbounded のまま）

**Date**: 2026-11-04
**Status**: 観察（実測済み・2026-11-04）。判断反映（ユーザー承認 2026-11-04・`decisions/2026-11-04-recommendations-approved-execution.md`・`decisions/2026-11-04-observation-records-closure-approved.md`・`decisions/2026-11-04-remaining-residual-approval-split.md`）: 実測で有意な成長確認済み（下記）。`indexTopCache` の bounded 化は実施済み（単独 commit）。その他の残項目は実施承認として記録（canonical DB operational 確認・B-06 optional live baseline）。`noFullCache` は stale-only bounded memory limit 実装済み（cap 1024 entries・active entry protected・単独 commit・`docs/ai/tasks/2026-11-04-no-full-stale-only-bounded.md`）: cap 超過 insert 時に TTL 超過済み stale entry だけを掃除、TTL 内 active entry は protected（eviction しない）。別意味変更（hard cap で active entry まで eviction する方式＝C-08 意味変更）は未承認。`versions`/`versionAt`/`versionForceAt` 非触。
**Backlog**: assess-20261103-1220

## 現象

- C-05/A-14 の実装は承認範囲の 4 cache（`searchCache`/`indexDataCache`/`indexNodeCache`/`infoCache`）に LRU entry 数上限 + size cap を追加した。
- 同系統の以下の cache は TTL だけで上限・eviction がない（承認範囲外・未変更）。
  - `indexTopCache`（`app/hitomi.go`・key 例 `galleries|<version>`）: version rotation（約 1 時間窓）で key が増え、entry が TTL 超過分しか消えない。長時間稼働で keys 集合が grow する。
  - `noFullCache`（C-08 negative cache・TTL 5 分）: TTL 判定は hit 時に key を delete するが、二度と access されない stale key は delete されないまま残る。上限量なし。
  - `versions`/`versionAt`/`versionForceAt`: field 数ぶんのみ（field が固定なら bounded だが、cache として上限宣言はない）。

## 判定

- 本項目の合格条件（対象 4 cache の上限・eviction・memory bound）はテストで固定済み（`docs/ai/tasks/2026-11-03-c05-cache-limits.md`）。上記は「非必須問題」として記録のみ（global AGENTS.md 条: 修復に枝打たず known-issues へ記録）。
- 修復する場合、entry 数上限（`boundedCachePut` の既存 helper 適用）が自然。ただし `noFullCache` は C-08 の negative cache 規則（TTL・singleflight・再試行 1 回）に接触するため、変更は別項目・別承認を要る。

## 実測（mock・外部 API 呼出しなし・server 起動なし・2026-11-04）

scratch: `docs/ai/tmp/assess-20261103-1220/c05measure_test.go`（`go test -overlay` 注入）。

- `indexTopCache`: N version rotation → keys 集合が N に growth（120 → 120・200 → 200・300 → 300）。eviction 0・上限量なし。TTL 超過模拟後、stale key の access は refetch・同一 key 再insert のみで keys 集合は shrink しない（len 300 維持）。有意な成長を確認。
- `noFullCache`: K=2000 stale key insert → access 10 件のみ delete・残留 1990。window rotation（bucket 変更）で old-window stale key は二度と access されず残留（1991）。上限量なし。

反映（実施済み・単独 commit・commit 参照＝本項目 commit・hash は board RESULT 報告）: 上記の実測で有意な成長を確認し、親判断（承認 2026-11-04）により `indexTopCache` の bounded 化を実施。cap = 構造同一 cache（`indexNodeCache`: bnode entry・entrySize = `maxNodeSize`・topCacheTTL）の既存 convention 値 1024 entries / 1 MB。`hitomi.go` へ `entrySize()`/`lastAccessed()`・`putIndexTopCacheLocked`・既定 cap 追加、`searchindex.go` の `indexTopNodeCached`（insert 経由化＋hit 時 lastAccess 更新の承認された 2 箇所のみ）。テスト固定 `TestIndexTopCacheEvictsLRUEntriesWhenOverEntryLimit`。変更後実測 N=1200 → len 1024（cap）・bytes 475,136 で bounded 確認。`noFullCache` は C-08 規則接触のため非触・別承認条件のまま維持（stale 残留 1990 の記録は上）。`versions`/`versionAt`/`versionForceAt` 非触。

## 影響

- C-05 commit（本項目分）には上記 cache の変更を含めない。
- full image rules（imageProxy 順序・永続 cache path・TN rules・UI full 判定・API rate policy）は無変更。
