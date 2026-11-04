# 画面確認: viewer の thumbnail/画像は CDN の時間窓に依存

**Date**: 2026-11-01
**Status**: 解決（2026-11-04・基準整除・ユーザー承認 `decisions/2026-11-04-observation-records-closure-approved.md`）。再観察不要（窓運依存）。strict full pixel pass は control evidence（cache hash `86f0dba8…` の `1/1` 表示確認済み）で充足判定。natural window observation は optional。窓依存 404・DNS 3番目解決失敗は環境挙動として記録維持。コード作業なし。

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

## 2026-11-02 current investigation

- Current canonical server `pid 5152` returns `no_full` for target gallery `4299126` entries.
- Target gallery `4299126` has `37` valid hashes, but none is present in `scratch/p3_canonical_fresh/cache/images`.
- Existing full-image cache files can still satisfy actual UI actual pixels if a UI-openable gallery contains one of those hashes.
- Confirmed existing cache evidence:
  - `scratch/p3_canonical_fresh/cache/images/86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3.avif`
  - `http://127.0.0.1:8731/api/image/86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3?format=avif`
    returned `200`, `x-cache: cache`, `content-type: image/avif`.
  - Browser decoding confirmed dimensions `1295x1812`.
- Pass criteria for actual UI full-image visual confirmation:
  - direct server response must be `x-cache: cache`, `local`, or a successful remote full fetch, not `no_full`;
  - UI must use `objectURL` or `local-direct` as viewer display source;
  - the displayed `img` must have `naturalWidth > 0`;
  - a normal OS physical screen capture must show the reader image, not only status text.
- If no current-window or existing-cache gallery hash matches, actual full-image visual pass remains blocked unless a controlled test environment is explicitly allowed.
