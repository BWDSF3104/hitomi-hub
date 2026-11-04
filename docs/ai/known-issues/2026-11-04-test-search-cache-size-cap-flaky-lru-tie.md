# `TestSearchCacheSizeCapBoundsMemory` が suite 全体実行で flaky

**Status**: 未解決（低優先・修復は実施段階）
**Date**: 2026-11-04

## 内容

A-09 実施（2026-11-04）の suite 全体実行で `TestSearchCacheSizeCapBoundsMemory`（C-05・close 済み）が 1 回失敗。単独実行は通過。同一タイムスタンプの LRU tie により淘汰順序が非決定になる疑似（A-09 の変更は cache write path・LRU 未触）。

- 実測: suite 全体 1 回 FAIL → 単独実行 PASS（A-09 子報告。記録は A-09 タスク Verification）
- 合格済み項目 C-05 の固定テスト。再現条件（同一タイムスタンプ複数 insert）が時刻分解能に依存。

## 次の対応（実施時）

- tie-break をテスト側で決定化する（insert 順 priority 明示・clock inject）か、C-05 の合格条件の範囲でテストを強化。実装は実施段階。本項目の判定は C-05 の単独実行合格で維持（非必須問題として記録しタスクを進める）。
