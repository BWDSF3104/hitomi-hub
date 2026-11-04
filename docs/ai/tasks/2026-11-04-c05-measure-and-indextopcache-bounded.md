# C-05観察: 非対象 cache 実測（mock）＋条件付き indexTopCache bounded 化

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 実測済み（有意な成長確認）→ app/searchindex.go call site の承認範囲外分は board ASK で親判断待ち

## 意図 / 背景

C-05（close `52ffde2`）の承認範囲は 4 cache のみ。非対象 cache（`indexTopCache`・`noFullCache`・`versions`）は TTL だけで上限量なし（既知 issue `docs/ai/known-issues/2026-11-04-c05-non-target-caches-unbounded.md`）。項目化条件（memory 使用の実測根拠）を mock 実測で満たせる。ユーザー承認（`decisions/2026-11-04-observation-records-closure-approved.md`）: 有意な成長を示せば `indexTopCache` のみ項目化（既存 `boundedCachePut` 適用・テスト固定・単独 commit）。`noFullCache`（C-08 規則接触）と `versions`（実質 bounded）は非触・別承認。外部 API 呼出しなし。

## 調査結果 / 変更内容

### 実測（mock・外部 API 呼出しなし・server 起動なし）

scratch: `docs/ai/tmp/assess-20261103-1220/c05measure_test.go`（`go test -overlay` で app パッケージへ物理ファイルを書かず注入・2026-11-04 実測）

- `indexTopCache`: version rotation（mock）で N rotation → keys 集合が N に growth（120 → 120・200 → 200・300 → 300）。eviction 0・上限量なし。TTL 超過模拟（fetchedAt を -2h へ移動）後、stale key を access すると refetch・同一 key 再insert されるだけで len は shrink しない（1 回 refetch・len 300 維持）。keys 集合は単調増加＝TTL 超過分しか消えない現状記録どおり。memory convention 換算（推測・entrySize = maxNodeSize = 464 B）: 300 keys ≒ 139,200 B。
- `noFullCache`: K=2000 key insert（stale 模拟・TTL 超過済み）→ access した 10 件のみ delete・残留 1990（上限量なし）。window rotation（bucket 変更）で同 hash でも新 key 追加・old-window stale key 残留（計 1991）。TTL 判定は hit 時だけ、二度と access されない stale key は残留。
- 判定: 有意な成長を示す → 条件付き実装（`indexTopCache` のみ）発動。`noFullCache`（C-08 negative cache 規則接触）・`versions`/`versionAt`/`versionForceAt` は非触維持。

### 判断持ち帰り（承認範囲）→ 親判断: 承認（2026-11-04）

- `boundedCachePut` の適用点 `indexTopNodeCached`（`app/searchindex.go`: 236-269）は insert 直書き map 書換 + hit 時 `lastAccess` 更新の 2 箇所変更が必要。`searchindex.go` は承認範囲（`app/hitomi.go`・`app/hitomi_test.go`）外 → 実装せず board ASK で親へ判断を仰ぎ、承認（範囲拡大・2 箇所のみ）を得て実施。

### 実施（承認後・単独 commit）

- `app/hitomi.go`: `indexTopCacheEntry` へ `lastAccess` field・`entrySize() = maxNodeSize`（C-05 convention）・`lastAccessed()` 追加、`Client` へ `indexTopCacheBytes`/`indexTopCacheMaxEntries`/`indexTopCacheMaxBytes`・`putIndexTopCacheLocked` helper 追加。cap 既定 `defaultIndexTopCacheMaxEntries = 1024`・`defaultIndexTopCacheMaxBytes = 1 << 20`（根拠: C-05 の構造同一 cache `indexNodeCache`（bnode 1 entry・entrySize = maxNodeSize・topCacheTTL・index fetch path 同一）の既存 convention 値をそのまま採用。invented number なし）。
- `app/searchindex.go`（承認された 2 箇所のみ）: `indexTopNodeCached` の hit 時 `entry.lastAccess = time.Now()`（C-05 の access order 意味）・insert を `putIndexTopCacheLocked` 経由へ。TTL（topCacheTTL）・既存 4 cache・C-08・TN・imageProxy・rate policy・gate 非触。
- `app/hitomi_test.go`: `TestIndexTopCacheEvictsLRUEntriesWhenOverEntryLimit` 追加（既存 C-05 固定テスト形状・insert 後 lastAccess 固定で決定化・cap 1000 B で memory bound 固定）。
- 変更後実測（同一 scratch overlay）: N=1200 rotation → len 1024（cap）・bytes 475,136（≤ 1,048,576）→ bounded 確認。300 rotation（cap 内）は growth 継続（120→120・200→200・300→300）で実測根拠と整合。
- commit 参照: 本項目の単独 commit（push 済み・commit hash は board RESULT 報告）。

## Verification

- scratch 実測 run（変更前・mock・外部呼出しなし）: 1回目 0.528s（テスト側 fixture バグで FAIL・テスト側修正）、2回目 2.701s PASS（`go test -overlay ... -run TestC05Measure -v`）。
- 変更後 bounded 実測（同一 scratch overlay・外部呼出しなし）: N=1200 → len 1024（cap）・bytes 475,136（≤ cap）→ PASS（suite 内 1.664s・wall 3.907s）。
- commit 前（実装後）: `app` で `go vet .` clean（1.044s）、`go test -count=1 -timeout 300s .` ok 17.398s（wall 19.595s・flaky `TestSearchCacheSizeCapBoundsMemory` 発火なし・決定化済み変更なし）、固定テスト単独 PASS（0.154s・wall 1.519s）。
- ルート `python -m pytest tests/ -v`: 51 passed 9.21s（wall 10.063s・`test_canonical_build_freshness.py` 含む）。canonical exe 再 build（0.143s・server 起動・停止なし）。実機 UI 確認は browser  tool を持つ親実施（本項目 UI 非関連）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
