# 全文（タイトル）検索は index 未実装のため使えない

**Status**: 未解決（記録のみ）
**Date**: 2026-11-01

## 現象

`app/hitomi.go` の `Search` は nozomi を直接読む。hitomi の client は

- namespace 付き term（`female:`・`male:` → area `tag`、`type:`・`artist:`・`group:`・`series:`・`character:` → area = namespace）→ `//ltn/<area>/<term>-<lang>.nozomi`
- namespace なしの term（タイトル検索・語群组）→ `hash_term(query)`（sha256）で `tagindex` の B-tree（`max_node_size=464`・`B=16`・big-endian）を降りて `nozomiurlindex` から URL を解決する

後者が未実装。現状は単語 term だけ直Path で動く（`goudoushi-japanese` → 200）。語群组（例 `big breasts`）は直Path で 404 になる。

## 実測

- `n/tag/female:yuri-english.nozomi` → 200・40016 bytes（10004 件）
- `n/type/doujinshi-japanese.nozomi` → 200・1192956 bytes（298239 件）
- `n/artist/kikunyi-japanese.nozomi` → 200・236 bytes（59 件）
- `n/tag/type:doujinshi-japanese.nozomi`（area を tag に固定していた旧実装）→ 404

## 対応案（未実施）

`decode_node`（big-endian: key 数→keys→datas(offset,length)→subnode B+1）と B-tree 降下、`global.json` の version を実装して `nozomiurlindex` 解決を追加する。node 形式は `scratch/search.js`（ltn 実測）から取得済み。
