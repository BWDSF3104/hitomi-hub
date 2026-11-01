# viewer の mini strip は 2 枚目以降の thumbnail を表示できない

**Date**: 2026-11-01
**Status**: 解決（検索 card は cover だけ、reader strip は local-first proxy で表示）

## 現象

search card と reader の mini strip で、1 枚目（cover）だけ表示され、2 枚目以降が空（alt 数字のみ・proxy 502）。

## 原因 1: hitomi の TN storage には card に出した image の分しかない

実測（gallery `4294784`・`docs/ai/tmp/card_thumb_probe.py`）:

- `files[0]` = `a39bb269…` → TN `200`
- `files[1]` = `58e06942…`、`files[2]` = `d99af925…`、`files[3]` = `1d4a9d9b…` → TN `404`
- hitomi 自身の card markup（`Common_Operations\scratch\hitomi-usage\galleryblock4294784.txt`）は `dj-img1`（cover）と `dj-img2`（`0a6e5e1e…`）の 2 枚のみ。`0a6e5e1e…` は `files[0..3]` に無い hash（hitomi が TN として cache 済みの image）

TN は hitomi が thumbnail として実際に表示した image にのみ存在（reader は full image を見るので任意 image の TN は無い）。viewer が `files[i]` で TN を要求しても、cover 以外は 404 になる。

## 原因 2: page model の不一致

- hitomi: gallery の page = image 25 件（`results_per_page = 25` は検索結果の page。gallery block は page ごとに image を並べる）
- viewer（`app/ui.html`）: `current.count = g.files.length`、mini strip は `current.files.slice(0, 20)` を page 1..20 とし、prev/next は file を 1 件ずつ送る。`read_state.last_page` はこの file index 基準

`app/viewer.go` の `searchResult()` も `info.Files[i]`（i = 0..3）を `page` として `src` を返している → hitomi の page thumbnail（page 2 = `files[25]`）ではない。

## 修復案（未実施・承認が必要）

- mini strip は TN が確定している cover のみ表示、または local file から thumbnail を生成（local-first の image がある gallery は自前で缩略图を出せる）
- page model を hitomi と揃える（25 image = 1 page）か、UI の表示を「image index」に改める（`read_state.last_page` の意味が変わるため decisions への記録が必要）

## 実装済み（2026-11-01）

- 検索 card の thumbnail は cover だけ。`app/viewer.go` の `searchResult()` は `pages` を返さず、`thumbnail` に cover hash と `gallery=<id>` を渡す。`app/ui.html` の results card から mini strip を削除。
- reader strip は cover/local だけを描画。`/api/gallery/<id>` は files の local フラグを返す。UI は `if (!entry.local && i !== 0) return;` で、cover（i=0）と local file がある image だけ `<img>` にする。
- `/api/thumb/` に `gallery=<id>` が付くと local-first。local file があれば `Galleries/<id>/<hash>.<format>` を返す。TN が無い hash は full image proxy へ fallback（decoder 生成 cache はなし）。
- reader main image は local-first の `/files/...`。local なしで CDN full image が窓に無い場合は upstream 404（既存の時間窓問題）だが、UI は broken 画像を strip に残さない。
- page model は現行維持（viewer の `page` は file index、hitomi の page model は 25 image per page）。本件では page model を変更しない。

## 画面確認（2026-11-01）

- search results: card に cover だけ表示。mini page thumbnails なし。例 cover proxy `97cb6a05…?format=webp&gallery=4295193` → naturalWidth 460。
- remote gallery `4295193`（local 0）: reader strip は cover 1 枚だけ。非 local の page thumbnails は描画しない。broken img なし。
- local gallery `4252697`（local 1）: reader main image `/files/4252697/899dc188…webp` → `200`、strip local entry → display。
- local なし remote full image `97cb6a05…` の full image proxy はこの窓で 502。これは既知の CDN 時間窓問題。
- Screenshot: `scratch/hitomi-usage/screen_cover_only_strip_local.png`。

関連: `docs/ai/known-issues/2026-11-01-viewer-cdn-time-window-404.md`（同一 hash の `a`/`b` は同一 content・窓に entry が無いと 404）
