# 2026-11-01 reader fixed size / no gap / placeholder complete replacement

## Decision
- Reader double page items should have no grid gap.
- Reader displayed size should be fixed at the approximate current full image display size: `min(45vw, 500px)` width and `min(78vh, 700px)` height. Fullscreen uses `min(90vh, 700px)` height.
- Placeholder thumbnail and full image should share the same fixed display box via `object-fit: contain`.
- `loadFullImage()` must replace placeholder not only on `onload`, but also immediately when the full image `Image` object is already complete after `src` assignment.

## Reason
- Cached full images can complete without firing `onload`, so placeholder-only replacement missed cache hits.
- TN placeholder can remain smaller than full image if display size is not fixed.
- Removing gap reproduces hitomi-like continuous double page layout better.
