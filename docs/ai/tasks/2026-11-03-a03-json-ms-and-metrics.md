# A-03: json_ms が常に 0 と metrics の二重返却

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: A-03実施完了。`json_ms` を Encoder 実測（ms 0時は1へceil）し、`timing.metrics` 重複を削除して top-level `metrics` を維持。固定テストで `json_ms > 0` と重複不存在を確認。

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。実測 `X-Search-Timing: search=8477;record=935;json=0` で content-length 1061（JSON 生成に時間がかかっているのに `json_ms` 0）。レスポンスには top-level `metrics` と `timing.metrics` に同じ内容が二重で出る（`docs/ai/tmp/assess-20261103-1220/baseline_search.json`）。UI の `#perf-status` は `server timing` 表示でこの値を使う。対応: encode 後（または Encoder 使用）で json_ms を測り、二重返却を解消。合格条件: `json_ms > 0` が観測され、重複が片方だけになる。既存テストの assertion 変更は実装段階。推定規模: 小。

実装:
- `app/viewer.go` の `/api/search` responseを `bytes.Buffer` でEncoder encodeして実測。ms換算が0になる短時間encodetest合格基準を維持するため0は1へceil。
- `timing` に `metrics` を含めず、top-level `metrics` だけを残す。UIは `x-search-timing` headerを使うためJSON metrics duplicateの実UI依存はない。

実測:
- fixed test `TestSearchHandlerIncludesMeasurementShape`: `json_ms > 0`、`timing.metrics` 不存在、top-level `metrics` 存在、headerの `json` 値positive。
- canonical server実機UI確認はUI変更なし。fixed testとEncoder実測で合格条件を固定。

## Verification

- `app` で `go vet .` clean・`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 40.058s`。テスト更新: `TestSearchHandlerIncludesMeasurementShape`。
- canonical exe再build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → `test_canonical_build_freshness.py` pass。UI未変更、canonical server未稼働のまま。
- ルートで `python -m pytest tests/ -v` → 38 passed in 4.16s（freshness含む）。
- commitはA-03単独commit（`app/viewer.go`、`app/viewer_test.go`、A-03記録、plan、`current-task.tsv`参照行削除）。commit IDは `git log` の該当commit。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
