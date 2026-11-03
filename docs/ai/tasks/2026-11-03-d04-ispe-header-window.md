# D-04: IsPE が先頭 64 バイトしか見ない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`src/scan.py` の `is_pe`/`scan_file` は先頭 64 bytes しか読まない（`readHead(path, 64)`）。`e_lfanew = 0x80` の MZ ファイル（PE signature を 0x80 に置く）で `mz_without_pe_header` が hit（`Galleries\\102\\b.bin`）。対応: `e_lfanew` を readHead（4 bytes）で读取し、signature offset まで seek。合格条件: `e_lfanew >= 64` の fixture（scratch script）で `mz_without_pe_header` が出ない。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
