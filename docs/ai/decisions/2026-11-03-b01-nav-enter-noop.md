# B-01 nav フォームの Enter は no-op（検索固定）。page 跳转は別項目

Date: 2026-11-03

**Related decisions**:

- `2026-11-03-b01-nav-enter-noop.md`（本エントリ。根拠は `docs/ai/tasks/2026-11-03-b01-enter-implicit-submit.md`）

**Decision**:

- `form#search` の暗黙 submit は `#search-submit` 単独（唯一の submit button）に固定する。prev/next/complete/fav は `type="button"`。
- `form#nav`（`#pagenum`）で Enter した場合は submit event だけが発火し、handler は no-op。従来的な prev ボタンの暗黙発火（前ページへ移動）を復元しない。
- nav で入力した page 番号へ移動する動作は本項目の範囲外。必要なら新項目（feature）として切る。
- B-01 はこの解釈で合格確定（commit `c1d2c07`、UI 実操作・network 実測済み）。

## Reason

B-01 の目的は「Enter の暗黙 submit を検索固定」であり、nav の Enter が prev を発火させていたのは button に `type` がなかったことによる副次的な誤動作だった。nav の Enter を page 跳转として扱うかどうかは動作設計の追加判断で、バグ修正の範囲とは分離する。

## Non-goals

- nav の Enter に page 跳转を与えない（別項目で判断）。
- `setSearchPage` の既存意味を変えない。
- viewer・フル画像経路・cache・TN の判定を変えない。
