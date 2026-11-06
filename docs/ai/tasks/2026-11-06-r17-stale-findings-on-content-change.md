# R-17: 同一 path の内容が変わっても旧 sha256 の finding が残る

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 再 scan で現行判定と食い違わない（path 基準で旧 finding を更新/無効化）。D-03 の unique key（path, sha256, rule, detail）は維持し、残存行の扱いを判断。記録: R-17

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
