# Immediate reader loading and full search card data

**Date**: 2026-11-01

## Decision

- `searchResult()` は search card で `files`、`scene_indexes`、`format`、`read_state`、`local_formats` を返す。mini page thumbnails は返さない。
- UI は `searchCardCache` に full card data を保存し、`galleryDataFromCard(card)` で gallery data shapeへ変換できる。
- `openGallery()` は full card data または `galleryCache` があれば await前に immediate renderする。
- full data が無い galleryは、route/meta/status 表示後、reader areaへ `data-reader-loading` と `読込中` を即表示する。

## Reason

Actual page content requires gallery metadata (files、format、scene indexes、read state、local formats). Without data, the UI cannot know the real page entry, so actual image content cannot be shown instantly. Full card data makes already-loaded search cards fast; loading UI communicates the remaining async data fetch.

## Verification

- `go vet ./...` pass.
- `go test .` pass `23.825s`.
- Screen verify for uncached loading UI and full card immediate render.