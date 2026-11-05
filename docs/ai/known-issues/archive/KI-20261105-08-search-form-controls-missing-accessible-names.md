# 検索 form の control に accessible name がない（U10）

**Status**: 解決（修正承認 2026-11-05 18:07・実装 2026-11-06）
**Date**: 2026-11-05
**Source task**: `docs/ai/tasks/2026-11-05-search-prev-next-button-layout.md`（ui-ux-pro-max 再検討 U10）

## 症状（実測）

fresh tab（canonical server `http://127.0.0.1:8731/search`）で `form#search` の control を照合:

- `#query`: `labels` なし・`aria-label` なし。`placeholder="furry / female:yuri language:english"` のみ（placeholder-only input）
- `#lang`: `labels` なし・`aria-label` なし・`placeholder` なし → accessible name が完全にない
- `#page`: `labels` なし・`aria-label` なし・`placeholder` なし → accessible name が完全にない
- `form#saved-search` の `#qname`/`#qtext` は placeholder のみ（`name` / term 例）

スクリーンリーダーは name なし edit field を「edit」等のみで読み上げる。保存検索の list 表示や badge 表示にも name 参照はない。

## 影響

レイアウト項目（prev/next 位置の清書）では解決しない。DOM 順＝読み順の整え（U4）と独立した問題。

## 次の対応（未実施）

- `aria-label`（layout を変えない最小）または `<label>` の追加。visible label を入れると form 行構成が変わるため、行契約 re-anchor の判断（同 task の判断保留 1）と調整が必要。
- 修復には実装承認が必要。正典 UI confirmation（fresh target・window size 基準・actual control 操作）と、`app/viewer_test.go` の該当照合（`form#query` と `input#query` の id 衝突禁止条、`document.getElementById("query")` の解決先）への影響確認が必要。

## 実施記録（2026-11-06）

- 最小方針（`aria-label` のみ）で実施: `#query`=検索 term・`#lang`=言語・`qname`=名前・`qtext`=term。`#page` は `0cb18b7` 済み（ページ）
- guard テスト `TestEmbeddedUISearchFormControlsHaveAccessibleNames`（`app/viewer_test.go`）で固定。id 衝突禁止条・button 照合への影響なし
- 正典 build 実機確認・検証結果は `docs/ai/tasks/2026-11-06-ki-20261105-08-search-form-accessible-names.md`
- visible label 導入（行契約 re-anchor）は方針として未採用（本KIの範囲外）
