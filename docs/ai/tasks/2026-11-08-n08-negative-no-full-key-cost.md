# N-08: negativeNoFullKey が呼出ごとに case 値をソート・連結（约 12KB）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `negativeNoFullKey` が呼ぶたびに約 2000 件の case 値をソートして連結している。image と TN の各リクエストで `canUse`、`imageFetchKey`、`clear`、`remember` と複数回走る。key は約 12KB の文字列になり、`noFullCache` が最大 1024 件で約 12MB を保持する。GG の parse 時に fingerprint（ハッシュ）を一度だけ計算して使えば解消する。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-08 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
