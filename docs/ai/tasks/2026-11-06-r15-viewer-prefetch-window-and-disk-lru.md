# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計確定（memory 参照 4000・または設定化、memory 削除は disk に同じ内容がある項目を優先、LRU は第2基準、閲覧中と列表示済みは捨てない）。LRU が効く範囲は cache 層のみ（Galleries と hitomi.db は削除対象外）。実施開始指示待ち。残設計点: disk 側の容量上限値、上限到達時の通知、設定名、guard test（viewer_test.go:816）の 2000 固定の更新。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
