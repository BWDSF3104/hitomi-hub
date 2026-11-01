# 2026-11-01 image rate gate and prefetch concurrency

## Decision
- Hitomi image requests use a lower separate rate interval than data calls: default `250 ms` via `hitomi.image_min_interval_ms`.
- General search, galleriesindex, gg, and gallery JSON calls keep the `1000 ms` policy.
- Viewer UI background prefetch concurrency is `4`.
- The image rate gate must be synchronized so concurrent handlers cannot corrupt the last-request time.

## Reason
- Full image download was dominated by the global `1000 ms` client gate, so UI concurrency alone could not improve throughput.
- Images are the main throughput bottleneck and can tolerate a lower interval without changing search/index policy.

## Impact
- Initial remote image fetch can be faster when hitomi does not return `429`.
- Cache hits remain unaffected. TN fallback still requires a full attempt before TN fallback unless separately optimized later.
- If hitomi starts rejecting image requests with `429`, the retry backoff remains in place.
