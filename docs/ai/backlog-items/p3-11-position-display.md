# P3 #11 現在表示位置の簡単表示

## 対象範囲

- Gallery / viewer 表示のみを対象とする。
- 検索結果側の位置表示は対象外。

## 現在の実装状況

- Gallery side has `current.count` as denominator.
- UI mainly shows `#pagenum` input as numerator.
- `#strip-group-status` shows strip group `G/T`, not page position.
- Route and hash already keep page state.
- Pair mode has `page-second` in route and `parseReaderHash()`.

## 実現可能性

- Gallery side can display current position UI-side.
- No server change is required.
- Numerator and denominator are already available in the same UI state.

## 制限事項

- Usually none.
- Fullscreen may hide UI controls. The position should remain visible in `#reader-controls` or an overlay.
- #15 can integrate the position display into its view state row.

## 推奨実装案

- Add `#reader-position` in `#reader-controls`.
- Add helper `renderPosition()` for gallery.
- Display format:
  - `current.page / current.count`
  - If pair mode is active, display `current.page-second / current.count`.
- Call from `showPage()`, `setGalleryMeta()`, `renderGalleryLoading()`, `hashchange`, and `setViewMode()` when display mode changes.
- Keep `pagenum` input working as page input.
- Use existing generation guards so stale render does not overwrite position.

## その他実装案

- Use `#note` without adding new DOM.
- Replace strip group status with page position.
- Integrate into `#view-state` row:
  - mode
  - gallery page X/Y
  - pair
  - fullscreen

## 検証方法

- Add `TestEmbeddedUI*` string tests for `#reader-position`, `renderPosition()`, pair display, and call sites.
- `go test .` and `go vet .` in `app`.
- Screen verify:
  - gallery page changes position without network calls
  - pair mode shows `page-second`
  - reload restore works
  - fullscreen keeps position visible
- Search-side position tests are not required.

## 採用決定

Recommended implementation is adopted for gallery/viewer only. Search result side is out of scope.