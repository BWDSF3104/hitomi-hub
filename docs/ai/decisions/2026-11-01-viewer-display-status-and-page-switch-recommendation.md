# Viewer actual display pass と status の分離、page switch delay 推奨

**Date**: 2026-11-01
**Status**: 決定（新セッション移行用・未実装）

## Purpose

新セッションで page switch delay と status reporting の方針を継続するため、`actual display pass` と `status` の意味、local full file の display source、UI status line を明確にする。

## Decision

1. `actual display pass` は DOM actual pixel truth である。

   合格条件:

   - current displayed `<img>` に `src` がある
   - `naturalWidth > 0`
   - viewer display source が full image objectURL / local file / remote full fetch result である
   - TN viewer fallback は禁止

2. `status` は backend/cache/source truth である。

   `status` は actual display pass ではない。`status = local` や `status = cache` は actual display pass を意味しない。

3. `full display cache` は objectURL cache の count として扱う。

   将来の UI status line では `actual pixels` と `objectURL cache` を別項目として表示する。

4. Local full entry では direct `/files/...` display src を優先する。

   Direct `<img>` では `x-cache` header を読めない。そのため status は server metadata から決める。

   - `sourceStatus`: `entry.local` / `entry.local_formats`
   - `sourceType`: `local-direct`
   - `displayPass`: `img.onload` 後の `naturalWidth > 0`

5. Page switch は actual display path を background prefetch にブロックさせない。

   Display source selection order:

   - active objectURL があれば objectURL
   - local full entry は direct `/files/...`
   - viewer actual display source が不明な remote full は fetch/blob/objectURL で取得
   - TN viewer display fallback は使わない

6. Stale async writes は current generation だけで許可する。

   Status / displayPass updateは `showGeneration`, `openGeneration`, `renderGeneration`, `backgroundGalleryId` をチェックする。Stale request の status write, strip write, displayPass update は無視する。

7. Status UI recommended format:

   ```text
   actual pixels X/Y | local direct L/Y | objectURL cache C/Y | no full N/Y | TN display fallback disabled
   ```

   Meaning:

   - `actual pixels X/Y`: actual display pass entries
   - `local direct L/Y`: direct `/files/...` src for viewer display
   - `objectURL cache C/Y`: active objectURL entries
   - `no full N/Y`: full bytes unavailable
   - `TN display fallback disabled`: TN display fallback remains forbidden for viewer display

## Data model

Use entry key:

```text
key = galleryId + "|" + hash + "|" + format
```

Maps:

```text
displayPassByEntry: key -> boolean
sourceStatusByEntry: key -> local | cache | tn-cache | tn | no_full | error
sourceTypeByEntry: key -> local-direct | objectURL | no_full | error
activeObjectUrlKeys: key -> true
```

Rules:

- `displayPassByEntry` is set only when `naturalWidth > 0` is observed.
- `sourceStatusByEntry` is source/cache truth.
- `activeObjectUrlKeys` tracks active objectURL. If objectURL is revoked, it cannot satisfy `objectURL cache` and should be removed.
- A stale revoked objectURL must not be treated as display pass.

## Tags display area

Backlog `#13` として記録済み。

Recommended CSS:

```css
.card {
  --search-card-height: 130px;
}

.tags {
  font-size: .9rem;
  line-height: 1.25;
  max-height: var(--search-card-height);
  overflow: hidden;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-line-clamp: 9;
}
```

Full tags must remain available through `title` attribute. Tags display area does not affect viewer actual display pass.

## Page switch delay measurement

Measured on cache-less ongoing runtime root:

```text
S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing
http://127.0.0.1:8736
```

- When JS `fullCache` already had active objectURL:
  - `showPage()` was about `4.3ms`
  - reader `2/2` actual pixels, `naturalWidth > 0`

- When `fullCache` was cleared and page was displayed again from server local file:
  - actual display took about `270.6ms`
  - server resource duration was about `255.3ms`

Conclusion:

- The delay is not missing downloaded full data.
- It is mostly JS fetch/blob/objectURL path plus browser decode overhead.
- Direct local src is the recommended improvement for already-downloaded local entries.

## Backlog note

- `#12` was removed as completed because strip group controls made the current 50-item group selectable.
- `#13` is Tags display area clamp.
- `#14` is Viewer page switch display/status separation.

## New session migration rules

- Future screen confirmation must use cache-less runtime root:
  `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing`
- TN display fallback is absolutely forbidden for viewer display.
- `status` text alone is not actual display pass.
- `naturalWidth > 0` is required for actual display pass.
- `no_full` is not actual display pass.
- Search-card TN cover is search-card display, not viewer display pass.
- Canonical server should not be stopped except for EXE update.
