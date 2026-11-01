# 画面確認: viewer の thumbnail/画像は CDN の時間窓に依存

**Date**: 2026-11-01
**Status**: 観察（実装変更なし）

## 確認できたこと（screen）

- UI は動作: 検索（`furry` → 32834 件・page 1）、保存検索（`yuri-en`・`kikunyi-jp`）、既読/お気に入り一覧、reader（gallery title・tags・files・`local`/`cdn（未ダウンロード）` マーカー、前/次/読み完了/お気に入り、page strip）
- local-first は動作: `/files/4252697/899dc188…webp` → `200`（reader の main image が表示、`naturalWidth > 0`）
- 画面操作は DB に反映: reader を開くと `POST /api/read`（`last_page=1`）、`お気に入り` クリックで `POST /api/fav` → `favorite=1`（CLI `list --list favorites`/`unread` で確認）

## 現象

- TN proxy の 404（proxy 側は 502）が多い。gallery 4252697（2026 以前の gallery）の page 1 hash は 19/20 が 404、検索結果（現行窓の gallery）は 200。同一 hash の `a`/`b` は同一内容（both 200）、`c` はこの環境で DNS 解決失敗（`number_of_frontends = 3` の 3 番目が使えない）
- TN が 200 の hash（`97cb6a05…`）でも shard（full image）は 404: `w2/1793494802/3190/97cb6a05….webp` → 404、TN（`…/webpbigtn/c/76/…`）→ 200。shard の data は live gg.js が指す窓に在るときのみ解ける（checkpoint 10 の性質と同じ）

## code との対応

- `common.js` の `subdomain_from_url` は `g = parseInt(prev two hex, 16)`・`g < 0x80 → 2 frontends`・`g < 0x59 → g = 1`・`letter = 97 + (g % frontends)`。app の TN subdomain は `gg.m(gg.s(hash))`（`subdomain_from_galleryid` の引数違い）。実測では `a`/`b` が同一 content を返すため機能上の差はなし（窓に entry が在るか否かが 404 の決定要因）

##  screenshot

`scratch/hitomi-usage/screen_4252697_page1.png`（search results + reader・local image・page strip）
