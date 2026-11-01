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
- `/api/image/`: local full file -> remote full image -> TN variant fallback `bigtn`, `smallbigtn`, `smalltn`, `smallsmalltn`; `hasavif` TN avif優先、webp TN を最終 fallback に残す。
- `/api/gallery/{id}/page/{page}` は remote source で `hasavif` entry に `avif` format を使う。local file は config format を先に確認する。

This keeps the viewer as an extension of hitomi: remote entries remain visible through hitomi TN, local files remain an app feature.

## Scene and format rules

`scene_indexes` is part of hitomi work page model. UI item count uses `scene_indexes.length - 1` when present; otherwise it uses `files.length`. Each scene item maps to `sceneIndexes[i] + 1`. A scene block is shown as one strip item and its size is shown as a badge when `size > 1`.

TN format selection uses hitomi `hasavif`: `hasavif` truthy uses `avif`; otherwise UI uses server format, default `webp`. `hasavif` in JSON is numeric, so `FileEntry.HasAVIF` is `int`.
