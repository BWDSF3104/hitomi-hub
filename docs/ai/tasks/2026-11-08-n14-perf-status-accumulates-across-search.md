# N-14: perfStatus が検索をまたいで累積し数値が食い違う

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `perfStatus` が検索をまたいで累積している。`cardsLoaded` と `cardTotalMs` は検索ごとにリセットされない。「計測」行の `searchMs` は今回分なのにカード数と時間は累計で、数値が食い違う。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-14 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
