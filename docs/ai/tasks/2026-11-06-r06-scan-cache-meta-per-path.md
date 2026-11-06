# R-06: scan_cache の size/mtime が sha256 1 組のみで同一内容の別 path の skip が効かない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 同一 sha256 の 2 path（Galleries と acquisition）を 1 scan 後 2 scan で双方 hash 計算 0。key に path を含めるか path 別 meta 表。記録: R-06（A-12 の残存点）

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
