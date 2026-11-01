# Operation priority and placeholder policy

Date: 2026-11-01

- UI display/navigation must not wait for image download.
- Current displayed full image is priority: `startFullPrefetch()` sets `currentPriorityDone`, and background workers wait for it before fetching remaining images.
- Reader placeholder uses current full cache objectURL first, otherwise empty placeholder; no TN fetch for reader placeholders.
- Strip uses full cache objectURL via `data-key`; `storeFullCache()` updates the current strip/search-card DOM when a full image objectURL becomes available.
- Search result cards render metadata first with cover `src=''`. TN covers are low priority and are applied only after `currentPriorityDone` resolves.
- `loadSearchCardCover()` must target the cover `<img>` via `img[data-search-id="<id>"]`, not the card `<li>` selector used for lazy card replacement.
- `revokeFullObjectUrls()` must clear the objectURL `Set`, not replace it with an array.
- `thumbProxy()` checks existing full cache before remote TN so already fetched full images can serve thumbnail placeholders without extra network calls.
- TN thumbnails are persistent server-side cache entries under `cache/images/thumbs/<hash>.<format>.<variant>`. `thumbProxy()` order is full cache hit, TN variant cache hit, remote TN. A remote TN success writes the TN variant cache; a persisted TN hit returns `x-cache: tn-cache`.
- Viewer display must not use TN fallback. `imageProxy()` uses local file, persistent full cache, or remote full storage only; it does not fetch TN variants.
- UI `cachedFullSrc()` must exclude `tn` and `tn-cache` from reader display. `loadFullImage()` must not create a full object URL when `x-cache` is `tn`.
- `prefetch-status` shows display intent, not TN display fallback: `full display cache X/Y | no full Z/Y | TN display fallback disabled`. TN counts may remain internal, but the top status must not present TN as viewer display progress.
- `/api/image` storage 404 is not a network error; it should return `x-cache: no_full`. UI should display no full placeholder, count `no_full` separately, and show `no full` in status. True fetch failures remain `error`.
