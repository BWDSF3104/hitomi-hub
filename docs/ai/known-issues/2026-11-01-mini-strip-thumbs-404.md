# viewer の mini strip は 2 枚目以降の thumbnail を表示できない

**Date**: 2026-11-01
**Status**: 観察（実装変更なし・修復の判断はユーザー）

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

関連: `docs/ai/known-issues/2026-11-01-viewer-cdn-time-window-404.md`（同一 hash の `a`/`b` は同一 content・窓に entry が無いと 404）
