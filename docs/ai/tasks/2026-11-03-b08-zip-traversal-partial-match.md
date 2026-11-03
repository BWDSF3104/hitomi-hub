# B-08: zip の .. 判定が部分一致で誤検知

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/scan.go:132`（`containsTraversal(name)`）と `rules/structural.yar` の `contains ".."` は `..` を部分一致で hit する。実測: zip entry 名 `Vol...01.webp`（正当）で finding `path_traversal:Vol...01.webp`、file status `hit`（hit は export から除外される）。対応: path separator を跨ぐ `..` セグメントのみ判定（例 `strings.HasPrefix`/segment 解析）、YARA rule も segment 化。合格条件: `Vol...01.webp` で hit しない、`../evil.exe` で hit する（既存テストは `../` のみ）。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
