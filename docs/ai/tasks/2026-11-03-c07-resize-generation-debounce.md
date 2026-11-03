# C-07: resize と fullscreen の generation

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/ui.html:1508-1514` の resize handler は event ごとに `renderReader()` + `startFullPrefetch()` を呼ぶ（debounce なし）。`renderGeneration`/`showGeneration`/`openGeneration` guard は存在し、weak にしないこと（正典）。対応: resize に小 debounce（例 150ms）を追加し、guard は維持。合格条件: 連続 resize で render 回数が 1、objectURL display path が保われる。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
