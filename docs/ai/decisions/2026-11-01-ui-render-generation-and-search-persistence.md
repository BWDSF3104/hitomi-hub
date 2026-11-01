# UI render generation and search persistence

**Date**: 2026-11-01

## Decision

- Reader full-image cache は gallery id + entry hash + format を key にし、`renderReader()` で毎回 revoke しない。
- `renderReader()` は cached objectURL を優先し、cache が無い entry だけ placeholder thumb を使う。
- Object URL cache は bounded cache とし、上限超過時に `revokeFullObjectUrls()` で objectURL を release する。
- `openGallery()`、`showPage()`、`loadFullImage()`、`loadSearchCards()` は generation で stale async write を ignore する。
- Search state は `localStorage` に `{ query, lang, page, ids, total }` として保存し、reader hash reload でも restore する。

## Reason

Page/work movement during full prefetch was unstable because stale async writes could write to detached DOM or current DOM after a new generation. Already full-prefetched entries showed thumbnails on page movement because `renderReader()` revokes object URLs on every render. Search results were lost on reload because search state was not stored outside default `runSearch()`.

## Verification

- `go vet ./...` in `hitomi-hub/app`: pass.
- `go test .` in `hitomi-hub/app`: pass `23.178s` after duplicate render block fix.
- `go build -o output\hitomi-hub\hub.exe .`: pass.
- Screen verify: `http://127.0.0.1:8731/?x=tnendpoint#/g/4295693/page/5-6`
  - already cached display after page movement uses objectURL, not placeholder
  - stale `loadFullImage()` does not change current reader DOM
  - reload on reader hash restores search results from `localStorage`
- Screenshot: `scratch\hitomi-usage\ui_cache_race_search_persistence.png`