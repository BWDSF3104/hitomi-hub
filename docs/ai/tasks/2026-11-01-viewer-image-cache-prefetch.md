# Viewer image cache and background full prefetch

**Status**: 完了
**Date**: 2026-11-01
**Backlog**: P2

**Next Action**: server-side full image cache checkpoint を実装し、テスト・ビルド・画面確認・コミット

## 意図 / 背景

- 現状の画像ファイルキャッシュは SQLite の `scan_cache` と viewer 内 `infoCache`、およびブラウザ HTTP cache だけ。server は取得した full image bytes を disk に保存しない。
- `download` 済みファイルがある gallery は local file から表示できるが、view proxy の remote fetch は cache miss のたびに hitomi storage/TN を再取得する。
- 応答性と操作性のため、閲覧画面に入ったら current page を即表示し、その gallery の full images を background で persistent cache / browser cache に読み込む。

## 調査結果 / 変更内容

- `viewer` に `cacheRoot` を追加し、full image proxy の lookup order を `local downloaded file > persistent cache > remote full > TN fallback` に変更する。
- full storage fetch 成功時のみ `cache/images/<hash>.<format>` に永続保存する。TN fallback は品質が落ちるため persistent full cache に保存しない。
- `imageProxy` に `x-cache: local|cache|miss|tn` を追加し、cache hit の再 fetched をテストで検証する。
- UI `showPage()` は current page image を `img.src` に設定した直後に `startFullPrefetch()` を non-blocking で開始する。
- UI prefetch は `AbortController` と concurrency 2 を使い、current page、current strip group、残 page の順で full image URL を background fetch する。route/gallery 変更時は旧 prefetch を abort する。

## Verification

### Checkpoint 1: persistent cache

- `python -m pytest tests/ -q`（Common_Operations 正典テストは本 checkpoint で変更なし）
- `go vet ./...` → no output, passed。
- `go test -count=1 ./...` → `hitomi-hub/app 29.917s`, passed。Added tests: normalize/safe cache path, persistent cache hit, TN fallback not persisted, UI background prefetch.
- `go build -trimpath -ldflags "-s -w" -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → passed。
- 画面確認 `http://127.0.0.1:8731/#/g/2648601/page/2`: `prefetch-status` = `full prefetch 7/27`, current image `naturalWidth=2059`、`naturalHeight=2880`、note `cdn/TN（未ダウンロード）`, thumbs `27`。スクショ: `scratch\hitomi-usage\image_cache_background_prefetch.png`。
- Server cache verify: `cache/images` に `.avif` files を確認。既存 cache file `f82b691eb39344f296449aaca795d7c649e81127fc2c66be8a5591e15dc75427` への 2 次 fetch で `x-cache=cache` を確認。

### Checkpoint 2: direct background full-image fetch

- UI は `current.files` の `local_formats` と `hasavif` から full image URL を組み、背景 prefetch では `/api/gallery/<id>/page/<n>` JSON を per-page で呼ばない。
- `go vet ./...` → no output, passed。
- `go test -count=1 ./...` → `hitomi-hub/app 30.059s`, passed。Test updated to require `entrySource(entry)`、`AbortController`、`PREFETCH_CONCURRENCY = 2`、nonblocking `startFullPrefetch()`。
- `go build -trimpath -ldflags "-s -w" -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → passed。
- 画面確認 reload 後: `prefetch-status` = `full prefetch 27/27`, current image `naturalWidth=2059`、`naturalHeight=2880`, note `cdn/TN（未ダウンロード）`, thumbs `27`。スクショ: `scratch\hitomi-usage\image_cache_background_prefetch_direct.png`。
- Performance resource check: reload 後の背景 prefetch resource は `/api/image/<hash>?format=avif&gallery=2648601` だけ。`/api/gallery/2648601/page/*` JSON は reload 後の prefetch で確認せず。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
