# 既知問題: viewer prefetch worker の race — await 後に `item` が undefined

**Status**: 未解決（R-15 served 確認で 1 回観測。表示・prefetch 完了に影響なし）
**Date**: 2026-11-07
**Source**: assess-20261106-1056（R-15 served 確認・isolated tab・fresh instance pid 92）

## 機構（実測で確定）

- `app/viewer.html` の `worker()`（`PREFETCH_CONCURRENCY` 並行）は `prefetchQueue.length` を checked してから `await currentPriorityDone` する。await の間に他の worker が queue を空にすると、`prefetchQueue.shift()` は undefined を返し、catch 側の `updatePrefetchCounts("error", item.entry, item.galleryId)` で `TypeError: Cannot read properties of undefined (reading 'entry')` が発生（served ページ line 1114）。
- HEAD `0b34704`（R-14）に既存の race。R-15 の変更（cap・DB 記録・prune）は worker に触れていない。

## 影響

- 実測 1 回の console error。gallery 4306867 の reader 実像素 9/9・objectURL 表示・route/reader 一致は正常完了。
- eviction・TN rules・`x-cache` 判定・generation guard は影響なし（R-15 合格条件には非該当）。

## 次の対応（未実施・承認が必要）

- 案: `shift()` 直後に `if (!item) return;` を入れる、または length check を await 後へ移す。実装開始指示が必要。
