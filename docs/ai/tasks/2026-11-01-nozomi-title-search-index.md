# nozomiurlindex 降下（phrase・タイトル全文検索）実装

**Status**: 完了
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

### 保存検索の新着追跡（`check`）

`app/check.go` + `hub.exe check`。`bookmark_queries` を 1 回引いて `galleries` に無い id だけを記録し `last_run_at` を更新。multi-term は term ごとの nozomi 集合の共通部分（`TestSearchQueryIntersectsTermsAndLanguage` で lock）。`language:` term は fetch せず lang を設定し、単独 `language:x` は `/n/index-<lang>.nozomi`（`nozomiPathParts` を term 単位に修正）。`notify` 列は現行実装では未使用。

### エクスポート（cbz・pdf）

`app/export.go` + `app/pdf.go` + `hub.exe export`。page 順は新設の `pageorder`（download 時に記録）、無ければ `files` 行の rowid 順。entry 名 `NNN_<hash>.<ext>`・zip は Store。`hit`/`scan_error`/`missing` は skip。`--out` は root 内限定。

- `recordFile` に `gallery_id` を追加（path の `<gallery_dir>/<id>/` から解析）。過去の行は scan 再実行で埋まる
- pdf は decoder が通る形式のみ（webp・png・jpeg）。webp は `golang.org/x/image/webp`（pure Go）。AVIF は pure-Go decoder が無く skip。1 ページ 1 画像・FlateDecode の raw RGB

## Verification（2026-11-01）

- `go vet ./...` clean、`go test -count=1 ./...` → ok（23 tests）。追加: `TestHashTermIsSHA256Prefix`、`TestDecodeNodeMatchesSearchlibLayout`、`TestIndexTokensSplitsHyphenAndUnderscore`、`TestSearchTitleIntersectsTokensInFirstTokenOrder`、`TestSearchTitleDescendsToLeafNode`、`TestDecodeIDsLengthGuard`
- live: `search --title "kemo nee-san 10"` → total 22・page 1 に 22 件・`4252697` を含む（scratch `idx_phrase.py` の照合と一致）
- version 実測: `1793467632`。word `kemo` data 402908 bytes（100726 件）

checkpoint 10（`check`）:

- `go vet ./...` clean、`go test -count=1 ./...` → ok（26 tests）。追加: `TestRunSavedQueriesTracksNewGalleries`、`TestRunSavedQueriesUsesIndexForPlainQuery`、`TestSearchQueryIntersectsTermsAndLanguage`
- live: `check --limit 10` → `artist:kikunyi language:japanese` で 10 件記録、再実行で残り 15 件、3 回目は 0 件（新着判定は `galleries` 行）。`female:yuri language:english` は 25 件 seen・0 新着（multi-term が正しく解けた）
- `list --list queries` の `last_run_at` が `2026-10-31T19:33:38Z` で更新されたことを確認

checkpoint 11（`export`）:

- `go vet ./...` clean、`go test -count=1 ./...` → ok（30 tests）。追加: `TestExportCBZUsesPageOrderAndSkipsHit`（Store method 確認）、`TestExportCBZFallsBackToInsertionOrder`、`TestExportPDFWritesPagesAndSkipsUndecodable`、`TestExportRefusesOutputOutsideRoot`
- live: `export --id 4294614` → cbz entry `001_2d18a9a8…webp`（method 0・210784 bytes）。`--format pdf` → `4294614.pdf` 2114910 bytes・`%PDF-1.4`・`/Count 1`・xref `startxref 2114723`・`%%EOF`
- `scan` 再実行（`--no-vt`）で `files.gallery_id` が path から埋まり、過去の行（4252697）でも export が 1 entry を出した

## 解った既知問題

- `known-issues/2026-11-01-plain-title-search-needs-index.md` → `known-issues/archive/` へ移動（実装済み）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
