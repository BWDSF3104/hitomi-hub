# C-05/A-14: cache に上限がない（対象 cache 全体）

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`searchCache`（`app/searchhtml.go:79-100`）は key ごとに ids 全体（例 `furry` = total 32892 → ids 配列）を保持し、`infoCache`/index caches も TTL だけで上限・eviction がない。実測: `furry` の search response に 25 ids だが cache は total 分の集合。対応: entry 数上限（LRU）または TTL だけでなく size cap。合格条件: 上限超過時に古い entry が消え、memory が一定以下。推定規模: 中。C-05 と同一項目（記録は A-14）。

### 確定設計（実装前）

- 対象 4 cache（`searchCache`/`indexDataCache`/`indexNodeCache`/`infoCache`）に LRU entry 数上限 + size cap（推定 entry bytes）を追加。正典 cache path・full image rules・TN rules・UI full 判定・API rate policy・外部依存は変更しない。
- entry に `lastAccess` を追加（hit 時に access order を更新）。insert 後 `len > maxEntries || totalBytes > maxBytes` の間、最も古い access の entry を delete（単 entry が size cap を超える場合は cache しない=即 eviction）。memory は per-cache bytes 合計で cap 以下をテストで固定。
- 既定上限（conservative）: searchCache 256 entries / 8 MiB、indexDataCache 512 entries / 8 MiB、indexNodeCache 1024 entries / 1 MiB、infoCache 256 entries / 4 MiB。entrySize: ids*8、node は `maxNodeSize`(464)、info は文字列+files 実測式。
- 正典 `boundedCachePut` generics helper（`app/hitomi.go`）で 4 cache 共通化。`indexTopCache`・`noFullCache`・`versions` は承認範囲外（unbounded のまま）→ known-issues に記録。
- 回帰固定: 既存 TTL hit（`TestSearchNozomiPartsUsesTTLCacheAndCopies`、`TestIndexVersionCacheUsesTTL`、top/node/data cache TTL test）、singleflight、C-08 negative cache、full image proxy、TN 表示源規則テスト + 新規 LRU/size cap/eviction 後の再 fetch テスト。合格条件（検証方法）は委譲指示のとおり。

## Verification

- 実装: `app/hitomi.go`（`boundedCachePut` helper・4 cache の entry 型に `lastAccess`+`entrySize()`・既定上限・bytes 合計フィールド）、`app/searchhtml.go`（searchCache hit LRU touch・put）、`app/searchindex.go`（indexNodeCache/indexDataCache hit LRU touch・put）、`app/viewer.go`（`infoCacheEntry`・info() hit LRU touch・put）。正典 cache path・full image rules・TN rules・UI full 判定・API rate policy・外部依存は無変更。UI 変更なし → Playwright 実機確認不要（ルール上）。
- 新規回帰テスト（合格条件固定・8件）:
  - `app/searchhtml_test.go`: `TestSearchCacheEvictsLRUEntriesWhenOverEntryLimit`（上限超過で最古 access eviction・hit で access order 更新・eviction 後は TTL hit ではなく再 fetch・memory cap 以下）、`TestSearchCacheSizeCapBoundsMemory`
  - `app/searchindex_test.go`: `TestIndexDataCacheEvictsLRUEntriesWhenOverEntryLimit`、`TestIndexDataCacheSizeCapBoundsMemory`、`TestIndexNodeCacheEvictsLRUEntriesWhenOverEntryLimit`、`TestIndexNodeCacheSizeCapBoundsMemory`
  - `app/viewer_test.go`: `TestViewerInfoCacheEvictsLRUEntriesWhenOverEntryLimit`、`TestViewerInfoCacheSizeCapBoundsMemory`
  - 既存回帰（変更なしで通過=固定確認）: `TestSearchNozomiPartsUsesTTLCacheAndCopies`（cache hit・copy）、`TestIndexVersionCacheUsesTTL`、top/node/data cache TTL hit tests、singleflight tests（`hitomi_test.go`・`viewer_test.go`）、C-08 negative cache（`TestViewer...negative`・scan_test の negative cache）、full image proxy tests、TN 表示源規則 tests。
- 初回 `go test` で新規 LRU 4 テストが失敗。原因分析: (1) Windows clock granularity（約 15.6 ms）で hit 時刻と直前 insert 時刻が同値（tie）になり map iteration 順で eviction 対象が非決定、(2) テスト期待値の fetch 計数 off-by-one。テスト側で lastAccess に明示的オフセットを入れて決定化＋期待値修正（実装変更なし）→ 再検証合格。同じ失敗の反復なし。
- 実測: `app` で `go vet . && go test -count=1 -timeout 300s .` → ok 64.966 s。canonical exe 再 build 後 project root `python -m pytest tests/ -v` → 38 passed 4.14 s（`test_canonical_build_freshness.py` 含む）。正典 Common_Operations tests（task_manager validation 未破損確認）→ 120 passed 6.11 s。正典 tool 変更なしのため `smoke_safe_rmdir.ps1` 適用なし。
- 承認範囲外として記録: `indexTopCache`・`noFullCache`・`versions` の unbounded（`docs/ai/known-issues/2026-11-04-c05-non-target-caches-unbounded.md`）。判断持ち帰りなし（テストで合格条件固定済み）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
