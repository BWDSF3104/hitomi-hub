# 全文（タイトル）検索は index 未実装のため使えない

**Status**: 解決（2026-11-01・`search --title`）
**Date**: 2026-11-01

## 現象

`app/hitomi.go` の `Search` は nozomi を直接読む。hitomi の client は

- namespace 付き term（`female:`・`male:` → area `tag`、`type:`・`artist:`・`group:`・`series:`・`character:` → area = namespace）→ `//ltn/<area>/<term>-<lang>.nozomi`
- namespace なしの term（タイトル検索・語群组）→ `hash_term(query)`（sha256）で `tagindex` の B-tree（`max_node_size=464`・`B=16`・big-endian）を降りて `nozomiurlindex` から URL を解決する

後者が未実装。現状は単語 term だけ直Path で動く（`goudoushi-japanese` → 200）。語群组（例 `big breasts`）は直Path で 404 になる。

## 解決（実装）

`app/searchindex.go` に B-tree 降下を実装。`hub.exe search --title "<phrase>"` で使う。

- `hash_term(term)` = sha256(UTF-8(term)) の先頭 4 bytes（小文字化後）
- `galleriesindex` の field `galleries` は **word → gallery IDs の倒引**。1 word の key は降下で引ける
- phrase は hitomi 本体では検索しない（client は word の suggestion だけ）。ここでは token を whitespace と `- / # '` で切り、各 token の集合の**共通部分**を取る。順序は最初の token の順序（newest first）
- 実測で `nee-san` は 1 つの key ではない（`nee`・`san` に切れる）。token 化はそれに合わせている
- version は `https://ltn.gold-usergeneratedcontent.net/galleriesindex/version`（`ltn.hitomi.la` はこの環境で応答せず 000）

## 実測

- `n/tag/female:yuri-english.nozomi` → 200・40016 bytes（10004 件）
- `n/type/doujinshi-japanese.nozomi` → 200・1192956 bytes（298239 件）
- `n/artist/kikunyi-japanese.nozomi` → 200・236 bytes（59 件）
- `n/tag/type:doujinshi-japanese.nozomi`（area を tag に固定していた旧実装）→ 404
- `galleriesindex` version `1793467632`。word `kemo` → 100726 件（data 402908 bytes）
- `search --title "kemo nee-san 10"` → 共通部分 22 件・`4252697` を含む（scratch 照合と同じ）
