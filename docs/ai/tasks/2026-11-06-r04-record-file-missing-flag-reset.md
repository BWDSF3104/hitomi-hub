# R-04: recordFile が missing_at_verify をリセットせず消失検出が永続的に機能しない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: external_removal 記録後に同 path 再 record で missing_at_verify = 0 へ戻り、verifyMissing が再度検出。記録: R-04

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
