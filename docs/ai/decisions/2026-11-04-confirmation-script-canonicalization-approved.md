# 確認スクリプト正典化の決定

**Date**: 2026-11-04

## 決定

- 確認作業で繰り返されるスクリプトを `S:\Documents\Kilo_Commons\Common_Operations\bin\` の単一正典へ寄せる。
- 正典スクリプトは以下とする。
  - `bin/kilo-screen-capture.ps1`: DPI-aware、physical/virtual screen 最大寸法、actual OS screen capture、PNG 保存、task-specific Drive copy、dimension pass/fail。
  - `bin/kilo-fresh-browser.ps1`: fresh browser profile/window、OS-level click、page への key delivery、address bar 入力を避ける gesture。
  - `bin/kilo-cdp-measure.ps1`: CDP evaluate で DOM/network 数値 read だけを行う measurement 正典。capture proxy にしない。
  - `bin/kilo-drive-confirm.ps1`: Google Drive task-specific folder へ confirmation text を書き出す正典。
- hitomi-hub の `tests/harness_lib.mjs` を共有 helper 正典とし、`bodyIn` / project file read を `strip_rebuild_dom_harness.mjs` と `read_lists_debounce_harness.mjs` で共有する。
- `bin\common.ps1` の project root resolution chain をそのまま使う。

## 基準

- normal OS screen capture 基準は `docs/ai/decisions/2026-11-02-browser-linked-fullscreen-criteria.md` を維持する。
- Playwright/headless screenshot、`outerWidth/innerHeight` proxy、CDP evaluate だけで visual pass を判定してはならない。
- capture dimension check は actual saved PNG 寸法と physical/virtual capture size を照合し、`-FailBelow` で exit code に反映する。
- Drive confirmation storage は `docs/ai/decisions/2026-11-02-google-drive-confirmation-storage-procedure.md` と `docs/ai/decisions/2026-11-02-google-drive-task-specific-screenshot-folder.md` に従う。
- `docs/ai/tmp` は scratch であり、正典スクリプトの既定 local output として使用できるが、永続成果物は Drive task-specific folder へ copy する。

## Supersede 対象

- `docs/ai/tmp` に存在する以下の重複スクラッチパターンの正典化:
  - `capture_strict_ui.ps1`
  - `activate_playwright_and_capture.ps1`
  - `check_screenshot_dimensions.ps1`
  - `browser-fullscreen-normal-capture*.ps1`
  - `capture_screen.ps1`
- これらは scratch のまま正典スクリプトへ置き換わる。既存 decision の normal screen capture 基準は supersede しない。
