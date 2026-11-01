# Viewer persistent image cache

**Date**: 2026-11-01

## Decision

- Viewer の full image proxy は server-side persistent image cache を持つ。
- Lookup order は `local downloaded file > persistent full cache > remote full image > TN variant fallback`。
- persistent cache path は `cache/images/<hash>.<format>`。config の `cache_dir` が無い場合、default は project root 配下 `cache/images`。
- remote full storage fetch success 時だけ persistent full cache に bytes を保存する。TN variant fallback は full quality cache として保存しない。
- `imageProxy` は `x-cache: local|cache|miss|tn` を返し、cache hit が network call を再発しないことを検証できる状態にする。

## Background prefetch

- Viewer 画面で current page を即表示した直後、その gallery の残 full images を background で prefetch する。
- UI は `AbortController` と concurrency `2` を使う。
- order は current page を優先し、その後 current strip group、残 page を扱う。gallery/page change 時に旧 prefetch を abort する。
- current page は `showPage()` で `img.src` に設定済みとして status 上の 1 件として扱い、背景 fetch では skip する。
- background prefetch は `current.files` の `local_formats` と `hasavif` から full image URL を組み、per-page `/api/gallery/<id>/page/<n>` JSON を呼ばない。local file は `/files/<id>/<hash>.<local format>`、remote full は `/api/image/<hash>?format=<format>&gallery=<id>` を使う。
