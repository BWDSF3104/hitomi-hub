# R-16: TN 書き込みと download 書き込みが非 atomic（full cache の temp+rename+magic と非対称）

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。full cache の writeImageCacheAtomically（A-07a）を共通化し、TN と downloadFilesParallel（main.go:737）へ適用。合格条件: write/close/rename 失敗で壊れた target が残らない。記録: R-16

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
