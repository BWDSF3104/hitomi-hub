# A-02/C-03: recordDiscovered が 1 件ずつ commit

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/main.go:991` の `recordDiscovered` は `dbExec` に対し 1 Exec（autocommit）で、search handler は ids を 1 件ずつ呼ぶ。canonical server（2026-11-03 03:36）`X-Search-Timing: search=8477;record=935;json=0`（25 ids → 約 37ms/件）。`limit=1` では `record=195`。対応: 1 request（または 1 saved query）を 1 tx にまとめ、external call を tx 外で完了してから書き込む。合格条件: mocked fetch/DB で commit 回数が request あたり 1。推定規模: 小。C-03 と統合済み。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
