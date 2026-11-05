# 正典 CDP/click tool が本項目の実測で #search-submit を確定できない

**Status**: 未解決（本項目で記録のみ・正典 tool 変更承認外）

## 実測

- `kilo-cdp-measure.ps1` を port `9344` で呼ぶと `CDP list is unavailable on port 9344`。
- `kilo-fresh-browser.ps1` の gesture は window center X を固定で使う（`ClickPercentX` を X に反映しない）。
- strict criteria の target `1280x900` で `#search-submit` は `x=55`、window center x は `640`。したがって OS-level click を `#search-submit` へ届ける canonical gesture が成立しない。

## 今回採用した確認機構

- strict criteria項目2の「`#search-submit` click または `form#search` submit must fire」に基づき、既存 CDP trusted input click を actual UI event として使用。
- `#search-submit` click、`#reader` focus後 `r` key、`ArrowRight` final page key、`#lists-refresh` click、`data-reset="read"` click、`data-reset="viewed"` clickは `kilo-playwright_browser_click` / `kilo-playwright_browser_press_key` の CDP input event。
- `kilo-screen-capture.ps1` 正典 capture と `kilo-drive-confirm.ps1` 正典 confirmation record を使用。visual layout は capture + CDP visual layout measurement（`document.elementFromPoint(700, 110)`、`.text`/`.badge`/`.meta` bounding box、`#lists-refresh` visible）で一致。

## 次の対応（段階3）

- 正典 `bin/kilo-fresh-browser.ps1` の `ClickPercentX` を X 位置へ反映する修正、または正典 CDP trusted input click helper を別途承認して追加する。
- 現行の実測では合格条件を満たすが、正典 tool の canonical gesture limitation は未解決として残す。
