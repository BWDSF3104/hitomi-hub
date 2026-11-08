# N-13: 未読一覧が無制限のまま（前回の指摘が未解決）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: 未読一覧が無制限のまま（前回の指摘が未解決）。`listRows("unread")` は `fully_read = 0` の全行を返す。開いただけの gallery（`viewed`）も入るため増え続ける。検索と手動更新のたびに全件を取得して描画。LIMIT か `viewed` だけの行の除外が指摘の対処。前批 R-18（`docs/ai/tasks/2026-11-06-r18-unread-list-limit.md`）で対応不要（現状維持）と判定済み。page 化・上限は一覧の意味（全件 vs window）と CLI `hub list` への波及のため承認案件。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-13 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
