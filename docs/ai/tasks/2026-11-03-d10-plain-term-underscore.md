# D-10: searchHTML plain term のアンダースコア

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

機構は事実（`normalizeSearchTerm` が `_` → space、`hashTerm` は `sha256(lower(term))[:4]`、`app/searchindex.go:27-30`）。実測: `big_breasts`（1 token）→ total 1、`big breasts`（2 token）→ total 473481（tokenization が違うため直接比較不可）。defect として再現できず、hitomi の index key が space 型か underscore 型かの正典確認が必要。判定: hitomi 準拠確認待ち。対応: 確認後に normalize 方針を固定。推定規模: 小（確認込み）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
