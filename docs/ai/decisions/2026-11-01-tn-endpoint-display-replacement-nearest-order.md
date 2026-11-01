# 2026-11-01 TN endpoint display replacement and nearest-order prefetch

**Superseded by**: none

## Decision
- `x-cache` is status metadata for prefetch counting. TN fallback bytes are not counted as full prefetch.
- UI display must replace the placeholder thumbnail with the image endpoint bytes regardless of `x-cache`, while retaining the cache status in `img.dataset.fullCache`.
- Background prefetch order must prioritize pages nearest to the currently viewed reader page.

## Reason
- The visual placeholder should reflect the actual `/api/image/` response even when that response is TN fallback bytes.
- TN fallback bytes cannot be persisted as full image cache, so status must remain TN.
- Prefetch should benefit the page the user is viewing first.

## Impact
- Status text can show `tn` while displayed images point to the same `/api/image/` endpoint.
- Full image content cannot be shown when the server returns TN bytes; UI can only reflect those TN bytes.
- The background prefetch order changes from forward-from-current to nearest-first.
