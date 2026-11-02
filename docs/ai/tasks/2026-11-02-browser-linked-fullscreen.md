# ブラウザ連動フルスクリーン修正と確認

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: documentElement target と actual fullscreen state を実装し、合格条件で確認する

## 意図 / 背景

hitomi.la のようなブラウザ連動フルスクリーンにする。CSS の `pseudo-fullscreen` overlay ではなく、実際の Fullscreen API が browser root に対して active であることを合格条件にする。

## 調査結果 / 変更内容

- `app/ui.html`: `fullscreenTarget()` を `documentElement` に変更。
- actual `fullscreenElement` と target 一致だけを `fullscreenState = "on"` とし、fallback は `pseudo-fullscreen` として明示。
- `f` は user gesture 経由で documentElement fullscreen を要求し、`navigationUI: "hide"` を試行。
- `exitFullscreen` は document-level APIs を使用。
- `fullscreenchange` は actual state を source of truth とし、fallback を保持。
- `keydown` で `f` は control focus からでも fullscreen gesture として扱う。`INPUT` typing では無効。
- fullscreen CSS で `#reader` を full viewport fixed にし、非必須 UI を hidden。
- 合格条件は `docs/ai/decisions/2026-11-02-browser-linked-fullscreen-criteria.md` に記録。

## Verification

- `go vet .` pass in `S:\Documents\Kilo_Commons\hitomi-hub\app`.
- `go test .` pass in `S:\Documents\Kilo_Commons\hitomi-hub\app`.
  - first fullscreen test run: `23.816s`
  - CSS selector fix run: `23.788s`
  - final with `Escape` handler: `23.876s`
- canonical build command:
  - `$env:CGO_ENABLED='0'; go build -trimpath -ldflags "-s -w" -o ..\output\hitomi-hub\hub.exe .`
  - pass, no output
- canonical process after rebuild:
  - `bgp_199e25f9d0015MSrDC1nSk8G7C`
  - URL: `http://127.0.0.1:8731`
- Playwright screen confirmation in fresh isolated browser tab:
  - URL: `http://127.0.0.1:8731/?x=fullscreen-browser-linked-escape#/g/4297838/page/1`
  - actual `f` key gesture from focus `BODY`
  - `document.fullscreenEnabled`: `true`
  - `document.fullscreenElement`: `HTML`, i.e. `documentElement`
  - `fullscreenState`: `on`, actual browser fullscreen, not CSS fallback
  - `#reader`: `1375x938`, fixed full viewport, padding `12px`, content area `1351x914`
  - double grid columns: `675.5px` each
  - each image display box: `675.5x914`
  - natural image size: `1134x1577`
  - `object-fit`: `contain`
  - keyboard: `ArrowRight` moved `page 1` to `3`, `ArrowLeft` moved back to `1`
  - `Escape` exited fullscreen, `fullscreenState` became `off`, last layout `double` preserved
  - visual screenshot had no browser UI: `S:\Documents\Kilo_Commons\Common_Operations\scratch\fullscreen-browser-linked-20261102-visual.png`
  - automated `outerWidth/outerHeight` proxy was `1389x1031`, not equal to inner `1375x938`; browser UI hidden was confirmed visually, not from window dimensions.
- Google Drive confirmation outputs:
  - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-20261102-visual.png`
  - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-confirmation-20261102.txt`

## Additional normal screen capture correction (2026-11-02)

- `app/ui.html`: fullscreen double images align to the visual center seam:
  - left-side visual image content uses `object-position: right center`.
  - right-side visual image content uses `object-position: left center`.
  - `gap: 0` remains.
- `app/viewer_test.go`: added assertion that the visual center seam gap is eliminated.
- Verification after gap correction:
  - `go vet .` pass.
  - `go test .`: `23.607s` pass.
  - canonical build pass.
  - canonical process restarted as `bgp_199e25f9d0015MSrDC1nSk8G7C`, URL `http://127.0.0.1:8731`.
- Normal screen capture confirmation procedure used instead of Playwright:
  - script: `S:\Documents\Kilo_Commons\Common_Operations\docs\ai\tmp\browser-fullscreen-normal-capture.ps1`
  - fresh normal browser window.
  - before capture taken before any key press, with browser chrome visible.
  - page content focused by normal mouse click at `1150,700`; `f` was sent to the page, not typed into the address bar.
  - after capture taken from the same normal screen capture method.
  - before: `S:\Documents\Kilo_Commons\Common_Operations\scratch\fullscreen-normal-before-20261102.png`
  - after: `S:\Documents\Kilo_Commons\Common_Operations\scratch\fullscreen-normal-after-20261102.png`
  - after visual result: browser chrome hidden, page content occupies the full physical screen, double display, no visual center seam gap.
- Google Drive normal capture outputs:
  - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-before-20261102-normal.png`
  - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-after-20261102-normal.png`
  - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-normal-confirmation-20261102.txt`
- Full physical screen correction:
  - The first normal capture was `1920x1080` because the PowerShell process was DPI-unaware.
  - Fixed script calls `SetProcessDPIAware()` before `Graphics.CopyFromScreen`.
  - Full physical screen captured as `3840x2160`.
  - script: `S:\Documents\Kilo_Commons\Common_Operations\docs\ai\tmp\browser-fullscreen-normal-capture-fullscreen.ps1`
  - before: `S:\Documents\Kilo_Commons\Common_Operations\scratch\fullscreen-normal-before-20261102-full.png`
  - after: `S:\Documents\Kilo_Commons\Common_Operations\scratch\fullscreen-normal-after-20261102-full.png`
  - Drive outputs:
    - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-before-20261102-normal-full.png`
    - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-after-20261102-normal-full.png`
    - `I:\マイドライブ\kilo\2026-11-02\browser-linked-fullscreen-normal-confirmation-full-20261102.txt`
  - Full physical capture before/after passes: before shows browser chrome, after shows browser chrome hidden and page content occupying `3840x2160`, double display, no center seam gap.

## Re-run confirmation (2026-11-02)

- Re-ran `browser-fullscreen-normal-capture-fullscreen.ps1`.
- Captured normal OS screen capture, not Playwright/headless screenshot.
- Before dimension check:
  - `3840x2160`
  - browser chrome/address bar visible.
  - browser is not fullscreen.
  - double display visible.
- After dimension check:
  - `3840x2160`
  - browser chrome hidden.
  - page content occupies the full physical screen.
  - double display remains active.
  - visual center seam gap remains zero.
- Result: before/after full physical screen confirmation passes again.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
