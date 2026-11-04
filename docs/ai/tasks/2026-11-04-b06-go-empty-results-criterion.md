# B-06残件: Go 側 empty results 基準小修正（key あり 200 応答）

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 実施完了・単独 commit + push（検証済み）

## 意図 / 背景

B-06（close `4f14d00`）の残件。`app/vt.go:150` は key 有無で判定するため、key あり＋空 results の 200 応答では Go 側 `pass` となり Python 参照実装の `not_evaluated`（`stats.total>0` + 空 results）と分岐する。推奨承認（`decisions/2026-11-04-recommendations-approved-execution.md`）で Go 側を empty results 基準の小修正。テスト固定。任意で live 1 回 200 応答（`stats`/`results`）基線固定（key 保管: `scratch/virustotal-api.key`・非追跡・報告に書かない・4 req/min）。

## 調査結果 / 変更内容

- `app/vt.go` 判定分岐修正: key 有無判定 `!hasResults && positives == 0` → empty results 基準 `len(results) == 0 && positives == 0`。key あり＋空 map の 200 応答が `pass` ではなく `not_evaluated` になり、Python 参照実装 `src/scan.py:257`（`not results and positives == 0`）と一致。`scan_cache` 保存は `processFile` 側 `needsCacheStore`（pass/hit のみ）で、`not_evaluated` は保存されない。findings は既存の `api_error:` + status 経路で `api_error:not_evaluated` を維持（変更なし・分岐のみ）。
- テスト追加（mock・実測固定）:
  - `TestVTEmptyResultsWithStatsTotalIsNotEvaluated`（`app/scan_test.go`）: key あり＋空 results＋stats.total>0 → `not_evaluated`・positives 0・pass キャッシュなし
  - `TestProcessFileVTEmptyResultsDoesNotStorePass`（`app/scan_test.go`）: 構造 pass 維持・scan_cache に pass verdict 行なし・findings `api_error:not_evaluated`・source `structural`。Go 側は A-12 scan meta 行（verdict=''）を常に書くため固定は「pass verdict 行なし」で行う（Python 参照実装は scan meta 書き込みなし・`tests/test_scan.py` 既存固定の COUNT 0 との差異は A-12 由来で本項目では非触）
- 回帰維持（変更なし・suite PASS で確認）: positives>0 → hit（`TestVTThreatNameUsesResultAndEngineName`）、404 → `unknown_to_vt`＋negative cache（`TestVT404NegativeCacheAvoidsRepeatedQuotaUsage`）、TTL 実測 30日/2時間（`scanCacheTTL`/`negativeScanCacheTTL`・`TestScanCacheTTLStalePassIsNotUsed`）、key 無・rate gate（`TestProcessFileVTFailureKeepsStructuralPassStatus`・`TestVTRateGate`）、Hitomi gate 1000ms 不変（非触）。
- 任意条件 live: 実施 1 回（hash-only・4 req/min 内）。`https://www.virustotal.com/api/v3/files/27545216921b2559534379243d19446661534481345149624164814776710798` → HTTP 404（key は有効だが応答 200 形状の基線固定はできず・未確認）。追加呼出しなし。key 保管 `scratch/virustotal-api.key`（non-tracked・記録に key 書かない）。

## Verification

- `app`: `go vet .`（1.3s・通過）→ `go test -count=1 -timeout 300s .` 1 回目 20.2s で known-issue flaky `TestSearchCacheSizeCapBoundsMemory`（`docs/ai/known-issues/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`・項目④でテスト側決定化）1 回 FAIL → 単独実行 PASS（0.153s）→ suite 再実行 PASS（18.1s）。新テスト単独 run 全 PASS。
- ルート: `python -m pytest tests/ -v` → 41 passed（7.85s・wall 8.9s）、`test_canonical_build_freshness.py` 含む通過。
- canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（2.1s）。実機 UI 確認は server を起動しない（親が実施）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
