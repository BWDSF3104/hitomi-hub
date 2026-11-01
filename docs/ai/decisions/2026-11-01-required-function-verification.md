# Required functionality and verification criteria

Date: 2026-11-01

## Required functionality

- The viewer must show actual image pixels when an image exists. Status text, generation guards, metadata, alt text, empty `src`, or broken placeholders are not proof of image display.
- Search cards may show low-priority TN cover images, but viewer completion is not satisfied by search-card TN alone.
- Reader/strip display must show visible actual image pixels when the current image is available.
- If full image bytes are unavailable, UI must clearly distinguish `no_full` from true `error`. This is correct status behavior, but it does not satisfy the functional requirement when the completion criterion requires visible viewer images.
- Priority behavior must remain: current displayed full image is priority, search-card TN is low-priority after priority, TN thumbnails can be persisted server-side, and status reflects display cache/no-full correctly.
- TN display fallback is absolutely forbidden for viewer display. `ensureTNDisplay()` must not be used for `#reader` or `#thumbs` display. Viewer display uses full image objectURL/local file only. If no full image is available, show `no full`; do not use TN pixels for viewer.
- Search-card TN cover is not viewer TN display. It may be processed separately under the existing search-card TN condition and existing rate-limit rules, but it does not make viewer display complete.
- Therefore, if a gallery has no full image but has TN, actual TN pixels may be visible only as a search-card cover, not in `#reader` or `#thumbs`. A reader/strip `naturalWidth=0` state with `no full` is not an image-display pass.

## Verification procedure

1. Use the actual running server, not mocked tests:
   - Canonical server: `http://127.0.0.1:8731`
   - Server EXE remains running except for EXE updates.
2. Perform actual UI operation: search, open a gallery, observe the viewer.
3. Inspect actual rendered image state with DOM checks:
   - `#results img`
   - `#reader img`
   - `#thumbs img`
   - For each expected displayed image, confirm `img.src` is present and `naturalWidth > 0`.
4. Save a screenshot under `scratch\hitomi-usage\` that shows actual visible image pixels.
5. Record the screenshot path, URL, gallery ID, status, `x-cache`, and DOM `naturalWidth` values in the task.
6. Run canonical Go checks:
   - `go vet ./...`
   - `go test .`
   - `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
7. Tests must use mocked `Client.fetch`; they must not call external APIs. Code tests can prove branch behavior but cannot replace actual screen verification.
8. If `naturalWidth=0` in reader/strip, mark the screen verification as abnormal/fail. Do not call it complete based only on status wording, generation guard, or source-code substring assertions.

## Loading-navigation behavior

- During loading, page movement, page input, gallery switch, or search replacement must immediately reflect the new target.
- Stale async writes must be ignored by the correct generation guard: `openGeneration`, `showGeneration`, `renderGeneration`, `searchGeneration`.
- `prefetchAbort` must cancel or stop applying old prefetch work when the displayed gallery/page changes.
- Old `loadFullImage()` or `prefetchPageImage()` results must not write the current DOM if their render/page/hash/generation no longer matches the current target.
- Background workers must wait for the current displayed priority for the current target, not for an old target.
- Status counts must remain scoped to the gallery being displayed; stale loads from another gallery must not contaminate current status.
- Strip/search-card cover updates must not write stale DOM elements after navigation.
- Expected result after a loading-time switch: route, title, reader DOM, strip, status, and cover images correspond only to the latest target. If any stale load writes title, route, reader img, strip img, status, or cover, verification fails.
- Page movement should be as lightweight as possible. For frequent next-page movement, `showPage()` should not recompute the full background order for all entries every time. It should display the current target from cache and keep the existing queue stable unless the target changes.
- Full image priority remains for the current displayed full image. Search-card TN is separate and low-priority, but must not be treated as viewer fallback.
- `post("/api/read")` and `showLists()` should be background/debounced so frequent page movement does not wait for them.
