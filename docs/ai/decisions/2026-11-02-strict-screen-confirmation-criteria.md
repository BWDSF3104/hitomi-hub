# Strict screen confirmation criteria (2026-11-02)

Status: active. This decision sets how screen confirmation pass criteria must be defined. Previous visual confirmation using a stale browser tab is not accepted as meaningful visual confirmation. The fixed target viewport requirement is superseded by `docs/ai/decisions/2026-11-05-window-size-based-screen-confirmation-rule.md`; screen confirmation uses the actual window size on the fresh target.

## Criteria setting method

1. First measure the actual window size on a fresh target and use that as the confirmation basis. Do not require a fixed target viewport.
2. Use a fresh isolated browser tab or fresh profile. Do not reuse a tab that already produced an intercepted/stale response.
3. Take a direct server baseline first. UI confirmation is valid only when the UI network response matches the direct server baseline.
4. Confirm behavior through actual UI submit/click events, not only by checking handler existence or DOM state.
5. Require network, DOM, and visual layout to agree at the same initial moment.
6. Add existing-feature regression checks so a new visual pass cannot hide existing functional breaks.
7. Any pass that requires resizing the window, refreshing, or bypassing a stale cache/route is not a valid initial display pass unless explicitly stated as a cache-clearing precondition.

## Search button screen confirmation pass criteria

Target endpoint example:

- `http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25`

The search button passes only when all of the following are true:

1. Direct server baseline passes before UI confirmation.
   - HTTP status is `200`.
   - `total > 0`.
   - `ids.length > 0`.
   - response headers include `Date` and `X-Search-Timing`.
   - response `content-length` is not the stale shape `84`.

2. UI request is triggered through `form#search`.
   - `#search-submit` click or `form#search` submit must fire.
   - `document.getElementById("search")` submit listener must be attached.
   - `runSearch()` must call `/api/search` with the UI values for `#query`, `#lang`, and `#page`.
   - UI fetch must use `cache: "no-store"` or the server must send `Cache-Control: no-store`.

3. UI network response must match direct server baseline.
   - `total` matches the direct baseline.
   - `ids.length` matches the direct baseline.
   - `X-Search-Timing` is present.
   - If UI receives `total: 0`, `content-length: 84`, `ids.length: 0`, or missing `X-Search-Timing`, the confirmation fails as stale response.

4. UI DOM must reflect the actual network response.
   - `#results` must contain at least one `.card`.
   - `#results` card count must match `ids.length`.
   - `#search-page-status` must show the actual total, not `total 0`.
   - `#count`, `#search-status`, and `#perf-status` must not contradict the actual response.

5. Existing features must not be broken.
   - `#search-prev` and `#search-next` must call `setSearchPage()`.
   - `#query`, `#lang`, `#page` inputs must be usable after search.
   - saved-search `save` must submit successfully.
   - `form#query` must be distinct from `input#query`; duplicate `id="query"` is a fail condition.
   - `document.getElementById("query")` must resolve to the intended saved-search form or the code must use a non-conflicting id/class.
   - reader navigation, view mode, acquisition controls, favorite/unread display, keyboard shortcut handling, and search persistence must not be broken by the change under confirmation.

## Initial display size screen confirmation pass criteria

Initial display size means the visual state at first load before any `window.resize`, before any reload, and before any manual cache clear.

No fixed target viewport is required. Use the actual window size on the fresh target as the confirmation basis. The following `1280x900` values are historical examples, not current target-size pass criteria. At root font-size `16px`, explicit contracts remain checked against the measured window size; otherwise expected values are:

1. `.side`
   - actual `getBoundingClientRect().width` must be `480px`, equivalent to `30rem`.
   - `.side` must not shrink below `480px`.
   - If CSS allows `.side` shrink, the confirmation fails.

2. `form#search`
   - controls must not wrap at the target viewport.
   - `#search` height must be a single control row, approximately `67px` to `70px` in the observed app.
   - If `#search` height becomes `100px` or more because controls wrap, the confirmation fails.

3. `#results`
   - card count must match the actual server `ids.length`.
   - If actual server baseline has `ids.length > 0` but UI shows `#results` with `0 件` or zero `.card`, the confirmation fails.

4. Card backgrounds
   - `.card` cover area must be `130px x 130px`.
   - `.card` background height must be bounded by the documented card height contract.
   - `--search-card-height` must control the card background itself, not only `.tags`.
   - If `.card` height varies unbounded because title/meta/tags/anchor rows expand the card, the confirmation fails unless the accepted design explicitly documents that variation.
   - If P3 #17 same-title anchor intentionally adds a row, the row must be explicit and bounded by a documented maximum card height. Otherwise unbounded card background growth fails.

5. No resize dependency
   - The pass snapshot must be captured before any `window.resize` event.
   - If the display becomes correct only after window resize, the confirmation fails.
   - Resize may be used to prove the bug, but it cannot be part of the acceptance pass.

6. Existing feature regression for initial display
   - direct fetch, UI fetch, DOM card count, card height, `.side` width, `#search-page-status`, `#count`, and `#results` must agree before resize.
   - existing search, saved-search, reader, view mode, acquisition, favorite/unread, and keyboard behavior must still pass after the initial display is accepted.

## Current evidence for strict criteria

Observed on `2026-11-01` through canonical server `http://127.0.0.1:8731`:

- Fresh tab:
  - viewport `958x938`
  - `.side` actual width `480px`
  - `#search` height `67.7734375px`
  - `#results` cards `25`
  - actual fetch:
    - `total: 32878`
    - `ids.length: 25`
    - `content-length: 1080`
    - `X-Search-Timing` present

- Stale tab before resize:
  - viewport `600x800`
  - `.side` actual width `232px`
  - `#search` height `102.859375px`
  - `#results` cards `0`
  - stale fetch:
    - `total: 0`
    - `ids.length: 0`
    - `content-length: 84`
    - `X-Search-Timing` absent

- Same stale tab after resize to `1280x900`:
  - `.side` actual width `480px`
  - `#search` height `67.7734375px`
  - `#results` cards `25`
  - page status became actual.

This proves that the previous confirmation was not sufficient. A valid confirmation must measure before resize, use a fresh tab, compare direct server baseline, network response, and DOM state, and include existing-feature regression checks.
