# C-05 の上限実装は承認範囲 4 cache のみ（残 cache は unbounded のまま）

**Date**: 2026-11-04
**Status**: Open（判断待ち・修復は本項目スコープ外）
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

## 影響

- C-05 commit（本項目分）には上記 cache の変更を含めない。
- full image rules（imageProxy 順序・永続 cache path・TN rules・UI full 判定・API rate policy）は無変更。
