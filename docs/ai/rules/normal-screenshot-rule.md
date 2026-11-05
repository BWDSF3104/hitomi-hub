# 本プロジェクトルール: 通常スクリーンショット

**Date**: 2026-11-05
**Status**: active
**Scope**: hitomi-hub

## Rule

- 本プロジェクトの通常スクリーンショットの被写体は、Playwright で表示したブラウザ画面の画像とする。
- 通常確認のスクリーンショット・visual layoutは `kilo-playwright_browser_take_screenshot` / `kilo-playwright_browser_snapshot` などの Playwright browser画面を基準にする。
- Edge は本プロジェクトではいかなる理由でも使用禁止。Playwright browser toolの target以外で Edgeを起動・参照・接続する確認方法は禁止する。
- 正典スクリプトは Edge executableを実行してはならない。Playwright browser toolの画面のみを通常の確認対象にする。
- 正典 `bin/kilo-fresh-browser.ps1` は Playwright browser toolへ switchするための policy guardとして扱う。Edge executable launchを実行しない。
- `bin/kilo-screen-capture.ps1` は、フルスクリーン状態、OS-level window state、ブラウザ viewport 外、複数 monitor など、Playwright browser画面内の画像だけでは確認できない事項の場合だけ使用する。
- OS full-screen capture を使用する場合、その画像は通常スクリーンショットではなく「例外的 OS full-screen 確認」として記録する。
- 被写体、取得方法、Playwright browser画面で確認できない例外理由を記録に明記する。

## 2026-11-05 適用記録

- 2026-11-05 06:27 に `taskkill /IM msedge.exe /F /T` を実行し、起動中 Edge を停止した。
- 以後、本プロジェクトの通常スクリーンショット確認は Playwright browser画面の画像を基準にする。
