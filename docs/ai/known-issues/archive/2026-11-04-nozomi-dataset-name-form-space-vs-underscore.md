# nozomi dataset name は hitomi 正典で space 形、アプリは underscore 形へ変換して fetch している

**Status**: 解決（2026-11-04・本実施）
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

## 実施（2026-11-04・本項目）

decision `docs/ai/decisions/2026-11-04-nozomi-dataset-name-space-form-correction.md` どおり正典 space 形へ揃えた。

- `SanitizeTerm`（`app/hitomi.go`）を正典形へ: 小文字化＋`/#` 除去のみ（space→`_` 変換削除）
- `nozomiPathParts` で colon split 前に `_` → space（results.js:30・search.js:653）を適用
- テスト固定更新（旧 `TestSearchHTMLPreservesUnderscoreTagTerm` を supersede → `TestSearchHTMLColonTagTermUsesCanonicalSpaceFormURL`）
- live 実測（explicit live test mode・呼出し 1・gate 1000ms 既定維持）: app 構築 URL `https://ltn.gold-usergeneratedcontent.net/n/tag/female:sole%20female-all.nozomi` → 200（1916712 bytes）
