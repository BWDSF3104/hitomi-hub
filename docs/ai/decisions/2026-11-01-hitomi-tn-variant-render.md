# hitomi work page TN variant rendering

- Date: 2026-11-01
- Status: implemented

## Decision

Viewer は元サイトの work page と同じ hitomi TN variant を正典表示にする。local file は「app 機能の marker / fallback」であって、remote entry を隠す filter ではない。

## TN path

hitomi TN path は:

```text
https://<{a|b}tn>.gold-usergeneratedcontent.net/<format><variant>/<last hex char>/<prev two hex chars>/<hash>.<format>
```

- `format` は TN に格納される形式のみ（webp / avif）
- `galleryinfo.files[].name` の ext（png / jpg）は TN path に使わない
- work page thumbnail variants:
  - DPR 1: `smallsmalltn`
  - DPR > 1: `smalltn`
  - search card cover: `bigtn`

## Proxy order

- `/api/thumb/`: TN first. TN failure -> local file -> full image proxy.
- `/api/image/`: local full file -> remote full image -> TN `bigtn` fallback.

This keeps the viewer as an extension of hitomi: remote entries remain visible through hitomi TN, local files remain an app feature.
