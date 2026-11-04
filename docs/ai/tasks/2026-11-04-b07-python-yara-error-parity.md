# B-07残件①: Python 参照実装 YARA error 表現を Go 表現へ

**Status**: 進行中
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 委譲（第11実行②・単独 commit・テスト固定）

## 意図 / 背景

B-07（close `bfdc83e`・Go 側）の残件①。Python 参照実装（`src/scan.py` process_file）は YARA error → `scan_error` のまま（Go の `not_evaluated` 維持＋`scan_source`=`structural+yara_error` と対応付が drifted）。推奨承認（`decisions/2026-11-03-remaining-item-recommendations-approved.md`）で Python 側を Go 表現へ揃える。export skip 条件（hit/scan_error）は維持・テスト固定。B-07 残件②（YARA error ファイルが VT gate に入り得る副作用）は decision 2026-11-02 #3 と整合済みで本項目では変えない。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
