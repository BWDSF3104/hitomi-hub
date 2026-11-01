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
