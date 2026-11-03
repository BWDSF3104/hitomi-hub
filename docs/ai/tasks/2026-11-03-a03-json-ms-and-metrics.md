# A-03: json_ms が常に 0 と metrics の二重返却

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。実測 `X-Search-Timing: search=8477;record=935;json=0` で content-length 1061（JSON 生成に時間がかかっているのに `json_ms` 0）。レスポンスには top-level `metrics` と `timing.metrics` に同じ内容が二重で出る（`docs/ai/tmp/assess-20261103-1220/baseline_search.json`）。UI の `#perf-status` は `server timing` 表示でこの値を使う。対応: encode 後（または Encoder 使用）で json_ms を測り、二重返却を解消。合格条件: `json_ms > 0` が観測され、重複が片方だけになる。既存テストの assertion 変更は実装段階。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
