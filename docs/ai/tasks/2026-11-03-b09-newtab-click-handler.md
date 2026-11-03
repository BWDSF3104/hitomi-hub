# B-09: newtab=1 が実質的に効かない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/ui.html:201` の `newTabMode` は `targetAttr()`（`app/ui.html:250-252`）で anchor に `target='_blank'` を付けるが、href は同一ドキュメント fragment（`#/g/<id>/page/1`）。HTML spec では same-document fragment link の `target` は無視される。実測（`?newtab=1` で card anchor click）: tab 数 3 のまま（新 tab 増えず、同じ tab の hash が `#/g/4300588/page/1-2` へ変化）。DOM には `target="_blank"` 属性が存在。対応: newTabMode では absolute URL（`window.location.pathname + href`）を使うか、click handler で `window.open`。合格条件: `?newtab=1` で card click が新 tab を開く（tab 数 +1）。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
