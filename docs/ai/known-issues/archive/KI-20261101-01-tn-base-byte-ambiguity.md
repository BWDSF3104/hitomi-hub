# TN base bytes can be misread by font display

**Date**: 2026-11-01
**Status**: 解決（2026-11-01・実測で確定）

## 症状

TN thumbnail proxy tests pass, but live zero-hash `?format=tn` can fail because the TN base string used in `ThumbnailURL()` does not match the saved exact-byte TN path.

## 原因

`tnbigtn`, `webpbigtn`, `avifbigtn` が表示上紛れる。実測（exact-byte）で確定した TN base は format 由来:

- webp `77656270626967746e`（webpbigtn）→ `200`・`image/webp`
- avif `61766966626967746e`（avifbigtn）→ `200`・`image/avif`
- `746e626967746e`（tnbigtn）は TN base として存在せず `404`
- `6174626967746e`（atbigtn）は TN base ではない（記録側の表示紛れ）

subdomain は `String.fromCharCode(97 + gg.m(g)) + 'tn'`（common.js・実測で `atn`/`btn`・`tn` 単独も 200）。TN に格納されるのは webp・avif ののみ。

## 影響

- Zero TN path が CDN 404。UI thumbnail が読み込めない。
- `image/tnbigtn` は不正な content-type。

## 修正（実施済み）

- `app/hitomi.go` `ThumbnailURL()`: TN base を format で決定、subdomain を `a`/`b` + `tn`（`ggM`）へ。正典バイトを `tnBigtn` 定数と hex で明示。
- `app/viewer.go` `imageContentType()`: `image/tnbigtn` 分岐を削除（webp → `image/webp`、avif → `image/avif`）。
- TN は `referer: https://hitomi.la/` が無いと 404（UA だけでは不可）。client は UA + referer を送る。
- live 検証: zero set（`atn`）/ one set（`btn`）/ avif base すべて 200。
