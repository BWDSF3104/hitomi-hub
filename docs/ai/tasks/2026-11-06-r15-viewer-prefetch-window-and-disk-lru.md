# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 方針判断待ち（strip 先読みは TN 禁止方針の帰結）。案: 先読みを現在ページ ±N へ、disk cache に容量 LRU。client 側 objectURL LRU（B-02）は既存。記録: R-15

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
