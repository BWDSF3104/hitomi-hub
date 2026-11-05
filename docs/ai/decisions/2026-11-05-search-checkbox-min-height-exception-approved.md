# 検索 form checkbox の min-height 例外（2026-11-05）

**Status**: 承認済み（親の判断採用 2026-11-05 14:59）
**Date**: 2026-11-05
**Scope**: hitomi-hub / `app/search.html` の `form#search`
**対象項目**: 案B refined（検索画面 prev/next の pager 行独立・単独 flex item 化）で残った段階3

## 選択

`form#search` の `min-height: 31px` 規則（U6・text input・button の寸法/baseline 統一）は維持し、`input[type=checkbox]` だけ例外として `min-height: auto` を適用する。

```css
form#search input, form#search button { box-sizing: border-box; min-height: 31px; }
form#search input[type="checkbox"] { min-height: auto; }
```

U6 の趣旨は「同一レイヤの control 寸法・baseline 統一」であり、checkbox を 31px へ引き延ばすことではない。checkbox は UA 既定の四角描画（実測 `13x13`）へ戻す。

## 理由

- 規則を全 `input` へ一律適用すると filters 行の checkbox が `13x31` の縦長 box として描画され、filters 行高さ `38px` になった（実測 2026-11-05 14:23）
- 承認 mock（`docs/ai/tmp/search-prev-next-layout-mock-proposed.png`）は filters 行高さ `20px`・checkbox は通常四角で、実装結果と視覚上食い違っていた（段階3として保留）
- 例外適用後の実測（fresh target・window size 基準）: checkbox `13x13`・`min-height: auto`、filters 行 `h=22px`（mock 想定 `20px` へ寄る）、`form#search` height `100px`
- 状態変化は opacity/color のみ（transform 不使用）という U9、focus ring（U2）、disabled visual state（U1）、境界色不変（U3 現状維持）、pager 行契約、DOM 順 `前 -> #page -> 次`、tab order = visual order はすべて維持。例外は checkbox の高さ指定のみで、bounds を動かす変更ではない

## 影響

- `form#search` の checkbox 描画のみ。DOM 構造・pager 順・gap `8px`・`:focus-visible`・`button:disabled`・viewer 側・route・prefetch・server・card height contract・`.side`/`.main` 規則は影響なし
- `form#saved-search` の input へは波及しない（規則は `form#search` scoped。global 未適用）
- re-anchor した行契約（`docs/ai/decisions/2026-11-05-search-form-row-contract-re-anchor.md`）の「各行必要幅 ≤ `.side` 可用幅 `465px`」は維持（filters 行必要幅 `329px` ≤ `465px`）
- 既存 Go test・Python test 合格（test ファイル変更なし）
