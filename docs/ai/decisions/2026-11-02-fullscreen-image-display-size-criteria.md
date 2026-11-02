# Fullscreen image display size correction method and pass criteria

Status: active.

This decision sets the correction method and screen confirmation pass criteria for the fullscreen image display size. Previous confirmation showed that the fullscreen reader container fills the viewport, but the displayed image is still capped to `500px x 700px`. That fails the criterion "the image should be shown as large as possible in the fullscreen screen".

## Known problem

Measured fullscreen state:

- viewport: `1375x938`
- `#reader` fullscreen target: `1375x938`
- displayed image size: `500x700`
- `object-fit`: `contain`
- `--reader-page-height` in fullscreen: `min(90vh, 700px)`
- `--reader-page-width`: capped at `500px`

Current behavior:

- fullscreen container fills the screen: pass
- image display box fills the screen: fail
- image display size maximized to available fullscreen area: fail
- keyboard page movement in fullscreen: pass, when focus is not `INPUT`, `BUTTON`, or `SELECT`

## Correction method

The correction should make fullscreen use a fullscreen-specific size contract, not the normal viewer size contract.

1. Keep `#reader` as the fullscreen target, or keep `#reader` as `pseudo-fullscreen` fixed target when actual fullscreen is unavailable.

2. Add fullscreen-specific CSS variables.

   Suggested variables:

   ```css
   body.fullscreen .reader,
   body.pseudo-fullscreen .reader {
     --fullscreen-page-padding: 12px;
     --fullscreen-page-height: calc(100vh - var(--fullscreen-page-padding) * 2);
     --fullscreen-page-width: calc(100vw - var(--fullscreen-page-padding) * 2);
   }
   ```

3. Fullscreen reader must fill the fullscreen viewport.

   For actual fullscreen:

   ```css
   body.fullscreen .reader {
     width: 100%;
     height: 100%;
     display: grid;
     gap: 0;
   }
   ```

   For pseudo fullscreen:

   ```css
   body.pseudo-fullscreen .reader {
     position: fixed;
     inset: 0;
     z-index: 10;
     margin: 0;
     padding: var(--fullscreen-page-padding);
     background: #111;
     width: calc(100vw - var(--fullscreen-page-padding) * 2);
     height: calc(100vh - var(--fullscreen-page-padding) * 2);
   }
   ```

4. Fullscreen single page should use the full available display area.

   ```css
   body.fullscreen .reader:not(.double),
   body.pseudo-fullscreen .reader:not(.double) {
     grid-template-columns: minmax(0, calc(100% - var(--fullscreen-page-padding) * 2));
   }

   body.fullscreen .reader img,
   body.pseudo-fullscreen .reader img {
     width: min(calc(100% - var(--fullscreen-page-padding) * 2), 100%);
     height: calc(100vh - var(--fullscreen-page-padding) * 2);
     object-fit: contain;
   }
   ```

5. Fullscreen double page should split the available fullscreen area into two cells.

   ```css
   body.fullscreen .reader.double,
   body.pseudo-fullscreen .reader.double {
     grid-template-columns: repeat(2, minmax(0, calc((100% - var(--fullscreen-page-padding) * 2) / 2)));
   }
   ```

6. Aspect ratio must be preserved.

   - Use `object-fit: contain`.
   - Do not use `object-fit: cover`.
   - Letterbox areas are allowed when the natural image aspect ratio does not fit the cell.
   - The pass criteria should measure the image display box, and the displayed image must maximize the limiting dimension inside that box.

7. Fullscreen keyboard behavior must remain valid.

   ```js
   window.addEventListener("keydown", async (e) => {
     if (!current.id) return;
     if (e.target.tagName === "INPUT") return;
     if (e.target.tagName === "BUTTON") return;
     if (e.target.tagName === "SELECT") return;
     if (e.key === "ArrowLeft") { if (movePage(-1)) await showPage(); }
     if (e.key === "ArrowRight") { if (movePage(1)) await showPage(); }
     if (e.key === "f") setFullscreenFromGesture();
   });
   ```

   In fullscreen, `#reader` can be the fullscreen target, but `#reader` must not be `INPUT`, `BUTTON`, or `SELECT`. Therefore keyboard navigation must pass when focus is on `#reader` or body.

## Screen confirmation pass criteria

Use a fresh browser tab, actual UI gallery, actual `f` key entry, and measure before any `window.resize`.

### Container pass criteria

The fullscreen display is valid only when `#reader` fills the fullscreen area.

Required:

- `#reader` actual bounding rectangle is fullscreen viewport minus fullscreen padding.
- With padding `12px`, `#reader` content area should be approximately `100vw - 24px` by `100vh - 24px`.
- Tolerance: `10px`.
- If actual fullscreen is unavailable, `pseudo-fullscreen` `#reader` must have the same full viewport dimensions.

Example at `1375x938`:

- target reader size: `1351x914`
- pass tolerance: `1341x904` to `1375x938`
- current `#reader`: `1375x938`, so container pass depends on whether `#reader` itself is measured as padding box or content box. For the screen confirmation, the image display area must be measured inside the padding, so the image display box must be approximately `1351x914`, not `1375x938` with no padding.

### Image display box pass criteria

For each displayed fullscreen image, the `img` element display box must fill the available fullscreen cell.

Single fullscreen:

- `img` display box should be approximately `100vw - 24px` by `100vh - 24px`.
- For example viewport `1375x938`, target is `1351x914`.
- Tolerance: `10px`.

Double fullscreen:

- Each `img` display box should be approximately half of the available fullscreen width, and full fullscreen height.
- For example viewport `1375x938`, target per cell is approximately `674.5x914`.
- Tolerance: `10px`.

### Actual displayed image pass criteria

The actual image content may be letterboxed. This is allowed when aspect ratio is preserved.

Required:

- `object-fit` must be `contain`, not `cover`.
- The image must maximize the limiting dimension inside the full display box.
- Portrait image:
  - displayed image height must be approximately fullscreen image display height minus padding.
  - displayed image width is `height * natural aspect ratio`, within `20px`.
- Landscape image:
  - displayed image width must be approximately display box width minus padding.
  - displayed image height is `width * natural aspect ratio`, within `20px`.

Current example:

- natural size: `1134x1577`
- portrait aspect: `width / height = 0.719086...`
- target fullscreen display height: `914px`
- expected displayed image height: `914px`
- expected displayed image width: `657px`, within `657px` to `677px` for double cell tolerance

Current displayed image:

- `500x700`

This fails because the image is capped to `700px` height and `500px` width.

### Keyboard pass criteria

Fullscreen keyboard navigation must pass independently.

Required:

1. In fullscreen, press `ArrowRight`.
   - `current.page` must move forward.
2. In fullscreen, press `ArrowLeft`.
   - `current.page` must move backward.
3. Focus must not be `INPUT`, `BUTTON`, or `SELECT`.
4. If focus is on `#reader`, body, or fullscreen target, `←/→` must be accepted.
5. `f` may be used to enter fullscreen from a user gesture.

### Full fullscreen pass criteria

A fullscreen screen confirmation passes only if all of the following are true:

1. Fresh tab or isolated context.
2. Actual UI gallery is opened.
3. Fullscreen is entered through actual `f` key gesture, or through explicit fullscreen button with user gesture.
4. Before any `window.resize`, `#reader` fills the fullscreen viewport minus padding.
5. Each fullscreen image display box fills the available fullscreen cell.
6. Aspect ratio is preserved.
7. `object-fit` is `contain`.
8. The displayed image maximizes the limiting dimension inside the fullscreen display box.
9. No current fixed cap such as `700px` remains for fullscreen image height.
10. `ArrowLeft` and `ArrowRight` move pages in fullscreen.
11. Existing reader navigation, double/single layout, strip group, view state, and keyboard behavior are not broken.

## Fail conditions

Fail if any of these is observed:

- `#reader` fullscreen target is not fullscreen viewport.
- `#reader` size is correct but image display box is capped to `500px x 700px`.
- fullscreen image height is capped at `700px`.
- fullscreen image width is capped at `500px`.
- image is cropped by `object-fit: cover`.
- aspect ratio is distorted.
- `ArrowLeft` or `ArrowRight` does not move page in fullscreen.
- keyboard handler fails because focus is `INPUT`, `BUTTON`, or `SELECT` when the user intends reader operation.
- fullscreen confirmation uses resize, reload, or stale cache to make the screen pass.
