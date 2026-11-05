# 本プロジェクトルール: 通常スクリーンショット

**Date**: 2026-11-05
**Status**: active
**Scope**: hitomi-hub

## Rule

- 本プロジェクトの通常スクリーンショットの被写体は、Playwright browser toolで表示したブラウザ画面の画像とする。
- 通常確認のスクリーンショット・visual layoutは `kilo-playwright_browser_take_screenshot` / `kilo-playwright_browser_snapshot` などの Playwright browser画面を基準にする。
- 通常スクリーンショットでは OS foreground化は不要。`kilo-screen-capture.ps1`を使う必要はない。
- `kilo-screen-capture.ps1`は、Playwright browser画面画像で確認できない OS-level exceptionのためにだけ使う。その場合だけ foreground activationが必要。
- 確認スクリプトは browser executableを launchしない。Playwright browser toolの targetを直接使用する。
- `bin/kilo-fresh-browser.ps1` は Playwright browser toolへ switchするための policy guardとして扱い、browser executable launchを実行しない。
- `bin/kilo-screen-capture.ps1` は、フルスクリーン状態、OS-level window state、ブラウザ viewport 外、複数 monitorなど、Playwright browser画面内の画像だけでは確認できない事項の場合だけ使用する。
- OS full-screen captureを使用する場合、その画像は通常スクリーンショットではなく「例外的 OS full-screen 確認」として記録する。
- 被写体、取得方法、Playwright browser画面で確認できない例外理由を記録に明記する。
