# C-09: /api/read と /api/lists の debounce 未実装

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/ui.html:1065-1076`（`runBackgroundReadAndLists`）は showPage ごとに `/api/read` + `/api/lists` を呼び、`openGallery`（`app/ui.html:1154,1165,1180`）と search（`app/ui.html:1416`）も `showLists()` を呼ぶ。debounce なし。対応: 同一 gallery/page の重複 post skip、list 表示の debounce（例 250ms）。合格条件: 連続 page 移動で post/list 回数が 1（mocked fetch で確認）。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
