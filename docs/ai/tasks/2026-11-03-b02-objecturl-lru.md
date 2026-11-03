# B-02: objectURL 2000 件上限で 2001 個目が壊れる

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`MAX_FULL_CACHE_ENTRIES = 2000`（`app/ui.html`）。`storeFullCache`（`app/ui.html:768-785`）は `fullObjectUrls` 追加後に size > MAX で `revokeFullObjectUrls()` を呼び、active を含む全 URL を revoke する。実測（fresh tab, `#reader` に真の cached full を objectURL で表示 → `fullObjectUrls` を 2000 へ填充 → 2001 件目を `storeFullCache`）: size 2000 → store 後 size 0、`activeObjectUrlKeys` 0、`fullCache.get(key).objectUrl` null、`displayViewerSource` ""、img `naturalWidth` 0（表示が空白化）。`prefetchedEntries` は statuses を保持したまま（count は objectURL cache 0/8 だが entry は残る）。スクショ: `docs/ai/tmp/assess-20261103-1220/03-b02-objecturl-revoke-all.png`（Drive へコピー済み）。対応: per-entry LRU eviction（revoke するのは最も古い key だけ）+ `prefetchedEntries`/`activeObjectUrlKeys` の同期削除。合格条件: 上限超過時に表示中の entry の objectURL が有効なまま、LRU 外の entry だけ revoke。推定規模: 中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
