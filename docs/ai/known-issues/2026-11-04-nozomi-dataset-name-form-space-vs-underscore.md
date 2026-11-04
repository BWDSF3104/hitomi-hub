# nozomi dataset name は hitomi 正典で space 形、アプリは underscore 形へ変換して fetch している

**Status**: 未解決（低優先・D-10 の合格条件スコープ外・判定は親へ）
**Date**: 2026-11-04

## 内容

D-10（plain term の index key 規約）の正典確認中に判明。

- hitomi 正典: `results.js:30` と `search.js:653`（`get_galleryids_for_query`）が term の `_` を space へ直した
  space 形の term を使う。colon term 分岐は `state.tag` へ space 形で入れ、`nozomi_address_from_state`
  （`search.js:612-624`）は `state.tag` を raw で join する。code 内コメント例（`search.js:617`）は
  `female:sole%20female-czech.nozomi`。
- アプリ: `SanitizeTerm`（`app/hitomi.go:507-509`）が space → underscore へ変換し、`nozomiPath`
  （`app/hitomi.go:629-635`）で URL を組む。多語 tag では dataset name が underscore 形になる。

live 実測（2026-11-04・呼出し 4・gate 1000ms 維持・
`docs/ai/tmp/assess-20261103-1220/d10_nozomi_name_check.py`・`d10_nozomi_name_check2.py`）:

- `n/tag/female%3Asole%20female-all.nozomi` → 200（1916664 bytes）／`female%3Asole_female` → 404
- `n/tag/female%3Abig%20breasts-all.nozomi` → 200（1883372 bytes）／`female%3Abig_breasts` → 404

多語 tag の nozomi fetch はアプリ側 URL が underscore 形なので 404 になり得る（space 形が正）。
既存テスト `TestSearchHTMLPreservesUnderscoreTagTerm`（`app/searchhtml_test.go`）は underscore 形 URL を
固定しており、正典（space 形）と矛盾する。

## 次の対応（実施時）

nozomi path 構築の space/underscore を正典（space 形）へ揃える。既存テストの固定内容変更を要するため
実施段階の判断が必要（D-10 のスコープは index key 規約のみ。本項目では記録のみで修復していない）。
