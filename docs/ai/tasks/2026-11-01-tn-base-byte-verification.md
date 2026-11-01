# tn TN base byte verification

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: なし（次の項目へ）

## 意図 / 背景

TN thumbnail proxy の base path/content-type を、font/表示で紛れる文字列ではなく実バイト列で確定する。

## 調査結果 / 変更内容

### 実測で確定した TN path（正典規則）

- `common.js` の `url_from_url_from_hash(..., base='tn')` は subdomain を `String.fromCharCode(97 + gg.m(g)) + 'tn'`（`atn`/`btn`）にする。path は `real_full_path_from_hash`（`<last>/<prev two>/<hash>`）+ `.<ext>`。`gg.js` には `bigtn` の literal token は無い → TN base は呼び出し側の `dir`（format）で決まる。
- 実測（`galleryblock` 証拠 `scratch\hitomi-usage\galleryblock4294784.txt`・live proxy）で TN base は format 由来:
  - webp `77656270626967746e`（webpbigtn）→ `200`・`image/webp`
  - avif `61766966626967746e`（avifbigtn）→ `200`・`image/avif`
  - `746e626967746e`（tnbigtn）は base として存在せず `404`
  - `6174626967746e`（atbigtn）は TN base ではない。記録側の表示紛れ（KI-20261101-01）で混ざっていた
- TN に格納されるのは webp・avif ののみ。`image/tnbigtn` は不正な content-type。
- TN は `referer: https://hitomi.la/` 無しで `404`（browser UA でも 404、client UA + referer で `200`）。client は UA/referer を送っている。
- `ggset.go` の switch snapshot は live gg.js とずれる（live: bucket `1793491201/`・`case → 0`）。TN path は bucket を使わないので TN 検証には影響なし。shard URL は snapshot の bucket で `200`、live bucket では `404`（同一 hash）。→ `docs/ai/known-issues/2026-11-01-ggset-snapshot-drift.md`

### 変更

- `app/hitomi.go` `ThumbnailURL()`: TN base を format で決定（webp/avif ののみ）、subdomain を `a`/`b` + `tn`（`ggM`）へ変更。正典バイトを `tnBigtn` 定数と hex コメントで明示。
- `app/viewer.go` `imageContentType()`: `image/tnbigtn` 分岐を削除し、format の mime を返す（webp → `image/webp`、avif → `image/avif`）。呼び出し側を `imageContentType(format)` に統一。TN の既定 format は `imageFormat(cfg)`（既定 webp）。
- `app/ui.html`: `thumbFormat` sentinel（`\x74\x6e`）を削除し server format（`current.format`）を使用。
- テスト: `searchhtml_test.go` の TN path を実測バイトへ更新、`viewer_test.go` に avif TN proxy（`image/avif`）を追加、`hitomi_test.go` に TN base 判定（`tnbigtn` が base として出ないこと）を追加。

## Verification（2026-11-01）

- `go vet ./...`（`app`）→ clean
- `go test -count=1 ./...`（`app`）→ ok（TN 規則のテスト含む）
- build（`-trimpath -ldflags="-s -w"`）→ `scratch/dist/hub.exe`。完成フォルダへコピー
- live proxy 検証（exe 単体・`127.0.0.1:8731`・呼出し 3・間隔 1000 ms 以上）:
  - `/api/thumb/21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065`（zero set）→ `200` `image/webp` 21416 bytes（`atn` + `77656270626967746e`）
  - `/api/thumb/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89`（one set）→ `200` `image/webp` 55320 bytes（`btn` + `77656270626967746e`）
  - `/api/thumb/...?format=avif` → `200` `image/avif` 16115 bytes（`atn` + `61766966626967746e`）
  - `746e626967746e` path は `404`（base ではないことを live で確認）
- `python -m pytest tests/ -q`（Common_Operations）→ 109 passed

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（KI-20261101-01）
- [ ] `backlog.md` の完了項目を削除（該当なし）
