# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 B: 全体先読み維持 + gallery 単位の保存容量上限、閲覧中と列表示済みは捨てない・利便性優先）。実施開始指示待ち。残設計点: 上限値、上限到達時の通知。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
