# N-09: info disk cache の prune が上限到達時に全 JSON を走査

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: info の disk cache は上限に達すると新規書き込みのたびに全 JSON を走査する（`pruneInfoDiskLocked`）。上限まで刈ると次の 1 件で再び超過し、最大 5000 件を読んで parse する。`infoDiskMu` を保持したままなので `readInfoDisk` も止まる。10% 程度多めに刈るか、parse せず mtime で刈る案。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-09 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
