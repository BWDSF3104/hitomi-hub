# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計確定（memory cap 4000・設定化、memory 削除は disk に同じ内容がある項目優先、pinned は現在閲覧中、LRU は第2基準、cache 層のみ・Galleries と hitomi.db は対象外）。不足の埋め合わせ待ち: disk 上限の単位（byte 推奨）、disk の last access 記録、pinned 定義、TN 層の扱い、削除後の再取得予算、知らせ方の面、client map（prefetchedEntries・countsByGallery）の cap 対象外、guard test（viewer_test.go:816）更新、確認テスト（canonical build・cold hygiene）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

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
