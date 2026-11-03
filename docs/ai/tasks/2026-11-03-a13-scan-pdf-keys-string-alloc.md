# A-13: ScanPDFKeys が key ごとに string(data) を作り直す

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（軽微）。`app/scan.go:164` は key ごとに `string(data)` を生成（MaxScanBytes 既定 2MB × pdfKeys 10 → 最大 20MB 一時的割当）。対応: `bytes.Contains(data, []byte(key))` または key を []byte で保持。合格条件: existing test `ScanPDFKeys` 通過。推定規模: trivial。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
