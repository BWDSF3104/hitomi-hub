# 手動フル画像リトライの修正方法と合格条件

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: test-first correct the `error` or `no_full` target condition, run canonical verification, restart the canonical server, then confirm actual UI `no_full` processing and save the screen confirmation to Google Drive.

## 意図 / 背景

現状、フル画像取得失敗時には automatic retry がない。`prefetchedEntries` が処理済み key を保持するため background requeue されず、`statusIsViewerDisplaySource()` も `error` を display source から除外する。既存の自動動作と display fallback の挙動に最小限の影響を与え、UI ボタン操作から任意タイミングで manual retry できるようにする。

## 調査結果 / 変更内容

- `prefetchPageImage()` は fetch 失敗時に status を `error` にして count するが、retry はしない。
- `loadFullImage()` は exception 時に `error` を store/count するが、retry はしない。
- `updatePrefetchCounts()` は `prefetchedEntries.has(key)` が true の場合 early return するため、manual retry で status recount を許可しないと同じ key の再取得を count できない。
- `statusIsViewerDisplaySource()` は `error`, `no_full`, `tn` を display source から除外する。

修正方法:
- `#retry-failed` ボタンを `#prefetch-status` 付近に追加。
- manual retry は `manualRetryFailedFullImages()` のみ。
- `sourceStatusByEntry.get(key)`が `error`または`no_full`の entry を対象。

### Test-first correction decision

- Existing live blocker: the served UI guard skips every entry, including `no_full`, so actual `#retry-failed` click processes `passCount: 0`.
- Required UI guard:
  ```js
  if (sourceStatusByEntry.get(key) !== "error" && sourceStatusByEntry.get(key) !== "no_full") continue;
  ```
- `app/viewer_test.go` must assert this combined condition before implementation.
- `note: "no full image: <page>"` is target page, not failed-entry count. `prefetch-status` remains `no full <count>/<total>`.
- Pass criteria are recorded in `docs/ai/decisions/2026-11-02-manual-retry-no-full-target-condition.md`.
- `manualRetryingKeys` で re-entrancy を防ぐ。
- `updatePrefetchCounts(status, entry, galleryId = current.id, allowRecount = false)` に optional recount を追加。
- default は false で既存動作を維持。
- manual retry 経路だけ `allowRecount = true` で old count を decrement してから new status を count。
- 既存の `showGeneration`, `openGeneration`, `renderGeneration`, `backgroundGalleryId`, `AbortSignal` ガードを維持。
- 現在表示中の entry は `loadFullImage()`、非表示 entry は `prefetchPageImage()` を manual retry 経路で呼ぶ。

合格条件:
- `S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\decisions\2026-11-02-manual-full-image-retry-criteria.md` を参照。

## Verification

- Manual retry implementation and tests pass, but strict UI confirmation is blocked until a reproducible actual UI `error` entry is found.
- Direct server baseline result:
  - `http://127.0.0.1:8731/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=error` returns `404 no_full`, not `error`.
  - `format=tn` on the same hash returns `404 no_full`, not `error`.
  - Direct `/api/gallery/4294614` returns lowercase hex hashes, so `entryFormat()` does not create `sourceStatusByEntry` error.
- Recorded blocker in `known-issues/2026-11-02-manual-full-image-retry-error-entry-not-guaranteed.md`.

### Latest canonical build UI confirmation

- Fresh isolated browser tab:
  - `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2`
- Actual UI state:
  - `note`: `no full image: 2`
  - status: `no full 37/37`
  - `errorDone`: `0`
  - `retry-failed` was enabled because `Number(counts.noFullDone) > 0`.
- Actual UI click on `#retry-failed` executed `manualRetryFailedFullImages()`.
  - After completion, `manualRetryRunning` was `false` and `manualRetryingKeys` size was `0`.
  - Counts remained `noFullDone: 37`, `errorDone: 0`.
- UI network requests for `/api/image/...` all returned `404`.
- Target page 2 hash:
  ```text
  b17f157ac25f3f066f9d0b3707a98040defc90cf8a5847da550b9a0240fb3c53
  ```
  - UI request and direct server baseline both returned `404` with `x-cache: no_full`.
- Direct served UI from canonical rebuilt server includes:
  ```js
  if (Number(counts.noFullDone) > 0) button.disabled = false;
  ```
- Browser screenshot:
  - `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\p3_fresh_tab_after_manual_retry.png`

Remaining strict UI blocker for the previous `errorDone > 0` entry is not relevant to the corrected `no_full` processing pass. A normal OS physical screen capture was obtained for the `no_full` state. Full image actual pixels remain `0/37` because the server still returns `no_full`.

### Google Drive confirmation result

- Test-first failure before implementation:
  - `go vet . && go test -count=1 -timeout 300s .` failed in `app` with `TestEmbeddedUIHasManualFullImageRetryButton` requiring the combined `error` or `no_full` condition.
- After implementation in `app/ui.html`:
  - `go vet . && go test -count=1 -timeout 300s .` in `app` passed: `ok hitomi-hub/app 34.932s`.
  - Canonical build passed:
    `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
  - `python -m pytest tests -v` from `S:\Documents\Kilo_Commons\hitomi-hub` passed: `25 passed in 2.87s`, including `tests/test_canonical_build_freshness.py`.
  - Canonical server was restarted from the rebuilt exe.
- Fresh isolated browser tab:
  - `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2`
- Direct server baseline before UI confirmation for target page 2 hash
  `b17f157ac25f3f066f9d0b3707a98040defc90cf8a5847da550b9a0240fb3c53`:
  - `HTTP/1.1 404 Not Found`
  - `X-Cache: no_full`
  - `Cache-Control: no-store`
  - `Content-Length: 0`
- Fresh UI state before actual UI click:
  - `note`: `no full image: 2`
  - `prefetch-status`: `actual pixels 0/37 | local direct 0/37 | objectURL cache 0/37 | no full 32/37 | TN display fallback disabled`
  - all `sourceStatusByEntry` statuses were `no_full`
  - corrected target condition selected `32` entries: `correctPassCount: 32`
- Actual UI click on `#retry-failed`:
  - During click: `manualRetryRunning: true`, `manualRetryingKeys` size `1`, first key `4299126|4a47023c6530b4e037011dcb8d2060c1746993e7d752c2a7e65bf0ae92657b03|avif`.
  - After completion: `manualRetryRunning: false`, `manualRetryingKeys` size `0`, `no full 37/37`, `errorDone: 0`, `correctPassCount: 37`.
  - UI network requests for `/api/image/...` were made for processed `no_full` entries and returned `404` with `x-cache: no_full`.
- Normal OS physical screen capture:
  - Physical screen: `3840x2160`
  - Scratch: `S:\Documents\Kilo_Commons\Common_Operations\scratch\manual-full-image-retry-screen-20261102.png`
  - Google Drive screenshot: `I:\マイドライブ\kilo\2026-11-02\manual-full-image-retry-screen-20261102.png`
  - Google Drive confirmation text: `I:\マイドライブ\kilo\2026-11-02\manual-full-image-retry-no-full-confirmation-20261102.txt`
- Visual result: fresh browser page showed `retry failed` enabled, `no full image: 2`, and `actual pixels 0/37`.
- Pass result: `no_full` entries are selectable, actual UI click processed them, and `/api/image/` network requests were made. Full image actual pixels remain `0/37` because the server still returns `no_full`.

## Additional investigation (2026-11-02)

- The `no_full` manual retry pass was confirmed on canonical server `pid 5152`.
- Full image actual pixels remain `0/37` for target gallery `4299126`.
- Existing full-image cache files can provide actual pixels only when a UI-openable gallery contains one of those hashes. Sample `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3` is decodable and served with `x-cache: cache`, but it is not in target gallery `4299126`.
- A controlled actual UI `error` path was confirmed by intercepting `/api/image/...` responses with `500` and no `x-cache`:
  - fresh UI tab `http://127.0.0.1:8731/?newtab=1#/g/4299126/page/1-2`
  - before click: `errorDone: 1`, `#retry-failed` enabled
  - actual UI click processed all `37` error entries
  - after click: `errorDone: 37`, `manualRetryRunning: false`, `manualRetryingKeys` empty
  - `prefetch-status` showed `error 37/37`
- This is controlled UI network failure evidence, not a natural direct-server production failure.

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
