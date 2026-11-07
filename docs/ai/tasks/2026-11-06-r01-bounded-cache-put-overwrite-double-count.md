# R-01: boundedCachePut が上書き時に古い entrySize を引かない

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 同一 key の再 insert で totalBytes が水増ししない（上書き時は old entrySize を引いてから加算）。記録: 2026-11-06-claude-review-20261106-findings.md R-01

## 意図 / 背景

`boundedCachePut`（`app/hitomi.go:92-107`）は C-05 の cache accounting 正典 helper。insert 時に `*totalBytes += entry.entrySize()` するが、既存 key の上書き（map 書き換え）で古い entry の `entrySize()` を減算しない。減算は eviction 時のみ（`app/hitomi.go:104-105`）。TTL 切れ entry は hit 時に map から消えないため、同一 key の再 fetch は上書きとなり、再 insert 回数ぶん `totalBytes` が水増しする。水増しした accounting により、実際の bytes が cap 内でも eviction が走る（早 eviction）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `app/hitomi.go:92-95`: `cache[key] = entry` 後に old entry を参照せず `*totalBytes += entry.entrySize()`。上書き時に減算なし。`len(cache) > maxEntries` は map 反復の件数なので上書きで変化しない（entry 数 guard は正常。影響は bytes accounting のみ）。
- 上書き経路の到達性（TTL hit が entry を delete しない）:
  - `searchCache`: `app/searchhtml.go:128-133`（TTL miss で map に残ったまま refetch → `putSearchCacheLocked`（`app/hitomi.go:219`）で同一 key 上書き）→ 到達可
  - `indexNodeCache`: `app/searchindex.go:282-287` → `putIndexNodeCacheLocked`（`app/hitomi.go:234`）→ 到達可
  - `indexDataCache`: `app/searchindex.go:315-320` → `putIndexDataCacheLocked`（`app/hitomi.go:224`）→ 到達可
  - `indexTopCache`: `app/searchindex.go:240-245` hit TTL miss で map に残る → `!refetched && actualVersion == version` 時に `putIndexTopCacheLocked`（`app/hitomi.go:229`、call site `app/searchindex.go:268`）で同一 key 上書き → 到達可
  - `infoCache`: memory hit（`app/viewer.go:1076-1081`）は TTL 判定なく無条件 return。`putInfoCacheLocked` の call site（`app/viewer.go:1090`・`app/viewer.go:1111`）は memory miss 後にのみ到達 → 現行 call site では上書き経路到達不可（helper 共通のため修正は helper 側で網羅）。父 findings（R-01 列挙）の `infoCache` 含めは「helper 上は対象・現行 call site では unreachable」として区別する。

### 有効性

**有効（確定・実測済み）**。判断保留なし。追加指示が挙げた既存 guard test `TestBoundedCachePutKeepsCountWithinCap`・`TestSearchCacheAccountingAndTTLCapBounded` は grep で該当なし（本コードベースに存在しない。最も近い既存テストは C-05 の size cap テスト群 `TestSearchCacheSizeCapBoundsMemory`（`app/searchhtml_test.go:145`）等。いずれも同一 key 上書きケースを扱わない）。

### 実測（scratch・mock・外部呼出しなし・server 起動なし・2026-11-06）

scratch: `docs/ai/tmp/assess-20261106-1056/r01_measure_test.go`（`go test -overlay` 注入。package main（app 実 package 名））。

1. helper 直接: key `a` に entrySize 40 insert → 同一 key を entrySize 8 で上書き。正味期待値 total=8・観測 total=48（水増し確定）。
2. 早 eviction: cap entries=10・bytes=48。`a`(40)+`b`(8)=48 で eviction なし → `a` を entrySize 8 で上書き。正味期待値 total=16・len=2（both survive）・観測 total=48・len=1・`b` が eviction（実 bytes 16 で cap 内なのに eviction 実行）。
3. 本番経路: `SearchNozomiParts` で 5 ids（40 bytes）insert 後、`fetchedAt` を 2 時間前へ（TTL miss・entry は map に残る）→ mock fetch 1 ids で refetch・同一 key 上書き。正味期待値 bytes=8・観測 bytes=48（水増し確定）。

### 推奨修正

`boundedCachePut` 冒頭、insert 前に既存 entry の減算（3 行程度）:

```go
if old, ok := cache[key]; ok {
    *totalBytes -= old.entrySize()
}
cache[key] = entry
*totalBytes += entry.entrySize()
```

helper 1 箇所で 5 cache すべてを網羅（`searchCache`/`indexDataCache`/`indexNodeCache`/`indexTopCache`/`infoCache`）。回帰テストは同一 key 上書きの accounting 固定（正味 8・ eviction 起きないこと）を scratch と同形状で追加。既存テスト（上書き case なし）は通過維持（baseline suite 通過済み）。

### 副作用・影響範囲・代替案

- 影響: memory 上限の「実 bytes 以下」保証は壊れていない（水増しは早 eviction を生むだけで、実 bytes は cap 以下）。現害は hit rate 低下・cache churn（LRU eviction churn）。重大度: 中・緊急度: 低・データ損失・不可逆変更なし。
- 修正は accounting 意味のみ。LRU 順序（`lastAccess`）・TTL・C-08・full image rules・TN rules・gate・rate policy 非触。UI 変更なし → Playwright 実機確認不要（ルール上）。
- 代替案: 上書き時に old を delete してから insert する案は `lastAccess` を new entry 側で引き継ぐ必要があり複雑（TTL miss の LRU access order 意味との整合で推奨案が自然）。上書き時に hit 扱いで return する案は accounting 問題を解決しない（refetch 値の更新が必要なので put は必要なまま）。

## Verification

- `go vet .`（app）: clean。
- scratch 実測: `go test -overlay "S:/Documents/Kilo_Commons/hitomi-hub/docs/ai/tmp/assess-20261106-1056/overlay.json" -run "TestR01Measure" -v .`（app）→ 3 tests FAIL（FAIL は bug 確認としての想定。水増し total=48/48、早 eviction len=1 を観測）。経過 0.920s。
- 既存 suite（上書き guard なしの確認・baseline）: `go test -count=1 -timeout 300s .`（app）→ ok 26.945s（追加テストなし・既存全通過）。
- 実装実施時（承認後）: helper 修正＋上書き accounting 回帰テスト追加 → `go vet . && go test -count=1 -timeout 300s .`（app）で確認。

### 実装＋回帰テスト（2026-11-07・実作業）

- 採用判断: 推奨案（段階1）。`app/hitomi.go` の `boundedCachePut` 冒頭に既存 key の減算を追加（`if old, ok := cache[key]; ok { *totalBytes -= old.entrySize() }`）。回帰テスト 3 件を `app/hitomi_test.go` に追加（scratch と同形状・合格値を assert: 正味 8・正味 16 len=2 双方 survive・本番経路 bytes=8）。
- `go vet .`（app）: clean。
- `go test -count=1 -timeout 300s .`（app）→ ok 26.659s（既存＋新規全通過。修正前の scratch 実測で FAIL した 3 ケースが修正後 PASS へ反転）。
- 新規回帰テスト個別実行（`-run "...ReplacesOldEntryAccounting|...PrematurelyEvictWithinCap|...TTLRefetchOverwriteKeepsNetAccounting" -v`）→ 3/3 PASS（0.923s）。合格条件（正味 total=8・早 eviction なし）を実測で確認。
- `python -m pytest tests/ -v` → 54 passed / 1 failed（`test_canonical_build_freshness_rule_exists`・既知 failure。AGENTS.md Rules 条の集約が原因。本項目は Python 非変更・本項目の不合格ではない）。
- 試行回数: 1（初回で合格条件を満たす）。UI 変更なし → 画面確認不要（ルール上）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
