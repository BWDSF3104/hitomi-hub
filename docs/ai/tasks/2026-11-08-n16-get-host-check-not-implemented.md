# N-16: GET の /api/* と /files/* に Host 検査がない（R-20 承認済み・未実装）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: GET の `/api/*` と `/files/*` に Host 検査がない（前批 R-20 は案 A 承認済み・未実装。`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）。外部にポートを公開していなくても DNS rebinding は成立し、同じブラウザで開いた悪意あるページが名前解決を 127.0.0.1 へ切り替えて読み取れる。既存の `isLoopbackHost` を GET にも適用すれば塞げる（LAN 運用時は configured private-network/host allowlist への更新が必要）。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-16 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
