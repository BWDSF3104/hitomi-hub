# D-10: searchHTML plain term のアンダースコア

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

機構は事実（`normalizeSearchTerm` が `_` → space、`hashTerm` は `sha256(lower(term))[:4]`、`app/searchindex.go:27-30`）。実測: `big_breasts`（1 token）→ total 1、`big breasts`（2 token）→ total 473481（tokenization が違うため直接比較不可）。defect として再現できず、hitomi の index key が space 型か underscore 型かの正典確認が必要。判定: hitomi 準拠確認待ち。対応: 確認後に normalize 方針を固定。推定規模: 小（確認込み）。

### 正典確認（2026-11-04・assess-20261103-1220）

正典根拠（hitomi live JS: `docs/ai/tmp/assess-20261103-1220/scratch_results.js`・`scratch_search.js`・`scratch_searchlib.js`）:

- `results.js:30` `term = term.replace(/_/g, ' ')` — term は `results.js:17` の `query_string.toLowerCase().trim().split(/\s+/)` で切った後、`_` を space へ直してから検索へ使う（`positive_terms`/`negative_terms` へ入る形も space 形）。
- `search.js:653`（`get_galleryids_for_query`）も plain term 分岐で `query = query.replace(/_/g, ' ')` を適用し、`search.js:674` の `const key = hash_term(query)` は space 形の term に適用される。
- `hash_term`（`searchlib.js:78`）は `sha256(term)` の先頭 4 bytes。小文字化は呼出側（`results.js:17`・`search.js:38`）で済んでおり、アプリの `hashTerm` の `strings.ToLower` は等価。

→ hitomi の index key は **space 型**。アプリ現挙動（`normalizeSearchTerm`）は既に正典一致。

live 応答観察（実測・`docs/ai/tmp/assess-20261103-1220/d10_live_key_form_check.py`・`d10_live_observation.txt`・呼出し 33・gate 1000ms 維持）:

- galleriesindex version `1793765075`
- `big breasts`（space 形）key `92ef4857`: FOUND・data offset 1408314632 length 8・count 1
- `big_breasts`（underscore 形）key `95e737a6`: NOT FOUND（降下 7 hop）
- `big` key `2a21fe6d` 510671 件・`breasts` key `ff5ae7e8` 492903 件・intersection 473712（2026-11-03 記録 total 473481 と増分整合）

同一 phrase で live index に存在するのは space 形 key のみ。`big_breasts` → total 1 は hitomi でも 1 件（1 term の exact key lookup）で、defect ではない。判定: space 型。承認済み方針（`2026-11-03-remaining-item-recommendations-approved.md` の A-04/D-09/D-10 条）どおり、挙動変更なし・テスト固定＋記録明記。

### 変更

- `app/searchhtml.go`: `normalizeSearchTerm` に正典根拠（`results.js:30`・`search.js:653`・live key 実測）のコメント追加。挙動不変。
- `app/searchhtml_test.go`: `TestSearchHTMLPlainTermUnderscoreUsesSpaceFormKeyPerCanonicalHitomi` 追加 — plain term の `_` が space 形 key として hash されること（underscore 形 key では降下しない）と、whitespace split（`results.js:17`）は別 lookup（共通部分）であることを固定。

### 観察（本項目スコープ外・修復せず known-issue 記録・判定は親へ）

nozomi dataset name 形は hitomi 正典で space 形（`search.js:612-624` は `state.tag` を raw で join、code 内コメント例 `female:sole%20female-czech.nozomi`）。アプリは `SanitizeTerm`（`app/hitomi.go:507-509`）で space → underscore へ直して URL を組む。live 実測（呼出し 4・gate 1000ms）: space 形 200 / underscore 形 404（`female:sole female` 1916664 bytes・`female:big breasts` 1883372 bytes、underscore 形は両方 404）。多語 tag の nozomi fetch はアプリ側 URL が underscore 形のため 404 になり得る。既存テスト `TestSearchHTMLPreservesUnderscoreTagTerm` は underscore 形 URL を固定しており正典と矛盾。記録のみ: `docs/ai/known-issues/2026-11-04-nozomi-dataset-name-form-space-vs-underscore.md`。

## Verification

- `app` で `go vet .` → clean（output なし）
- `go test -count=1 -timeout 300s .` → ok `hitomi-hub/app` **16.627s**（flaky LRU tie 発火なし）
- 追加テスト単独実行: `go test -count=1 -run "TestSearchHTMLPlainTermUnderscoreUsesSpaceFormKeyPerCanonicalHitomi|TestSearchHTMLPlainTermUsesIndexExactTerm" -v .` → 両 PASS（0.168s）
- canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 成功（server 非起動）。実機 UI 確認は該当なし（UI 非関連・親実施）
- ルート `python -m pytest tests/ -v` → **40 passed in 8.32s**（`test_canonical_build_freshness.py` 含む）
- 外部呼出し計 **37**（index 33 + nozomi 4）、gate 1000ms 維持・回避なし

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager.py end` 2026-11-04）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
