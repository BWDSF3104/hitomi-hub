# A-10: download が既存ファイルを確認しない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/main.go:450-460`（`downloadFilesParallel`）は既存ファイル確認なしで fetch 後 `os.WriteFile`。再実行時に gate 分の quota を消費して上書き。対応: `os.Stat` で既存・size 一致なら skip（または `--overwrite` フラグ）。合格条件: 2 回目 scan/download で remote 呼出 0、written は skipped として報告。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
