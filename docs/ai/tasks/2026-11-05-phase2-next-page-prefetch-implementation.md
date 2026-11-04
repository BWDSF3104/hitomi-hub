# Phase 2 実装 — 次ページのアイドル先読み（優先2・未開始）

**Status**: 進行中（実装完了・合格条件1項目未達）
**Date**: 2026-11-05
**Next Action**: conflict first card display 合格条件（baseline+1000ms）未達（remote 変動 +2169ms）。run C cold run の card 4304598 cache=1 異常は未解明（known-issues 候補）。次: remote 変動を排除した再測定か条件解釈のユーザー判断。commit/push 済み（本項目分のみ）。

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase2-next-page-prefetch`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase2-next-page-prefetch.md`）の実装タスク。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 2（全体順 Phase 1→P3→**P2**→P4。根拠は評価記録の記載のみ）。
- 評価記録（有効性「有効」・実測で閉じ・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase2-next-page-prefetch.md`。
- 検討結果の反映（推奨）: client 側スケジューラ（`app/search.html` に閉じる）。発火=現ページ card 完了＋800ms アイドル（「次」hover/focus で即時）。対象=page+1 のみ。取得順 ID→`/api/search-card/<id>`（並行度 2）→TN cover。中断=新規検索/ページ移動/gallery 開/query 入力/`visibilitychange` hidden／429・5xx→全体停止＋5分クールダウン（メモリ保持）。マーカー=query param `priority=background`（実測で応答・cache 挙動不変確定済み。サーバー側は metrics 区分記録のみ・ゲート制御は P4 範囲）。世代は `searchGeneration` に紐づけ。
- 順序依存: Phase 1（優先1・未開始）と `app/search.html` を共有 — 既存実施計画の順 P1→P2。P3 とは file 競合なし（P3 先行なら P2 成果が永続化され P2 側追加作業不要）。P4 の要否判定は本実装の並行度パラメータに依存（1 固定なら P4 不要）。

## 調査結果 / 変更内容

- `app/search.html`: scheduler block 追加（`PREFETCH_IDLE_MS = 800`・`PREFETCH_CONCURRENCY = 2`・`PREFETCH_COOLDOWN_MS = 300000`）。`api()` に `err.status = r.status`（429/5xx 判定用）。`runSearch()` で `searchCardsCompleted` フラグ管理＋idle アーム。`#search-next` hover/focus で即時発火、query input / gallery 開 / `visibilitychange` hidden / hashchange で中断、429/5xx で全体停止＋5分クールダウン（メモリのみ）。対象は page+1 のみ。`prefetchEligible` は saved query startup で false、submit で true。既存 submit handler の exact string（Phase 1 test 固定）は別 listener `prefetchEligible = true;` で維持。
- `app/metrics.go`: `recordMarkerRequest(priority string)` → `recordMetric("marker."+priority, 0, 0, 0, 0)`。`app/viewer.go` の search/searchCard/thumbProxy handler で marker 記録（応答・cache 挙動不変）。
- `app/metrics_test.go`: `TestBackgroundPrefetchMarkerRecordedSeparatelyInMetrics`（mock fetch `classifyFetchURL == "image"` で TN・それ以外 gallery、metrics gallery/image calls 1 ずつ、marker.background 2）。
- `tests/search_prefetch_harness.mjs`（40 checks）＋ `tests/test_search_prefetch.py`。
- design decision 参照: `docs/ai/decisions/2026-11-05-phase2-scheduler-scope.md`、`2026-11-05-phase2-marker-metrics-only.md`。作業前チェックの decision 読了: `2026-11-01-root-and-tn-display-correction-policy.md`、`2026-11-01-fast-nonblocking-viewer-policy.md`、`2026-11-01-ggset-generation-and-drift.md`。

## Verification

- `go vet .` ✓ ／ `go test -count=1 -timeout 300s .` ✓（17.269s）／個別 guard tests（`TestStorageURLUsesFormatNotNameExt|TestCanonicalGGRuleComposition|TestThumbnailURLMatchesVerifiedTNPath|TestBackgroundPrefetchMarkerRecordedSeparatelyInMetrics`）✓。
- `python -m pytest tests/ -v` 53 tests ✓（`test_search_prefetch.py`・`test_canonical_build_freshness.py` 含む）。node harness 40 checks ✓。Phase 1 harness（`TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit`）✓。
- canonical build ✓（`output/hitomi-hub/hub.exe`）。正典 server run E（fresh root `p2_phase2_fresh5`・port 8732・fresh isolated tab）。
- server direct: `X-Search-Timing: search=7530;record=36;json=1`・total 58523・ids 25（`total 0`・stale なし）。UI cold run（cacheDistribution miss 25・`X-Search-Timing` あり・`.side` 480px・`#query` 一意・resize なし・card 25=ids.length）→ direct と一致 ✓。baseline first card display 2321ms。
- scheduler 発火: `female:yuri` p2 background fetch 25件（requests 114-138）、`furry` p2 background（requests 46-70）は新規検索開始で中断 ✓。
- transition（p2→p3・prefetch 完了後 UI click）: 616ms ≤1000ms ✓・card fetch 0・metrics gallery/image delta 0 ✓（transition 中の metrics 増分は p4 prefetch 完了分）。
- conflict（p8 cold 新規検索・background prefetch running）: `image` gate slot ≈33ms ≤1000ms ✓。first card display 4490ms（baseline+2169ms）→ baseline+1000ms 超過 ✗（remote image fetch ネットワーク ~4.3s が支配的。合格条件のこの項目は未達）。
- 異常（未解決）: run C cold run の card `4304598` が `x-cache: cache`（cold root・原因不明）。run E cold run では再現なし（miss 25）。known-issues 候補として observation 記録。
- Google Drive strict 確認: `I:\マイドライブ\kilo\2026-11-05-phase2-next-page-prefetch-implementation\` に `procedure-20261105.txt`、`runE_conflict_p8.png`、`phase2-baseline-search-page-20261105.png`、`phase2-baseline-next-page-on-click-20261105.png` 保存 ✓。
- 総合: 合格条件のうち conflict first card display ≤ baseline+1000ms のみ未達（remote 変動由来・gate slot は ✓）。実装・static tests・transition・中断・クールダウン設計は合格。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
