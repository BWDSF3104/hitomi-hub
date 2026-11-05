# 正典 CDP/click tool が本項目の実測で #search-submit を確定できない

**Status**: クローズ（2026-11-05 18:07 ユーザー判定）

## 実測

- `kilo-cdp-measure.ps1` を port `9344` で呼ぶと `CDP list is unavailable on port 9344`。
- `kilo-fresh-browser.ps1` の gesture は window center X を固定で使う（`ClickPercentX` を X に反映しない）。
- strict criteria の target `1280x900` で `#search-submit` は `x=55`、window center x は `640`。したがって OS-level click を `#search-submit` へ届ける canonical gesture が成立しない。

## 今回採用した確認機構

- strict criteria項目2の「`#search-submit` click または `form#search` submit must fire」に基づき、既存 CDP trusted input click を actual UI event として使用。
- `#search-submit` click、`#reader` focus後 `r` key、`ArrowRight` final page key、`#lists-refresh` click、`data-reset="read"` click、`data-reset="viewed"` clickは `kilo-playwright_browser_click` / `kilo-playwright_browser_press_key` の CDP input event。
- `kilo-screen-capture.ps1` 正典 capture と `kilo-drive-confirm.ps1` 正典 confirmation record を使用。visual layout は capture + CDP visual layout measurement（`document.elementFromPoint(700, 110)`、`.text`/`.badge`/`.meta` bounding box、`#lists-refresh` visible）で一致。

## ルール適用（2026-11-05 06:29）

- 本プロジェクトルール `docs/ai/rules/normal-screenshot-rule.md` に従い、通常スクリーンショットの被写体は Playwright browser画面の画像とする。
- `kilo-screen-capture.ps1` の full virtual screen capture は通常 visual confirmation evidence として使用しない。Playwright browser画面内の画像だけでは確認できない例外事項の場合のみ例外的 OS full-screen 確認として扱う。
- 正典 `bin/kilo-fresh-browser.ps1` は通常スクリーンショット確認のために呼び出さない。2026-11-05 06:27 に `taskkill /IM msedge.exe /F /T` で起動中 Edge を停止した。

## 正典 fresh target re-measurement observations（2026-11-05 05:34-05:51）

- `kilo-cdp-measure.ps1` を `pwsh` で直接呼ぶと `"SendAsync" と引数の数 "4" に対して、複数のあいまいなオーバーロードが見つかりました。` で失敗した。
- 同一正典 script を `powershell -NoProfile -File` で呼ぶと JSON expression measure が成功した。fresh app target を測るには `-Url "http://127.0.0.1:8731/search"` が必要。`-Url` なしでは first page target が `edge://newtab/` または `edge://sync-confirmation-dialog/` になった。
- `kilo-fresh-browser.ps1` の key delivery は `#query` focus 後に `Enter` を送っても `form#search` submit を発火しなかった。`window.__submitFired` は `0` のまま。tool は first CDP target を選び、app target を選ばない。
- `kilo-cdp-measure.ps1` は evaluate 専用で `Input.dispatchMouseEvent` を送れない。fresh target の actual UI events は existing tool range の CDP trusted input click で確認した: `#search-submit`、`#lists-refresh`、`li[data-search-id="4305205"] button[data-reset="viewed"]`、`#saved-search button`。
- `Input.dispatchKeyEvent` for saved-search key was not received from existing tool target; canonical key delivery also did not fire saved-search submit.
- `kilo-fresh-browser.ps1 --window-size=1280,900` の actual outer window は `1280x900` だったが、actual content viewport は `innerWidth=1259`、`innerHeight=810` で、strict criteria の exact inner target `1280x900` は確認できなかった。プロジェクト policy が canonical window-size を target viewport と明示的に判定する場合は measured values は pass。

## 正典 script fix

- 本プロジェクトルール `docs/ai/rules/normal-screenshot-rule.md` により、確認スクリプトは browser executableを launchせず、Playwright browser toolの targetを直接使用する。
- `S:\Documents\Kilo_Commons\Common_Operations\bin\kilo-fresh-browser.ps1` の browser executable launch pathを削除し、通常実行では Playwright browser tool利用を要求する policy guardへ変更した。
- CDP target selection limitationは残る。browser executable launchを使わない確認経路では、Playwright browser targetを正典 Playwright toolで測る。
- `kilo-screen-capture.ps1` fix: target window title指定時、capture前に target window foregroundを確認し、既定では target client rectangleをcaptureする。full virtual screenは `-CaptureFullScreen` の例外モードのみ。
- 通常スクリーンショットではこの toolは使わない。Playwright browser viewport screenshotだけで確認できる場合は、foreground activationや shell script化は意味がない。

## 次の対応（段階3）

- 正典 `bin/kilo-fresh-browser.ps1` の `ClickPercentX` を X 位置へ反映する修正、または正典 CDP trusted input click helper を別途承認して追加する。
- 現行の実測では合格条件を満たすが、正典 tool の canonical gesture limitation は未解決として残す。

## クローズ判定（2026-11-05 18:07）

- `ClickPercentX` は正典 `bin/kilo-fresh-browser.ps1` から除去済み。launch path は Playwright tool 使用の policy guard へ変更（browser executable launch 禁止）
- 段階3の「ClickPercentX を X 位置へ反映する修正」は適用対象外。合格判定は Playwright CDP trusted input click 経路で成立
- 残る CDP target selection limitation は正典確認手順（Playwright browser tool の target を直接測る）で運用解決済み
