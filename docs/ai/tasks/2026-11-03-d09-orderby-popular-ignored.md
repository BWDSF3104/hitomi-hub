# D-09: orderby:popular が解析されるが無視される

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/searchhtml.go`（`parseTerms`）は `orderby` を `state.orderby`（`popular` 等）へ入れるが、`applySearchOrder`（178-191）は `state.orderbydir`（asc/desc）しか適用しない。`orderby:popular` は集合順序を変えない。hitomi 準拠（results.js の sort）は未確認 → 判定は hitomi 準拠確認後に確定。対応: hitomi 準拠確認後に sort を実装 or parse から除去。推定規模: 小〜中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
