# B-07: YARA 一時失敗で構造 pass が scan_error

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/main.go`（`processFileWithYara`, 513-517 相当）は structural pass の file でも `runYara` が error を返すと `scan_status = scan_error`（`scan_source = structural+yara_error`）へ上書きし、export は `scan_error` を skip。実測（scratch root, `yr` を存在しない binary へ）: `Galleries\105\clean.cbz`（structural `pass`）が `scan_error` / `structural+yara_error` になり、export summary の `pass` が 0。対応: `not_evaluated` を維持し `scan_error` は「判定不能の上位」ではなく別列（または retry 用列）で表現。合格条件: YARA error で `scan_status` が `not_evaluated`、export に `skipped.scan_error` が増えない。推定規模: 小〜中（テスト assertion 更新が必要）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
