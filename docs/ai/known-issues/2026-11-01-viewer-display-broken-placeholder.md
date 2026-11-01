# 画面確認異常: 閲覧画面は空 placeholder のまま（search-card TN 表示は正常）

**Date**: 2026-11-01
**Status**: 異常（完成系不合格）

## 現在の画面

- URL: `http://127.0.0.1:8731/?x=current#/g/4296091/page/1-2`
- Screenshot: `scratch\hitomi-usage\current_after_load_20261101.png`

## 判定

- Search-card cover は TN 画像が実表示されている。
- 閲覧画面（reader/strip）は `img src=''`、`naturalWidth=0` で、実画像像素がない。
- status: `full display cache 0/40 | no full 39/40 | TN display fallback disabled`
- note: `no full image: 2`

これは「画面確認で status / generation guard を見た」だけでは合格にならない。画面確認の合格条件は actual rendered image pixels で、空 `<img>`、broken alt placeholder、`naturalWidth=0` は異常として扱う。ただし TN display fallback は viewer display で絶対許可しないため、viewer actual pixels は full image objectURL / local file が存在する場合だけ成立する。Full image が `no_full` の場合、viewer display は actual pixels で合格にはならない。Search-card TN cover 表示は search-card の合格であり、viewer display の合格ではない。

## code との対応

- `ui.html:300` の `displayPlaceholder(entry)` は `""` を返す。
- `ui.html:523` の `renderReader()` は full cache objectURL がない場合 `src=''` の placeholder を出す。
- `ui.html:469` の `loadFullImage()` は `x-cache: no_full` で objectURL を作らず return する。
- `ui.html:485` の `loadFullImage()` は `tn` / `tn-cache` を display source にせず return する。
- `ui.html:227` の `updateThumbCache()` は full objectURL のときだけ strip/search-card img を update する。

## 結論

Search-card TN cover 表示と閲覧画面の実画像表示は別問題。Viewer actual pixels は full image objectURL / local file が存在する場合だけ合格。Search-card TN cover 表示は search-card の合格であり、viewer display の合格ではない。

CP8 lightweight non-blocking viewer verification:
- `http://127.0.0.1:8731/?x=cp7lightweight#/g/4295779/page/3-4`: reader and strip have full cache blob objectURL and `naturalWidth > 0`.
- `http://127.0.0.1:8731/?x=cp7lightweight#/g/4296091/page/1-2`: `no_full` is explicit, reader/strip remain no actual TN pixels, matching policy.
- `showPage()` no longer awaits `post read` or `showLists()` in the display path. Page movement returned in `2ms`.
- Background queue was not recomputed for same-gallery next-page movement; `prefetchQueueKeys` remained stable.

Screenshot: `scratch\hitomi-usage\cp8_lightweight_actual_full_pixels.png`
