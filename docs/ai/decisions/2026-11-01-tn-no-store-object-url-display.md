# 2026-11-01 TN no-store and objectURL display

## Decision
- TN image responses must be `cache-control: no-store` because they are fallback bytes and cannot be treated as full image cache.
- Local, persistent full cache, and remote full image responses may keep long private cache.
- UI image fetches must use `cache: "no-store"` for the x-cache check path to avoid stale browser TN responses.
- Displayed reader images should show the fetched response bytes through `URL.createObjectURL(blob)`, not refetch the same URL through `img.src`.
- Displayed object URLs are revoked when the reader re-renders.

## Reason
- Previously, TN fallback bytes could remain cached in the browser, so already fixed placeholders could continue showing TN even when full image fetch was retried.
- Fetching x-cache status and then setting `img.src` to the same URL can cause a second network request.

## Impact
- Existing stale browser TN entries are avoided through no-store fetches and no-store TN responses.
- Full image bytes already cached on the server can be served without network fetch on subsequent requests.
- TN status remains `tn`, while display can show TN bytes.
