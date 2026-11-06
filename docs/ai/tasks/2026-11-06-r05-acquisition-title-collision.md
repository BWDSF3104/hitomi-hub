# R-05: acquisition フォルダが title だけで決まり同名別言語 gallery で上書き衝突

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 A: 作品名 + 作品番号、予約名・末尾点・末尾空白・長さ上限を処理、既存フォルダは再命名せず新規保存分から）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
