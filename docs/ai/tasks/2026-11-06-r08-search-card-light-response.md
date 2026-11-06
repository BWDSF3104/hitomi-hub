# R-08: search card が files 全件 + 件あたり最大 2 回の os.Stat で重い

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 A: 1 gallery 1 回の集合判定、保存済み枚数と代表画像の材料は残す）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
