# 見開きフルスクリーン対応・x-cache TN 状態分離

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: 完了（検証済み）

## 意図 / 背景
- フルスクリーンでも見開き表示を可能にする。
- full prefetch が TN fallback を full 完了として扱わず、閲覧部が placeholder のまま残る誤解を防ぐ。

## 調査結果 / 変更内容
- `displayMode()` に加え `doubleActive()` を追加し、`single` でない限り `window.innerWidth >= 720` で fullscreenでも double item を含める。
- `readerClass()` は fullscreen double active時に `reader fullscreen double` を返す。CSSは `.reader.double` と `.reader:not(.double)` で gap0 の固定 size を保つ。
- `setViewMode()` と `fullscreenchange` exit時に `renderReader(); startFullPrefetch();` を呼ぶ。
- `loadFullImage()` は `fetch` の `x-cache` を見て、`tn` の場合は placeholder を維持し、`local|cache|miss` のときだけ full に swap する。
- `prefetchPageImage()` は `x-cache` を返す。`updatePrefetchCounts()` が full / tn / error を分離して `prefetch-status` に出す。

## Verification
- `go vet ./...` passed.
- `go test -count=1 ./...`: `29.935s`.
- Playwright full screen double: `reader fullscreen double`, `gridColumns: 500px 500px`, `readerGap: 0px`.
- Cached full gallery `2648601 page 2-3`: `full prefetch 27/27 tn 0/27`, displayed full image natural `2059x2880`, display `500x700`.
- TN fallback gallery `4295693 page 5-6`: `full prefetch 0/15 tn 15/15`, placeholder remains TN.
- Screenshot: `scratch\hitomi-usage\reader_fullscreen_double_xcache.png`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
