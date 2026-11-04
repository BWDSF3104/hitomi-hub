# flaky LRU tie: テスト側決定化（tie 排除）

**Status**: 進行中
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 委譲（第11実行④・単独 commit・単独実行+suite確認）

## 意図 / 背景

既知 issue（`docs/ai/known-issues/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`）の修復。`TestSearchCacheSizeCapBoundsMemory`（C-05 `52ffde2`）は suite 全体で 1 回 FAIL し得る（LRU tie）。推奨承認（`decisions/2026-11-04-recommendations-approved-execution.md`）でテスト側決定化（tie を排除・insertion order/sizes を確定）。production LRU 意味を変えない（C-05 close 済み・本挙動非触）。単独実行＋suite 全体での再現確認。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
