# Manual full image retry pass criteria implementation

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: Set exact pass criteria, correct the UI retry button state, update tests, and run canonical verification.

## 意図 / 背景

Manual full-image retry の合格条件を、既存の `error` のみではなく `error` と `no_full` の両方を処理する条件として確定し、テストと実装へ反映する。ボタン状態は `errorDone` だけで判断せず、`errorDone > 0` または `noFullDone > 0` のどちらかで有効になる必要がある。既存の automatic retry、display source 判定、TN display fallback 禁止、generation guard、abort guard は維持する。

## 調査結果 / 変更内容

合格条件:
- `manualRetryFailedFullImages()` は `sourceStatusByEntry` が `error` または `no_full` の entry だけ対象にする。
  ```js
  if (sourceStatusByEntry.get(key) !== "error" && sourceStatusByEntry.get(key) !== "no_full") continue;
  ```
- `#retry-failed` は `current.id` があり、`manualRetryRunning` でなく、`Number(counts.errorDone) > 0` または `Number(counts.noFullDone) > 0` のとき有効。
  ```js
  button.disabled = manualRetryRunning || !current.id || !(Number(counts.errorDone) > 0 || Number(counts.noFullDone) > 0);
  ```
- `no_full` は `X-Cache: no_full` の storage miss で、natural production `error` ではない。`error` は `!r.ok` かつ `x-cache` なし、または fetch exception の場合だけ。
- `statusIsViewerDisplaySource()` が `error`, `no_full`, `tn` を display source から除外する既存判定を維持する。
- `updatePrefetchCounts(..., allowRecount = false)` の default を維持し、manual retry だけ `allowRecount = true` で old status を decrement し new status を count する。
- actual UI 確認は canonical build から起動した server 上で行い、controlled route による network failure confirmation は production natural failure と区別して記録する。
- full actual pixels visual pass は、UI-openable gallery が existing decodable cache hash を含み、server response が `cache`/`local`/remote full fetch、display source が `objectURL`/`local-direct`、`img.naturalWidth > 0`、normal OS screen capture で表示が確認できる場合に成立。
- race detector proof は cgo/gcc が可用な環境で `go test -race -count=1 .` を通すこと。この環境では mutex structure test と concurrent handler/cache test が local pass で、data-race proof は未完了。

変更:
- `app/viewer_test.go` の exact assertion を combined condition に更新。
- `app/ui.html` の `renderRetryButtonState()` を combined condition に更新。
- manual retry target condition は test-first assertion で確定済み。

## Verification

- Code/test pass:
  - `go vet . && go test -count=1 -timeout 300s .` in `app` passed: `ok hitomi-hub/app 35.057s`.
  - Canonical build passed:
    `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
  - `python -m pytest tests -v` from `S:\Documents\Kilo_Commons\hitomi-hub` passed: `25 passed in 2.75s`, including `tests/test_canonical_build_freshness.py`.
  - `go env CGO_ENABLED` is `0`, so `go test -race -count=1 .` remains blocked locally. The race detector was not used for data-race proof; only mutex structure and existing concurrent handler/cache tests pass.
- Canonical actual UI server:
  - Canonical exe was rebuilt from the modified source.
  - Canonical server was started on port `8731` from `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh`.
  - Direct target baseline for target gallery `4299126` target hash
    `b17f157ac25f3f066f9d0b3707a98040defc90cf8a5847da550b9a0240fb3c53`
    returned `404` with `X-Cache: no_full`.
  - Direct cache baseline for existing decodable hash
    `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3`
    returned `200`, `X-Cache: cache`, `content-type: image/avif`.
- Controlled actual UI `errorDone > 0` behavior pass:
  - Fresh isolated UI tab used the served UI from canonical server.
  - `/api/image/**` network responses were controlled to return `500` without `x-cache`.
  - Before click:
    - `errorDone: 1`, `noFullDone: 0`, `total: 37`, `#retry-failed` enabled.
    - `prefetch-status`: `actual pixels 0/37 | local direct 0/37 | objectURL cache 0/37 | no full 0/37 | TN display fallback disabled | error 1/37`
  - At click:
    - `manualRetryRunning: true`
    - `manualRetryingKeys` size `1`
    - first key `4299126|179adedebafe2b771e1d333670910001eca0a6ea096908d0e9f08bf4e088602e|avif`
  - After completion:
    - `errorDone: 37`, `noFullDone: 0`, `manualRetryRunning: false`, `manualRetryingKeys` size `0`, `#retry-failed` enabled.
    - `prefetch-status`:
      `actual pixels 0/37 | local direct 0/37 | objectURL cache 0/37 | no full 0/37 | TN display fallback disabled | error 37/37`
  - Normal OS screen capture, taken after bringing the Playwright browser window to the front:
    - `S:\Documents\Kilo_Commons\Common_Operations\scratch\manual-retry-pass-criteria-implementation\actual-ui-controlled-error-20261102.png`
    - `I:\マイドライブ\kilo\2026-11-02-manual-retry-pass-criteria-implementation\actual-ui-controlled-error-20261102.png`
- Controlled actual UI full actual pixels pass:
  - Fresh isolated UI route opened gallery `999999` with existing decodable cache hash
    `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3`.
  - Server response for that hash returned `X-Cache: cache`, `content-type: image/avif`.
  - UI display state:
    - `sourceStatusByEntry.get(key)`: `cache`
    - `sourceTypeByEntry.get(key)`: `objectURL`
    - `displayPassByEntry.get(key)`: `true`
    - `img.naturalWidth`: `1295`
    - `img.naturalHeight`: `1812`
    - `prefetch-status`: `actual pixels 1/1 | local direct 0/1 | objectURL cache 1/1 | no full 0/1 | TN display fallback disabled`
  - Normal OS screen capture, taken after bringing the Playwright browser window to the front:
    - `S:\Documents\Kilo_Commons\Common_Operations\scratch\manual-retry-pass-criteria-implementation\actual-ui-controlled-cache-20261102.png`
    - `I:\マイドライブ\kilo\2026-11-02-manual-retry-pass-criteria-implementation\actual-ui-controlled-cache-20261102.png`
  - This is a controlled UI confirmation because the gallery route is synthesized. It is not production target gallery `4299126` full pixels.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
