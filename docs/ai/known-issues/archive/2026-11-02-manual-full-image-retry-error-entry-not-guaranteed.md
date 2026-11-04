# Strict UI 確認で manual retry 用の `errorDone > 0` を作る再現可能な entry が見つからない

Status: 解決（2026-11-04・推奨基準確定）。natural production `error` の追及は不要と判定（ユーザー承認 `decisions/2026-11-04-recommendations-approved-execution.md`）。合格基準は control network failure（500・`x-cache` なし）＋ actual UI click（`TestEmbeddedUIManualFullImageRetryPassCriteria`・combined `error` or `no_full`・button `errorDone>0 or noFullDone>0` で実装・assert 済み）。natural reproduction は optional（デバッガは forced 再現のみで natural 基準を満たさない。既存 network interception が再現手段）。

## Known facts

- Strict UI visual confirmation requires `#retry-failed` to be enabled only when `errorDone > 0`.
- `manualRetryFailedFullImages()` must target `error` or `no_full`.
- In the current served UI, the guard skips every entry, including `no_full`, so it processes 0 entries.
- The UI sets `sourceStatusByEntry` to `error` only from:
  - `prefetchPageImage()` `!r.ok` where `x-cache` header is absent and cache becomes `error`, or explicit error status from worker catch.
  - `loadFullImage()` `!r.ok` or fetch exception.
- Direct server baseline for `http://127.0.0.1:8731/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=error` returns `404 no_full`, not `error`.
- `format=tn` on the same sample hash also returns `404 no_full`.
- Real hitomi gallery hashes inspected via direct `/api/gallery/4294614` are lowercase hex hashes accepted by `validHash()`; `format=error` therefore does not create UI `sourceStatusByEntry` error.
- No existing UI path currently produces an actual UI `error` entry from direct server baseline without a network failure or a guaranteed invalid hash/format path.

### Latest canonical build confirmation

- On canonical rebuilt server `pid 17980`, direct served UI contains `Number(counts.noFullDone) > 0` button enable.
- Fresh isolated browser tab `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2` showed:
  - `no full 37/37`
  - `errorDone 0`
  - enabled `#retry-failed`
  - actual UI click processed `manualRetryFailedFullImages()`
- The same entry remains `no_full`, not `error`, so this is not a reproducible actual UI `errorDone > 0` entry.
- Normal OS physical screen capture for the `no_full` state was obtained on `2026-11-02` at `3840x2160`.
- This screen confirmation passes `no_full` manual-retry processing, not full-image actual-pixel display. Full image actual pixels remain `0/37` because the server still returns `no_full`.
- The previous `errorDone > 0` strict UI condition remains unresolved as a separate case; the current confirmed pass uses the corrected combined `error` or `no_full` target condition.

## Current no_full live blocker

- On canonical server `pid 17980`, fresh tab `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2` showed `no full image: 2` and `no full 37/37`.
- `note: no full image: 2` is the target page label, not the failed-entry count.
- Live `sourceStatusByEntry` values were all `no_full`.
- Evaluating the current `manualRetryFailedFullImages()` guard over `current.files` returned `passCount: 0`, proving actual retry click processes no entries.
- Required behavior: a `no_full` display failure must be selectable for manual retry or reload before `prefetchPageImage()` / `loadFullImage()`.
- The existing `app/viewer_test.go` exact-string assertion currently preserves the wrong guard and therefore passes despite this live behavior failure. Test-first should replace it with an assertion for the correct combined `error` or `no_full` target condition, plus a fresh live UI pass showing a `no_full` entry is processed.

## Additional investigation

- Current server for this investigation is `pid 5152` on port `8731`.
- Natural `no_full` cannot become `error`: `imageProxy` intentionally maps storage 404 to `x-cache: no_full`; UI distinguishes `no_full` from `error`.
- Direct server baseline cannot make UI `error` for the current target because the target entry hashes are accepted by `validHash()` and the server returns `no_full`, not a true backend failure.
- A controlled actual UI failure path was confirmed on `2026-11-02`:
  - UI was opened fresh for `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2`.
  - Browser network interception changed `/api/image/...` responses to `500` with `content-type: application/json` and no `x-cache`.
  - Before the retry click, UI state showed `errorDone: 1`, enabled `#retry-failed`.
  - Actual UI click processed all `37` error entries.
  - After click, UI state showed:
    ```text
    actual pixels 0/37 | local direct 0/37 | objectURL cache 0/37 | no full 0/37 | TN display fallback disabled | error 37/37
    ```
  - `manualRetryRunning` became false, `manualRetryingKeys` was empty, and `#retry-failed` remained enabled.
- This proves the UI `!r.ok` and fetch-exception error counting paths work, but it is a controlled network failure, not a reproducible production server failure or direct server baseline.
- Recommended criterion:
  - Use controlled actual UI network failure for manual retry behavior confirmation.
  - Do not call this a production strict visual pass unless a natural non-404 backend failure without `x-cache` occurs.
  - Keep `no_full` as separate behavior: retry target condition `error` or `no_full` is valid; button enable should accept `errorDone > 0` or `noFullDone > 0`.
  - For normal OS screenshots, bring the Playwright browser window to the front before capture.

### Implementation pass result

- The exact button pass condition was implemented and asserted:
  ```js
  button.disabled = manualRetryRunning || !current.id || !(Number(counts.errorDone) > 0 || Number(counts.noFullDone) > 0);
  ```
- `app/viewer_test.go` now has `TestEmbeddedUIManualFullImageRetryPassCriteria`, asserting combined target condition, combined button condition, recount decrement path, `x-cache` handling, no_full-before-generic-error, re-entrancy, and UI click handler.
- Controlled actual UI behavior was confirmed on canonical server `pid 9652`. Normal OS screenshots were taken after bringing the Playwright browser window to the front.
- Natural production `error` remains unavailable for valid hash storage misses because `imageProxy` returns `404` with `x-cache: no_full`; this remains controlled-behavior evidence, not production natural backend failure.
- Full actual pixels for target gallery `4299126` remain `0/37`. Controlled cache display `1/1` was confirmed using existing cache hash `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3`, but the target gallery still does not contain that hash.

## Full-image actual pixels plan

- Full actual pixels remain `0/37` for target gallery `4299126`.
- `scratch/p3_canonical_fresh/cache/images` has existing full-image cache files. Sample:
  - `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3`
  - `/api/image/86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3?format=avif`
    returns `200`, `x-cache: cache`, `content-type: image/avif`.
  - The browser decoded that cache image as `1295x1812`.
- The existing cache hashes do not appear in target gallery `4299126`, so existing cache alone cannot satisfy target actual pixels.
- Options:
  1. Find a current search-result gallery whose `files[]` hash matches one existing, decodable full-image cache entry, then open it through actual UI and require `naturalWidth > 0` and `displayPassByEntry` pass.
  2. Define a controlled actual UI test with a known decodable cache hash, but that must be recorded as controlled confirmation, not production target pass.
  3. Fetch a current-window hash through the canonical acquisition path; if full bytes are available, it can become persistent cache and display.

## Action

If a reproducible actual UI `error` entry is found later, record it before running the strict confirmation. Use that entry only if it satisfies fresh isolated browser tab, direct server baseline, actual UI network response, and actual UI click requirements.