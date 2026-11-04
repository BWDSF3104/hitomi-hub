# flaky LRU tie: テスト側決定化（tie 排除）

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 単独 commit（テスト側決定化）

## 意図 / 背景

既知 issue（`docs/ai/known-issues/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`）の修復。`TestSearchCacheSizeCapBoundsMemory`（C-05 `52ffde2`）は suite 全体で 1 回 FAIL し得る（LRU tie）。推奨承認（`decisions/2026-11-04-recommendations-approved-execution.md`）でテスト側決定化（tie を排除・insertion order/sizes を確定）。production LRU 意味を変えない（C-05 close 済み・本挙動非触）。単独実行＋suite 全体での再現確認。

## 調査結果 / 変更内容

原因（実測根拠）: 淘汰は `boundedCachePut`（`app/hitomi.go:60-75`）が `lastAccessed()` 最小の entry を選ぶ。同一タイムスタンプ（clock 分解能）だと `e.lastAccessed().Before(oldest)` が false になり、`oldest` が Go map 反復順で決まる → 淘汰対象 non-deterministic。A-09 suite 全体で 1 回 FAIL・単独 PASS（既知 issue 記録）。

テスト側決定化（tie 排除・production 非触）: `TestSearchCacheSizeCapBoundsMemory`（`app/searchhtml_test.go`）で insert 後に各 entry の `lastAccess` を固定（base-30s / -20s / -10s）。直近 insert した entry の `lastAccess` は `time.Now()`（>= base）で固定値より常に後 → 淘汰対象は常に固定値側 entry に決定化（tie 非依存）。既存 sibling LRU テスト（`searchhtml_test.go:85-99`）と同じ test 側 convention。production LRU（`boundedCachePut`・`searchCacheEntry`・TTL・cap 40）の意味は変更なし。既存 assertion（bytes ≤ cap・len == 1・newest `female:cosplay` 生存）はそのまま維持（弱化なし）。既存の他の固定テスト変更なし。

## Verification

- `go vet .`（app）: clean。
- suite 全体 `go test -count=1 -timeout 300s .` 3 回: PASS（74.655s / 74.754s / 70.685s）。`-count=3` 相当も PASS（52.223s）。FAIL なし。
- 単独実行 `go test -count=1 -timeout 300s -run TestSearchCacheSizeCapBoundsMemory .`: PASS（0.174s）。
- ルート `python -m pytest tests/ -v`: 51 passed（10.03s）。`test_canonical_build_freshness.py` 含む（Go テストのみ変更で exe 内容不变 → freshness 通過）。実機 UI 確認・canonical exe 再 build は該当なし（server 起動・停止なし）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
