# 検索 form の行契約への re-anchor（2026-11-05）

**Status**: active（criteria 本体は書き換えない。適用基準の re-anchor 記録）
**Date**: 2026-11-05
**Scope**: hitomi-hub / `app/search.html` 検索 form
**対象項目**: 案B refined（検索画面 prev/next の pager 行独立・単独 flex item 化）

## Supersede しない

`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` の criteria 本体（direct server baseline 先行、fresh target、actual UI event、network/DOM/visual 一致、既存回帰チェック、`.side` = `480px`、card height contract、`#results` card 数 = `ids.length`）はそのまま維持する。この entry は `form#search` の「単一行・wrap なし」項目の適用基準だけを、2026-11-05 承認（案B refined）の 3 行契約へ re-anchor する。

window size 基準は `docs/ai/decisions/2026-11-05-window-size-based-screen-confirmation-rule.md`（ルール側: `docs/ai/rules/screen-confirmation-size-rule.md`）を維持する。固定 target size は合格条件にしない。

## re-anchor 後の行契約

`form#search` は 3 視覚行で構成される:

1. search 行: `#query`（`flex: 1 1 160px; min-width: 0`）+ `#lang` + `#search-submit`
2. pager 行: `<span class="pager">`（単独 flex item・`flex-shrink: 0`）= `前` `#page` `次` + `#search-page-status`（同じ行の右）
3. filters 行: 4 checkboxes

各行の合格条件:

- 各行必要幅 ≤ `.side` 可用幅 `465px`（`.side` 実測 `480px` − 縦スクロールバー `15px`）
- 行内で control が分割されない（`前` と `次` は `#page` を挟んで同一 pager 行に隣接。pager group は `flex-shrink: 0` で構造保証）
- DOM 順 = 視覚読み順（tab order = visual order）。pager 内 DOM 順は `前 -> #page -> 次`

criteria の「`#search` height must be a single control row, approximately `67px` to `70px`」と「`100px` or more because controls wrap」は、この 3 行契約では次のように読む:

- 不合格になるのは *意図しない wrap による分割*（`前` と `次` が行をまたぐ、または行必要幅 > `465px`）
- 明示された 3 行契約（各行 fit・行内 wrap なし）は合格。`67〜70px` の単一行数値は本項目の行契約では適用しない

## 実測根拠（fresh target・window size 基準・人工 resize 不使用）

- actual window size: outer `1663x1020`、content viewport（inner）`1650x927`、`devicePixelRatio` 2
- `.side` 実測 `480px`（shrink 不可・正典契約維持）、`clientWidth` `465px`（scrollbar `15px`）
- `form#search` height `116px` = `31`（search 行）+ `31`（pager 行）+ `38`（filters 行）+ gap `8px` × 2
- search 行 必要幅 `465px`（flex fill）= `#query` `307` + `#lang` `96` + `#search-submit` `46` + gap `8` × 2 → `≤ 465px`
- pager 行 必要幅 = pager group `165`（`前` `40` + `#page` `69` + `次` `40` + gap `8` × 2）+ status `167` + gap `8` = `240px` → `≤ 465px`
- filters 行 必要幅 `306px`（mock 実測）≤ `465px`。実測行 `w=465`（flex fill）、行内 wrap なし
- pager group 内 隣接: `前` `x=16..56`、`#page` `x=64..133`、`次` `x=141..181`（gap `8px`）→ `前`/`次` の隣接は構造で保証
- `#search-page-status` は pager 行の右（`x=314..481`、行右端）
- `:disabled`（page 1 の `#search-prev`）= `opacity: 0.5`・`cursor: not-allowed`・`transform: none`（bounds を動かさない）
- `:focus-visible`（keyboard focus）= `outline: rgb(138,180,248) solid 2px`・`outline-offset: 2px`
- control 境界色は現状の border-less 見た目を維持（focus ring で補完）。U3 の `#6b7280` 等への変更はしていない

## 残る適用基準（未確定・持ち帰り）

- `input, button { box-sizing: border-box; min-height: 31px; }` を text input と button に適用した結果、`filters` 行の checkbox（`input[type=checkbox]`）が `13x31` の縦長 box として描画される（実測 `h=38` 行）。承認 mock（`docs/ai/tmp/search-prev-next-layout-mock-proposed.png`）の filters 行高さ `20px`（checkbox は通常の四角）と視覚上で食い違う。checkbox を `min-height` 適用から除外するか、現状（縦長描画）を許容するかの判断はユーザー承認を要る（段階3）。この entry は re-anchor 記録のみで、rule 本体は変更していない。
