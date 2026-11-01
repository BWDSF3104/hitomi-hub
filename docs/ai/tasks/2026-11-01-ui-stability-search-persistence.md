# UI full cache/race guard/search persistence

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: Implement cache key, render generation, stale async guard, and search state persistence.

## 意図 / 背景

ユーザー報告の不安定点を修正する:
- full prefetch中の page/work movement で stale async write が current reader を汚す
- full prefetch済み gallery で page movement時に objectURL cacheが revoke され thumbnail placeholderへ戻る
- reload時 reader hashでは search resultsが default searchで消える

## 調査結果 / 変更内容

- `fullCache`, `prefetchedEntries`, `countsByGallery`, `galleryFormats` を追加し、full prefetch済み entryの objectURL/statusを保持する。
- `renderReader()` は cached objectURLを優先し、cacheが無い entryだけ placeholder thumbを使う。毎回 revokeしない。
- objectURL cacheは bounded cacheとし、上限超過時に `revokeFullObjectUrls()` で objectURLを releaseする。
- `openGeneration`, `showGeneration`, `renderGeneration`, `searchGeneration` を追加。`openGallery()`, `showPage()`, `loadFullImage()`, `loadSearchCards()` の stale async writeをignoreする。
- Search stateを `localStorage` の `hitomi_hub_last_search` に `{ query, lang, page, ids, total }` として保存し、reload restoreする。`runSearch()`で `#/q/<query>/page/<page>` routeを反映する。

## Verification

- `go vet ./...` in `hitomi-hub/app`: pass.
- `go test .` in `hitomi-hub/app`: pass `23.178s` after duplicate `renderReader()` block fix.
- `go build -o output\hitomi-hub\hub.exe .`: pass.
- Screen verify: `http://127.0.0.1:8731/?x=tnendpoint#/g/4295693/page/5-6`
  - page movement after full prefetch keeps objectURL display and no placeholder flicker
  - stale render `loadFullImage()` does not change current DOM
  - reload on reader hash restores search results from `localStorage`
- Screenshot: `scratch\hitomi-usage\ui_cache_race_search_persistence.png`
- Decision: `docs/ai/decisions/2026-11-01-ui-render-generation-and-search-persistence.md`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
