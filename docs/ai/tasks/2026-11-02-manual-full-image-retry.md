# 手動フル画像リトライの修正方法と合格条件

**Status**: 進行中
**Date**: 2026-11-02
**Next Action**: strict UI confirmation after finding a reproducible actual UI `errorDone > 0` entry.

## 意図 / 背景

現状、フル画像取得失敗時には automatic retry がない。`prefetchedEntries` が処理済み key を保持するため background requeue されず、`statusIsViewerDisplaySource()` も `error` を display source から除外する。既存の自動動作と display fallback の挙動に最小限の影響を与え、UI ボタン操作から任意タイミングで manual retry できるようにする。

## 調査結果 / 変更内容

- `prefetchPageImage()` は fetch 失敗時に status を `error` にして count するが、retry はしない。
- `loadFullImage()` は exception 時に `error` を store/count するが、retry はしない。
- `updatePrefetchCounts()` は `prefetchedEntries.has(key)` が true の場合 early return するため、manual retry で status recount を許可しないと同じ key の再取得を count できない。
- `statusIsViewerDisplaySource()` は `error`, `no_full`, `tn` を display source から除外する。

修正方法:
- `#retry-failed` ボタンを `#prefetch-status` 付近に追加。
- manual retry は `manualRetryFailedFullImages()` のみ。
- `sourceStatusByEntry.get(key) === "error"` の entry を対象。
- `manualRetryingKeys` で re-entrancy を防ぐ。
- `updatePrefetchCounts(status, entry, galleryId = current.id, allowRecount = false)` に optional recount を追加。
- default は false で既存動作を維持。
- manual retry 経路だけ `allowRecount = true` で old count を decrement してから new status を count。
- 既存の `showGeneration`, `openGeneration`, `renderGeneration`, `backgroundGalleryId`, `AbortSignal` ガードを維持。
- 現在表示中の entry は `loadFullImage()`、非表示 entry は `prefetchPageImage()` を manual retry 経路で呼ぶ。

合格条件:
- `S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\decisions\2026-11-02-manual-full-image-retry-criteria.md` を参照。

## Verification

- Manual retry implementation and tests pass, but strict UI confirmation is blocked until a reproducible actual UI `error` entry is found.
- Direct server baseline result:
  - `http://127.0.0.1:8731/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=error` returns `404 no_full`, not `error`.
  - `format=tn` on the same hash returns `404 no_full`, not `error`.
  - Direct `/api/gallery/4294614` returns lowercase hex hashes, so `entryFormat()` does not create `sourceStatusByEntry` error.
- Recorded blocker in `known-issues/2026-11-02-manual-full-image-retry-error-entry-not-guaranteed.md`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
