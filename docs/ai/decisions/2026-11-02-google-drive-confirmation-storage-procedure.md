# Google Drive confirmation result storage procedure

Status: active.

## Storage location

Use the user interaction folder:

- `I:\マイドライブ\kilo`

For each confirmation day, create a date folder:

- `I:\マイドライブ\kilo\YYYY-MM-DD`

Example:

- `I:\マイドライブ\kilo\2026-11-02`

The confirmation result is stored under the date folder. Each confirmation result has:

1. screenshot file:
   - `YYYY-MM-DD` folder
   - `<confirmation-topic>-YYYY-MM-DD.png`

2. procedure text file:
   - `YYYY-MM-DD` folder
   - `<confirmation-topic>-confirmation-YYYY-MM-DD.txt`

Example:

- `I:\マイドライブ\kilo\2026-11-02\search-button-initial-display-screen-20261102.png`
- `I:\マイドライブ\kilo\2026-11-02\search-button-initial-display-confirmation-20261102.txt`

## Required contents

The procedure text must be written in clear Japanese.

It must include:

1. Date.
2. Screenshot file name.
3. Purpose of the confirmation.
4. Operation procedure.
5. Pass criteria.
6. Actual measured values or observed result.
7. Whether the confirmation passed.
8. Fail conditions.
9. Code changes, if any.
10. Verification commands and results.

## Required strict confirmation conditions

Google Drive confirmation results are valid only when the confirmation used strict screen confirmation conditions:

- fresh isolated browser tab or fresh browser context
- no reuse of stale tab or stale route
- server direct baseline before UI confirmation
- UI network response must match actual server response
- no stale cached response such as `total: 0`, `content-length: 84`, missing `X-Search-Timing`
- confirmation is measured before `window.resize`
- resize may be used only to prove a bug, not to pass the confirmation
- existing feature regression must not be broken by the change under confirmation
- duplicate `id="query"` and duplicate form ids are fail conditions

For initial display size:

- `.side` actual width must match the CSS contract before resize
- `#search` controls must not wrap
- `#results` card count must match actual server `ids.length`
- `.card` background height must be bounded by `--search-card-height`

For fullscreen image size:

- `#reader` fullscreen display area must fill fullscreen viewport minus explicit padding
- fullscreen image display box must fill the available fullscreen cell
- `object-fit: contain` must preserve aspect ratio
- fullscreen keyboard `ArrowLeft` and `ArrowRight` must move pages when focus is not `INPUT`, `BUTTON`, or `SELECT`

## Playwright screenshot storage

The Playwright MCP screenshot tool is restricted to its allowed output root, so save the screenshot first to:

- `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`

Then copy it to the Google Drive date folder using PowerShell:

```powershell
Copy-Item -LiteralPath 'C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\<file>.png' -Destination 'I:\マイドライブ\kilo\YYYY-MM-DD\<file>.png'
```

Use the text procedure file in the same Drive date folder. Do not save confirmation screenshots into the project repository.

## Memory Bank recording rule

The project Memory Bank should record the Google Drive output folder and file names in the task Verification section. It should not store the screenshot binary in the git repository.

Use the task file Verification section as the canonical project record, and use the Google Drive procedure text as the user-visible confirmation result.
