# Image rate gate and prefetch concurrency checkpoint

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: test, screen verify, commit

## 意図 / 背景
- Viewer prefetch concurrency は `2` だが、server side hitomi client の全 remote call が `1000 ms` gate を通していたため、image download throughput が gate に支配されていた。
- search・galleriesindex・gg の policy を 1000 ms に維持したまま、image request だけ低速化しすぎない lower interval を分離する。

## 調査結果 / 変更内容
- `Client` に `image_min_interval_ms` を追加し、image proxy の remote request は `getImage()` を使う。
- default image interval は `250 ms`。一般 data call は `1000 ms` を維持。
- rate gate は `sync.Mutex` で protected last time を使い、concurrent handler で last time が壊れない。
- UI background prefetch concurrency を `4` に上げる。

## Verification
- `go vet ./...`: pass.
- `go test .`: pass, `23.338s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`: pass.
- Test asserts `defaultHitomiConfig().ImageMinIntervalMS == 250` and image gate uses `imageLast` separately from `last`.
- Screen verify at `http://127.0.0.1:8731/?x=concurrency4#/g/4295693/page/1-2`:
  - `PREFETCH_CONCURRENCY`: `4`
  - status `full prefetch 15/15 tn 0/15` for gallery already persisted in image cache.
  - displayed `img.dataset.fullCache`: `cache`
  - displayed `img.src`: `blob:...`
  - `fullObjectUrls` count: `2`
- Screen verify TN response remains `no-store` with lower image gate:
  - fetch `/api/image/6cf66a705f1c527c499dbd5b66fb07e506ed72cdd723b7747f1161517828667e?format=png&gallery=4295693`
  - `x-cache`: `tn`
  - `cache-control`: `no-store`
- Screenshot: `scratch\hitomi-usage\image_gate_concurrency4.png`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
