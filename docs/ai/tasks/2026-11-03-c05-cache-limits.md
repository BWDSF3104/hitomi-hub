# C-05/A-14: cache に上限がない（対象 cache 全体）

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`searchCache`（`app/searchhtml.go:79-100`）は key ごとに ids 全体（例 `furry` = total 32892 → ids 配列）を保持し、`infoCache`/index caches も TTL だけで上限・eviction がない。実測: `furry` の search response に 25 ids だが cache は total 分の集合。対応: entry 数上限（LRU）または TTL だけでなく size cap。合格条件: 上限超過時に古い entry が消え、memory が一定以下。推定規模: 中。C-05 と同一項目（記録は A-14）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
