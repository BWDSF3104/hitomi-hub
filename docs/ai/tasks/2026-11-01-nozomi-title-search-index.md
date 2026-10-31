# nozomiurlindex 降下（phrase・タイトル全文検索）実装

**Status**: 進行中
**Date**: 2026-11-01
**Next Action**: hitomi の search 経路と urlindex の实际格式を確認

## 意図 / 背景

ユーザー指示（2026-11-01）: デバイス間同期は不要、それ以外の未実装項目を順番に実装。順は
1. phrase・タイトル全文検索（本タスク）
2. 保存検索の自動更新（新着追跡）
3. CBZ/PDF エクスポート

hitomi に server-side の全文検索 API はなく、client は `hash_term`（sha256 先頭 4 bytes）で B-tree を降りて倒引を引く。nozomi の直Path は namespace/単一 tag でしか動かない（既知問題として記録済み）。

## 調査結果 / 変更内容

`searchlib.js`（ltn 実測）から正典を取得: `B=16`・`max_node_size=464`・`hash_term` = sha256(UTF-8) 先頭 4 bytes・node 形式（big-endian: key 数 → keys(key≤32) → datas(offset,length) → subnode B+1）・version は `<dir>/version`。

- `galleriesindex` の field `galleries` は **word → gallery IDs の倒引**。1 word の key は降下で引ける（`kemo` → 100726 件）
- hitomi 本体は phrase を検索しない（suggestions は word 単位）。phrase は client 側で token 集合の共通部分として実装
- `nee-san` は 1 つの key ではない（`nee`・`san` に切れる）。token 化は whitespace と `- / # '`
- `ltn.hitomi.la` はこの環境で応答せず（000）。version/data とも `ltn.gold-usergeneratedcontent.net` で 200

実装（`app/searchindex.go`）: `hashTerm`・`decodeNode`・`locateKey`・`isLeaf`・`decodeIDs`（length guard）・`indexVersion`（client 内 cache）・`indexNode`・`indexData`・`IndexIDs`・`indexTokens`・`SearchTitle`。`main.go` に `--title` を追加（`valueFlags` 登録済み）。`--page`/`--limit` は共通部分の後に適用し、DB 記録は表示分のみ。

## Verification（2026-11-01）

- `go vet ./...` clean、`go test -count=1 ./...` → ok（23 tests）。追加: `TestHashTermIsSHA256Prefix`、`TestDecodeNodeMatchesSearchlibLayout`、`TestIndexTokensSplitsHyphenAndUnderscore`、`TestSearchTitleIntersectsTokensInFirstTokenOrder`、`TestSearchTitleDescendsToLeafNode`、`TestDecodeIDsLengthGuard`
- live: `search --title "kemo nee-san 10"` → total 22・page 1 に 22 件・`4252697` を含む（scratch `idx_phrase.py` の照合と一致）
- version 実測: `1793467632`。word `kemo` data 402908 bytes（100726 件）

## 解った既知問題

- `known-issues/2026-11-01-plain-title-search-needs-index.md` → `known-issues/archive/` へ移動（実装済み）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
