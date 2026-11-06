# R-12: 起動時に restoreSearchState と runSearch が二重に走りカード取得が捨てられる

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: ページロードで card fetch が 1 回だけ（searchGeneration が restore 分を進めない）。cold 確認の汚染源としても記録。記録: R-12

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
