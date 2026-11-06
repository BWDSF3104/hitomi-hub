# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計確定（memory 4000 件・設定化、disk 8000 件ずつ、disk 削除は作成順、memory 削除は保存済み status 優先→LRU、現在閲覧中・表示中は捨ない、通知 UI なし、last access 記録なし、sha256 lookup なし）。件数と作成順と status は DB の 1 表（例 cache_entries: hash、format、kind、created_at、source_status）で持つ。テスト viewer_test.go:816 の 2000 固定更新。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
