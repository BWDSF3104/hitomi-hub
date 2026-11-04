# nozomi dataset name は正典 space 形へ揃える（D-10 確認結果に基づく追加判断）

**Status**: 判断確定
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220

## 根拠

- `results.js:30`・`search.js:653` が term の `_` を space へ直した space 形の term を使う。colon term 分岐は `state.tag` へ space 形で入れ、`nozomi_address_from_state`（`search.js:612-624`）は `state.tag` を raw で join する（code 内コメント例 `female:sole%20female-czech.nozomi`）。
- hitomi 正典 `sanitize`（`searchlib.js:74`）は `input.replace(/[\/#]/g, '')` のみ。space → `_` 変換は存在しない。小文字化は呼出側で済んでいる。
- live 実測（2026-11-04・呼出し 4・gate 1000ms）: `female%3Asole%20female-all.nozomi` → 200／`female%3Asole_female` → 404、`female%3Abig%20breasts-all.nozomi` → 200／`female%3Abig_breasts` → 404。

## 判定

- アプリ `SanitizeTerm`（`app/hitomi.go:507-509`）の space → `_` 変換は正典と矛盾（正典は colon split 前に `_`→space、dataset name は space 形）。正典形へ揃える: term へ `_`→space を適用後 colon split、`SanitizeTerm` は小文字化＋`/#` 除去のみ（space→`_` なし）、URL は space 形（`%20`）。
- 既存テスト `TestSearchHTMLPreservesUnderscoreTagTerm`（`app/searchhtml_test.go`）の固定内容（underscore 形 URL）は正典矛盾として supersede（本 decision）。固定内容は正典 space 形へ更新する。
- known-issue `docs/ai/known-issues/2026-11-04-nozomi-dataset-name-form-space-vs-underscore.md` は本判断で解決段階（実施時に Status 解決・archive へ mv）。

## スコープ限定

- acquisition 経路（B-04）のフォルダ命名（`acquisitionTitleName`）は変更しない。本追加項目は nozomi path 構築と関連テストのみ。
- 実施は実行計画の追加項目（単独 commit・push）。承認済み方針 `docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md` の A-04/D-09/D-10 条（hitomi 準拠確認後に hitomi 挙動へ）に基づく。実測根拠（正典コード＋live 応答）が揃ったため 2026-11-04 親判断で委譲。
