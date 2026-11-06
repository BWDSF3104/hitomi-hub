# R-07: http.Client が default Transport（MaxIdleConnsPerHost 2）で TLS 再接続が繰り返される

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 8 並行 prefetch/download で接続Reuseが確認できる（MaxIdleConnsPerHost を 16 程度へ）。gate semantics・fetch 口（テスト inject）非触。記録: R-07

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
