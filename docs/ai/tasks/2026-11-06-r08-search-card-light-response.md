# R-08: search card が files 全件 + 件あたり最大 2 回の os.Stat で重い

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。指摘の「files は死荷重」は誤り: local N/M 表示と cover fallback（search.html:408,414）と guard test（TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen）が files を使用。ReadDir 1 回 + 軽量応答の設計承認が必要。記録: R-08

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
