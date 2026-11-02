# Strict UI 確認で manual retry 用の `errorDone > 0` を作る再現可能な entry が見つからない

Status: open.

## Known facts

- Strict UI visual confirmation requires `#retry-failed` to be enabled only when `errorDone > 0`.
- `manualRetryFailedFullImages()` targets only `sourceStatusByEntry.get(key) === "error"`.
- The UI sets `sourceStatusByEntry` to `error` only from:
  - `prefetchPageImage()` `!r.ok` where `x-cache` header is absent and cache becomes `error`, or explicit error status from worker catch.
  - `loadFullImage()` `!r.ok` or fetch exception.
- Direct server baseline for `http://127.0.0.1:8731/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=error` returns `404 no_full`, not `error`.
- `format=tn` on the same sample hash also returns `404 no_full`.
- Real hitomi gallery hashes inspected via direct `/api/gallery/4294614` are lowercase hex hashes accepted by `validHash()`; `format=error` therefore does not create UI `sourceStatusByEntry` error.
- No existing UI path currently produces an actual UI `error` entry from direct server baseline without a network failure or a guaranteed invalid hash/format path.

## Action

If a reproducible actual UI `error` entry is found later, record it before running the strict confirmation. Use that entry only if it satisfies fresh isolated browser tab, direct server baseline, actual UI network response, and actual UI click requirements.