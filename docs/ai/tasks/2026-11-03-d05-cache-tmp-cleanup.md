# D-05: cache 書き込みの temp 掃除がない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

解消済み。`app/viewer.go:325-365`（`writeCacheFileOnce`）は `cleanupTemp()` を write/close/rename の各 failure path で呼ぶ。テスト `assertNoCacheTempResidue`（`app/viewer_test.go:2457`）で success path も確認済み。現行代码に defect はない。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
