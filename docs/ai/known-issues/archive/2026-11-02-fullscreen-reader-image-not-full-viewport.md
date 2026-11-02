# Fullscreen reader image does not fill the screen

Status: open

## 確認結果

追加確認としてフルスクリーン表示を確認した。

### 1. フルスクリーン画面が画面いっぱいになるか

`#reader` fullscreen target は viewport full size で表示される。

Measured viewport:

- viewport: `1375x938`
- `#reader`: `1375x938`

But image itself is not full screen.

Single fullscreen image:

- `natural size`: `1134x1577`
- displayed size: `500x700`
- `object-fit`: `contain`

Double fullscreen images:

- `natural size`: `1134x1577`
- displayed size per image: `500x700`
- `object-fit`: `contain`

This is because fullscreen CSS still caps the reader page height:

```css
body.fullscreen .reader { --reader-page-height: min(90vh, 700px); }
body.fullscreen .reader img { height: var(--reader-page-height); }
```

And width remains capped by `--reader-page-width` in the base reader rule.

Therefore:

- fullscreen container fills the screen: pass
- image fills the screen / image maximizes to available screen area: fail

If the pass criterion is "image fills full screen", current implementation does not pass.

### 2. フルスクリーン画面でキーボードページ移動を受け付けるか

Accepts keyboard page navigation.

Test sequence in fullscreen:

1. Open gallery from search result.
2. Press `f` while focus is outside `INPUT`, `BUTTON`, `SELECT`.
3. `ArrowRight` moves page.
4. `ArrowLeft` moves page.

Evidence:

- fullscreen state: `on`
- fullscreen element: `#reader`, reported as `DIV`
- key `f`: entered fullscreen
- key `ArrowRight`: changed `current.page` from `1` to `3`
- key `ArrowLeft`: changed `current.page` from `3` to `2`

Pass condition remains:

- keydown target must not be `INPUT`, `BUTTON`, or `SELECT`.
- focus must be on body, `#reader`, or another non-input element.
- `ArrowLeft` and `ArrowRight` must call `movePage()` and update `current.page`.

## Required pass criteria for fullscreen image

To pass the first user criterion, fullscreen confirmation should require:

1. `#reader` as fullscreen target or pseudo-fullscreen target has full viewport size.
2. The displayed image size, not only the container, must fill the fullscreen display area.
3. `object-fit: contain` may be kept for aspect ratio, but the container must not cap the image below available fullscreen area.
4. In single fullscreen, image area should use approximately full viewport width/height, minus a documented padding.
5. In double fullscreen, each page should fill its half of the available screen width and available fullscreen height.
6. The pass must be measured before manual resize, using a fresh tab and actual keyboard `f` entry.

## Current fail condition

Current fullscreen pass fails for the "image fills full screen" criterion because `--reader-page-height` remains capped at `700px` and `--reader-page-width` remains capped at `500px`.

Correction method and pass criteria are recorded in `docs/ai/decisions/2026-11-02-fullscreen-image-display-size-criteria.md`.
