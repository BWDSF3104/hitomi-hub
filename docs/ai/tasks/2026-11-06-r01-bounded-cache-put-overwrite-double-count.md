# R-01: boundedCachePut が上書き時に古い entrySize を引かない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 同一 key の再 insert で totalBytes が水増ししない（上書き時は old entrySize を引いてから加算）。記録: 2026-11-06-claude-review-20261106-findings.md R-01

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
