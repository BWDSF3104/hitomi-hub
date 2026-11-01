# Immediate gallery navigation during full prefetch

**Date**: 2026-11-01

## Decision

- `openGallery()` は gallery data fetchのawait前に、旧prefetch abort、route update、search card meta/status displayを行う。
- `galleryCache` に一度開いた gallery data を保存し、cache済み gallery は immediate renderへ使う。
- `searchCardCache` に lazy search card metadata を保存し、未cache gallery route/statusを immediate表示する。

## Reason

Click handlerが native hashchangeを `preventDefault`して `openGallery()` を awaitするため、gallery data fetchまで URLと画面が古いgalleryに残る。full prefetch中の clickで「即座に作品ページへ移動しない」と感じる原因になる。

## Verification

- `go vet ./...` pass.
- `go test .` pass `23.304s`.
- `go build -o output\hitomi-hub\hub.exe .` pass.
- Screen verify:
  - uncached `openGallery(4295770,1)` callで await前に `window.location.hash` が `#/g/4295770/page/1` へ更新され、search card title/status `gallery read load: 4295770` を確認。
  - cache済み gallery page3へ即移動し、reader objectURL displayを確認。
- Screenshots: `scratch\hitomi-usage\immediate_gallery_route_search_card.png`, `scratch\hitomi-usage\immediate_cached_gallery_movement.png`