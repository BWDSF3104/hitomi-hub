# P3 #12 は正規 cache acquisition として実装

## 決定

`#12` は「まとめ download」としてではなく、**cache の正規買い出し処理**として実装する。

## 動作

- Server-side endpoint を使う。
- UI の button は acquisition として扱う。download という名前は使わない。
- Preconfigured acquisition folder 内に gallery title folder を作成する。
- Cache file は copy する。Move はしない。
- Copy 後の filename は page number を使う。
  - 例: `001.jpg`, `002.png`
  - Hash を filename にしない。
- Existing title folder がある場合、上書きするか確認を出す。
- Server は UI の explicit confirmation 後に `overwrite=true` を受け取る。

## Source rules

- Source is server-side persistent cache.
- Cache file path remains unchanged:
  - `cache/images/<hash>.<format>`
- Missing cache file is not silently skipped unless the acquisition mode is copy-only.
- If acquisition mode includes normal acquisition, missing cache entries are fetched through normal rate-gated image acquisition before copy.

## Path rules

- Acquisition folder must be inside app-writable root or explicitly approved preconfigured folder.
- App must not write outside its own allowed folder.
- Page-order source is `pageorder` when available. Fallback is `current.files` display order.

## UI rules

- UI must show:
  - acquisition button
  - progress
  - abort
  - overwrite confirmation
  - per-page result
- UI should display per-entry status such as `copied`, `skipped`, `missing cache`, `overwrite`, `error`.

## Verification

- Unit tests should assert copy behavior, not move behavior.
- UI tests should assert acquisition button, overwrite confirmation, progress, abort, and handler wiring.
- Screen verify should confirm:
  - title folder is created
  - page-number filenames are created
  - existing cache source remains
  - existing title folder triggers confirmation before overwrite
  - acquisition does not call network when cache exists