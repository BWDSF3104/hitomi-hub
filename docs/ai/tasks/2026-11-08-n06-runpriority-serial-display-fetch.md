# N-06: runPriority が表示中の 2 枚を直列取得している

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `runPriority` が表示中の画像を直列に取得している。見開きの 2 枚を 1 枚ずつ `await` する。image lane の間隔は 60ms なので並列にしても支障はない。cold 時の待ちが約 2 倍。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-06 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
