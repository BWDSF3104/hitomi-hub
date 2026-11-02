# Browser-linked fullscreen correction criteria

Status: active.

The fullscreen behavior should be browser-linked. It is not enough for `#reader` to fill the page with CSS only, and it is not enough to call fullscreen only on the reader element. A browser-linked fullscreen pass requires the browser itself to enter fullscreen mode through the Fullscreen API.

## Correction method

1. `fullscreenTarget()` must be `documentElement`, not `#reader`.
2. `document.fullscreenElement`, `document.webkitFullscreenElement`, and `document.mozFullScreenElement` must be read as the actual fullscreen truth.
3. Fullscreen entry must happen only from a user gesture, such as `f` key press, fullscreen button click, or fullscreen control click.
4. Fullscreen entry should try browser UI hiding when supported: `navigationUI: "hide"`.
5. Exit must use document-level exit APIs, not element-level exit APIs.
6. `fullscreenState` must be `on` only when the actual fullscreen element is the target `documentElement`.
7. CSS-only `pseudo-fullscreen` remains a display fallback, but it does not pass browser-linked fullscreen criteria.
8. `f` must remain a fullscreen gesture even when focus is on a button or select, because `f` is a shortcut. It must be ignored while typing in an input.

## Browser-linked pass criteria

Use a fresh isolated browser tab, actual UI gallery, actual user gesture, and measure before any `window.resize`.

Required:

1. `document.fullscreenEnabled` is true.
2. Fullscreen is entered through actual `f` key gesture or actual fullscreen button click.
3. `document.fullscreenElement` is active and equals `documentElement`.
4. Browser UI is hidden in an actual normal screen capture. The confirmation must compare before and after. Before must show browser chrome/window not fullscreen. After must show browser chrome hidden and page content occupying the full physical screen. Headless/Playwright screenshots and `outerWidth/innerHeight` proxy alone do not pass.
5. Non-essential page chrome is hidden or does not occupy the fullscreen display area.
6. `#reader` fills the full screen viewport minus fullscreen padding.
7. Image display boxes fill the available fullscreen cell.
8. `object-fit` is `contain`, not `cover`, and aspect ratio is preserved.
9. In double display, the visual center seam gap must be zero. Grid gap remains `0`; the left-side image content must align right and the right-side image content must align left.
10. `ArrowLeft` and `ArrowRight` move pages in fullscreen.
11. `Escape`, browser UI exit, or `f` exits actual fullscreen and `fullscreenchange` syncs state.
12. Fullscreen exit restores the last layout mode.
13. Existing search, reader navigation, double/single layout, strip group, view state, and prefetched full image display are not broken.

## Normal screen capture procedure

Browser-fullscreen visual confirmation must use a normal OS screen capture, not a Playwright/headless screenshot.

Required procedure:

1. Open a fresh normal browser window or fresh profile through the canonical server URL.
2. Capture the screen before any fullscreen key press. The before capture must show normal browser chrome and must not be fullscreen.
3. Focus the page content, not the address bar. A normal mouse click on the reader/page content is the accepted confirmation method. `f` must be delivered to the page, not typed into the address bar.
4. Send the actual `f` key gesture or click the fullscreen control.
5. Capture the screen after the fullscreen transition using the same normal screen capture method.
6. Pass only when before and after compare correctly: before shows browser chrome/window not fullscreen, after shows browser chrome hidden, actual Fullscreen state active, `#reader` occupies the full physical screen, and double pages have no visual center seam gap.

## Fallback pass

`pseudo-fullscreen` may be accepted only as a degraded fallback when the Fullscreen API is unavailable or denied. It fails browser-linked fullscreen criteria unless explicitly marked as fallback.
