# 検索画面のprev/nextボタン位置の清書（レイアウト検討）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 案A/B選択と form#search 行契約 re-anchor の承認待ち（実装開始指示未受領）

## 意図 / 背景

ユーザー指示（2026-11-05 13:34）: 「検索画面 前と次のボタン位置を綺麗に清書したい。検討」。

`app/search.html` の `form#search` は単一行 flex-wrap で、`#query`（size=30）が幅を占有するため意図しない位置で wrap し、`#search-prev` と `#search-next` が別の行に分かれる。本項目は現行の実測・原因特定と清書案の提示まで。ソースは無変更（実装開始指示未受領）。

正典読了（search/viewer に触る項目の作業前チェック・ファイル名明記）:
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`
- `docs/ai/decisions/2026-11-04-search-page-layout-and-vestige-cleanup-approved.md`（現行レイアウトの正典）
- `docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md`（`form#search` 単一行基準）
- `docs/ai/decisions/2026-11-05-window-size-based-screen-confirmation-rule.md`（window size 基準）
- `docs/ai/rules/normal-screenshot-rule.md`（通常スクリーンショット基準）

## 調査結果 / 変更内容

### 現行実測（canonical exe・server `http://127.0.0.1:8731`・fresh tab）

正典 exe は fresh（`python -m pytest tests/test_canonical_build_freshness.py -v` = 2 passed）。正典 server（PID 27220・session lifetime・port 8731）で測定。

- actual window size: outer `1920x1040`、inner（content viewport）`1920x897`
- `.side` 実測幅 `480px`（shrink 不可・正典契約維持）、`.side` 内可用幅 = `465px`（縦スクロールバー 15px 控除）
- `form#search` height `68px` = 2 視覚行（`y=119` 行1・`y=157/161` 行2）
- 行1: `#query` 242px + `#lang` 96px + `#page` 69px + `#search-prev` 33px（`x=442..475`＝行1右端）
- 行2: `#search-next` 33px（`x=16`＝行2左端）+ `#search-submit` 46px + labels 72/47/72/77px
- children の必要幅合計 `844.6px` > `465px` → wrap 確定。前と次が行をまたぐ（前が行末、次が行頭）＝用户の不快Points
- `button:disabled` の CSS 規則なし → page 1 の `#search-prev`（disabled）が enabled と同一見た目。境界判定（`renderSearchPageStatus` の disabled）は機構として正しいが視覚化されていない

### 構造原因

- flex-wrap の break 位置が content 幅に依存するため、prev/next の隣接が保証されない（`#page` 96+69+33 の合計が行1に余白を残す位置で切れる）
- `<input>` は flex item の `min-width: auto`（= size 属性由来の intrinsic 幅）で shrink 不可。size=30 の `#query` は 242px 固定で、行1の残り 113px に pager group（147.8px）が入らない
- pager を単独 flex item（wrapper span）にすれば内部 wrap がないので分割しない → 隣接は構造で保証できる

### 清書案（実測根拠）

案A（最小・単一 control 行＋filters 行）
- 行1: `#query`（`flex:1 1 140px; min-width:0`）・`#lang`・pager group（`前 [page] 次`）
- 行2: 4 checkboxes（`flex-basis:100%` の明示行）
- 実測 fit: query を 158px まで shrink すると行1必要幅 `449px` ≤ `465px` で単一行。form height `57px`
- 制約: `min-width:0` がないと必要幅 `471px` > `465px` で wrap（実測）。query が size=30（242px）を保てず placeholder が切り詰める。正典 criteria の「単一行 67〜70px」数値は 57px へ移動

案B（推奨・pager 行の独立）
- 行1（search 行）: `#query`（size 30 のまま）+ `#lang` + `#search-submit` → 必要幅 `396.8px` ≤ `465px`（余白 68px・余白分 query が伸びる）
- 行2（pager 行）: `前 [page] 次` group + `#search-page-status`（page カウンタを同じ行の右に）→ 必要幅 `147.8 + 167 + 6.4 = 221.2px` ≤ `465px`
- 行3（filters 行）: checkboxes → 必要幅 `217px` ≤ `465px`
- 行高さ: 29 / 31 / 20 + gap 12.8 → form 全体 `86.8px`（現行 68px）
- viewer の reader nav（`前 [pagenum] 次`）と同形になり、ページ番号表示とページ送り操作が同じ行に揃う

案C（任意・visual polish）
- `前`/`次` に `min-width: 2rem`（現行 33px・同寸なので symmetrize は既に成立、検索 46px との差は許容）
- `button:disabled { opacity: .45; cursor: default; }` 追加（境界状态的視覚化）
- pager group を行末へ寄せる（`margin-left: auto`）— 読み順は `前 → page → 次` のまま
- 検索ページでの `←` / `→` ページ送りキー（viewer には既にある、search 側は現在なし）— 範囲外・任意

### テスト影響（実装時に反映）

- `app/viewer_test.go` `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit`（L3116-3140）: `form#search` 内 `<button` 3個、`#search-submit` のみ `type="submit"`、prev/next `type="button"` を文字列照合。pager group を **form 内**（`<span class="pager">`）に置く限り合格（照合は DOM 順に依存しない）。pager を form 外へ移すと「3 buttons」照合が壊れる → re-anchor 承認要。
- `app/viewer_test.go` `TestEmbeddedUIHasSearchPrevNextButtons`（L3074-3095）: id と `setSearchPage` 本文の照合 → 配置変更で無影響。追加案: pager wrapper と `前 → page → 次` の DOM 順を固定する新 test。
- `tests/search_prefetch_harness.mjs` L23（`#search-next` の hover/focus exact string）: id 維持で無影響。disabled 時に発火しない guard（`runPrefetch` の `page > searchTotalPages` return）は現行どおり。
- `tests/read_lists_debounce_harness.mjs`、`tests/strip_rebuild_dom_harness.mjs`: 影響なし。
- `tests/test_canonical_build_freshness.py`: 変更後 2 html byte 照合を再実行。

### 画面確認（実装時）

- window size 基準（fresh target・resize 不使用）で actual inner size を記録。direct server baseline 先行、actual `#search-prev`/`#search-next` click で `setSearchPage` 動作と network 一致、card 数 = `ids.length`。
- 行単位の fit を「必要幅 ≤ `.side` 可用幅（465px）」として記録（案B の設計値を正典化）。正典 criteria の `form#search`「単一行・wrap なし」項目は案B の行契約（search 行・pager 行・filters 行）へ re-anchor が必要（同 criteria の observed drift と 2026-11-03 review の記録 drift と整合させる）。
- スクリーンショットは Playwright browser 画面画像（通常スクリーンショット正典）、Drive のタスク別フォルダへ copy。

### 判断保留（ユーザー承認が必要）

1. 案A（単一 control 行＋filters 行）か案B（pager 行独立）か。案B は form height が 68px → 87px になり、正典 criteria の `form#search` 単一行項目の re-anchor を要る。
2. `#page` を `前`/`次` の間に置く DOM 順変更（viewer nav と揃える）。現行順は `page → 前 → 次`。
3. pager を form 内の単独 flex item（`<span class="pager">`）化するか（隣接の構造保証のため推奨）。form 外へ移す場合は button 3個照合の re-anchor が発生。
4. disabled ボタンの視覚化（opacity）を追加するか。
5. `←` / `→` による検索ページのページ送りを追加するか（任意・範囲外）。

## Verification

- 正典読了（作業前チェック）は上記ファイル名で記録済み。
- 測定は canonical exe（byte 照合合格 2 passed）と正典 server（port 8731・PID 27220）で実施。UI 表示変更なし（調査のみ）のため画面確認合格条件は未適用。
- 実測値（fresh tab・actual window size）: inner `1920x897`、`.side` `480px`、`.side` 可用幅 `465px`、`form#search` height `68px`（2 視覚行）、prev `x=442..475`（行1右端）、next `x=16`（行2左端）、children 必要幅 `844.6px`。
- クローン probe 測定（DOM 操作は measurement のみ・ファイル変更なし）: 案A 単一行 fit 必要幅 `449px`（`min-width:0` 必須・`471px` では wrap）、案B 各行 `396.8px` / `221.2px` / `217px` ≤ `465px`。
- スクリーンショット（通常スクリーンショット正典・Playwright browser 画面）: `docs/ai/tmp/search-prev-next-layout-current.png`（現行の分割位置を記録）。
- direct server baseline: `GET /api/search?query=furry&lang=all&page=1&limit=25` → `200`、`X-Search-Timing: search=1751;record=36;json=1`、ids 25。
- console error は `favicon.ico` 404 のみ（本項目と無関係）。

実装は未着手（実装開始指示未受領）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
