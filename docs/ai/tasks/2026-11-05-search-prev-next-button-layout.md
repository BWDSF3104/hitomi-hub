# 検索画面のprev/nextボタン位置の清書（レイアウト検討）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 段階3 判断待ち: filters 行 checkbox の min-height 31px 適用（実測 13x31 縦長描画・行 h=38）を承認規則どおり許容するか input[type=checkbox] を除外かのユーザー承認。3 行契約・DOM 順（前->page->次）・focus ring・disabled visual state・Drive 確認は実測合格（code 分は単独 commit）

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

### 再検討（ui-ux-pro-maxスキル・2026-11-05 13:58）

スキル検索（Query Contract に従い 1 intent = 1 query、追加 domain/stack）:
- `pagination pager control alignment prev next --domain ux`
- `disabled control state clarity not tappable --domain ux`
- `inline form field group hierarchy spacing --domain ux`
- `compact control group nowrap single row flex wrap --stack html-tailwind`（実装 stack は Tailwind ではない。flex/shrink 指針として参照）
- `visible focus dark mode state contrast parity --domain ux`

適用できたルールと本アプリ実測（canonical exe・port 8731・fresh tab・actual window inner `1920x897`）:

| # | ルール（skill） | 本アプリ現状（実測） | 是正 |
|---|---|---|---|
| U1 | Disabled state: 透明度低下 + cursor 変更。enabled と同一見た目は不可 | `button { cursor: pointer }` が disabled にも効く。`:disabled` 規則なし → page 1 の `前` が enabled と区別不可（disabled 属性は正しいが視覚表現なし） | `button:disabled { opacity: .5; cursor: not-allowed; }`。disabled 時の文字 contrast `3.89:1`（opacity .5）で状態差は知覚可（1.4.11 の exempt 範囲だが状態差の表示として妥当） |
| U2 | Focus indicator ≥2px 周長・3:1 state contrast | 実測 computed outline = `rgb(16,16,16) auto 1px` → button surface `#2f343b` に対し `1.52:1`（1px）→ 基準未達 | `button:focus-visible, input:focus-visible { outline: 2px solid #8ab4f8; outline-offset: 2px; }` → `5.95:1`（button surface 比）/ `8.59:1`（page 比） |
| U3 | 非テキスト UI 境界 ≥3:1 | control 境界 `#2f343b` = `1.44:1`、input surface `#1d2126` = `1.12:1` → 境界は情報を持たず text contrast（`10.40:1`）で判別 | 境界を識別子にするなら `#6b7280`（`3.75:1`）または `#606a73`（`3.28:1`）。現状の border-less button 見た目を維持し focus ring で補完する選択も可（要判断） |
| U4 | Tab order は visual order と一致 | 現行 DOM order `query → lang → page → prev → next → submit` は現行 visual order（前が行1末・次が行2頭）と一致 | `#page` を `前`/`次` の間へ移す場合は DOM 順を視覚行順どおりに書く（`前 → page → 次`）。行 wrapper 導入後も DOM 順＝読み順を維持 |
| U5 | 隣接 control gap ≥8px（spacing rhythm 4/8） | `form { gap: .4rem }` = `6.4px`（`card` は `.5rem` = 8px を使用 → rhythm 不一致） | 制御行 gap を `8px`（`.5rem`）へ統一。mock 実測で 3 行すべて fit（row1 `465px` flex fill / row2 `465px` flex fill / row3 必要幅 `306px` ≤ `465px`） |
| U6 | 同一レイヤの control 寸法・baseline 統一 | input height `29px`・button `31px`（`align-items: center` でも 1px drift） | `input, button { box-sizing: border-box; min-height: 31px; }` |
| U7 | 単一 group は shrink-0（分割されない）、collection は flex-wrap | `#search-prev`/`#search-next` が兄弟 flex item で個別に break 可 | pager を単独 flex item（`<span class="pager">`, `flex-shrink: 0`）化。実測 group 幅 `164.7px`（min-width 40px の `前`/`次` + `#page` 69px + gap 16px）。これにより前・次の隣接が構造で保証される |
| U8 | 現在位置の視覚表示 | `#search-page-status`（`page 6 / 1317・total 32918`）は form 外の meta 行 | pager 行と同じ行に置くとページ番号と操作が同一読み域になる（status 必要幅 `167px` ≤ `465px`） |
| U9 | 状態変化は layout bounds を動かさない（opacity/color で表現） | 現状 animation なし | disabled は opacity 変更のみ（transform 不使用）で周囲の jitter なし |
| U10 | Inputs に label（placeholder-only 不可） | `#query` は placeholder のみ、`#lang`/`#page` に accessible name なし（`labels` なし・`aria-label` なし） | 範囲外（本項目のレイアウト変更では解決しない）。記録のみ: `docs/ai/known-issues/KI-20261105-08-search-form-controls-missing-accessible-names.md` |

Scope note: skill の tap-target（≥44pt iOS / ≥48dp Android）と safe-area は App UI 限定。本アプリは desktop web（dark のみ）なので、touch-target 規則は適用せず、8px gap を spacing rhythm として適用。light/dark parity は単一 dark theme のみ実測（`#e8eaed` on `#14161a` = `15.03:1`、meta `#9aa0a6` = `6.86:1`、status `#8ab4f8` = `8.59:1`）。

### 再検討後の推奨（案B refined）

- 行1: `#query`（`flex: 1 1 160px; min-width: 0`）+ `#lang` + `#search-submit`
- 行2（pager 行）: `<span class="pager">`（`前` `#page` `次`・単独 flex item・`flex-shrink: 0`・`前`/`次` `min-width: 40px`）+ `#search-page-status`
- 行3（filters 行）: 4 checkboxes
- 制御: `gap: 8px`、`min-height: 31px`、`:focus-visible` ring（U2）、`:disabled { opacity: .5; cursor: not-allowed; }`（U1）、境界色は U3 の判断に従う
- mock 実測: form 全体 `102px`（mock 容器 `117px`・padding 8px 含む）、pager group `164.7px`、`前`/`次` `40px`・`#page` `69px`・`#search-submit` `48px`、status `167px`（切り詰めなし）
- mock スクリーンショット（Playwright browser 画面・案の mock であり実装確認ではない）: `docs/ai/tmp/search-prev-next-layout-mock-proposed.png`
- 現行（is Before）: `docs/ai/tmp/search-prev-next-layout-current.png`

テスト影響（refined 案）: `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit` の「form 内 button 3個」照合は pager を form 内 wrapper に置けば合格。`TestEmbeddedUIHasSearchPrevNextButtons`（id・`setSearchPage` 本文）は無影響。新固定テスト案: pager wrapper の単独 flex item 性（`class="pager"` と DOM 順 `前 → page → 次`）＋ U1/U2 の CSS 規則存在。`:focus-visible`/`:disabled` 追加は既存 harness（`search_prefetch_harness.mjs` L23 exact string）へ影響なし。

### 判断保留（ユーザー承認が必要）

1. 案A（単一 control 行＋filters 行）か案B（pager 行独立）か。案B refined は form height が 68px → 約102px になり、正典 criteria の `form#search` 単一行項目の re-anchor を要る（案A は `57px` で数値基準側が動く）。
2. `#page` を `前`/`次` の間に置く DOM 順変更（viewer nav と揃える）。現行順は `page → 前 → 次`。U4（tab order = visual order）の観点では、変更後も DOM 順＝読み順で書くことが必須。
3. pager を form 内の単独 flex item（`<span class="pager">`・`flex-shrink: 0`）化するか（隣接の構造保証のため推奨）。form 外へ移す場合は button 3個照合の re-anchor が発生。
4. disabled の視覚化（U1: `opacity: .5` + `cursor: not-allowed`）を追加するか。現行は `button { cursor: pointer }` が disabled にも適用され、見た目が enabled と同一。
5. focus indicator（U2）を正典 CSS に明記するか。UA既定 ring は実測 `1px`・`1.52:1` で skill 基準（≥2px・3:1）未達。`#8ab4f8` 2px + offset 2px へ。
6. control 境界（U3）を識別子として 3:1 に引き上げるか（`#6b7280` = `3.75:1`）。現状の border-less button 見た目を変えるため、見た目の変更承認が別途必要。境界を変えず focus ring で補完する案も可。
7. 制御行 gap を 8px（`.5rem`）へ統一し、`min-height: 31px`（U5・U6）で input/button の寸法を揃えるか。
8. `←` / `→` による検索ページのページ送りを追加するか（任意・範囲外）。
9. 範囲外として記録した U10（`#query`/`#lang`/`#page` の accessible name 欠如）を本項目で扱うか、KI-record のまま別項目にするか。

### 承認（ユーザー 2026-11-05 14:08・「推奨で承認する」）

実行固有ID: 未発番（本項目は 2026-11-05 検討 task 直結。正典上の実行固有IDは使用しない）。承認内容は案B refined（上記「再検討後の推奨」）＋ U1・U2 の CSS 追加。実装開始指示は command `implement-items`（2026-11-05 14:08）で受領。

承認範囲（`app/search.html` の検索 form の DOM 構造と CSS のみ）:

- 3 行構成: search 行（`#query` `flex: 1 1 160px; min-width: 0` + `#lang` + `#search-submit`）/ pager 行（`<span class="pager">` = `前` `#page` `次` + `#search-page-status` を同じ行の右）/ filters 行（4 checkboxes）
- pager は form 内の単独 flex item（`flex-shrink: 0`）で、`前`/`次` の隣接を構造で保証。`前`/`次` は `min-width: 40px`
- DOM 順＝視覚読み順（tab order = visual order）。pager 内の順は `前 → #page → 次`（U4）。`#page` は `form#search` 内に残す
- 制御規則: 制御行 gap `8px`（`.5rem`）、`input, button { box-sizing: border-box; min-height: 31px; }`、`:focus-visible { outline: 2px solid #8ab4f8; outline-offset: 2px; }`（U2）、`button:disabled { opacity: .5; cursor: not-allowed; }`（U1）。状態変化は opacity/color のみ（transform 不使用・bounds を動かさない）

承認範囲外（この項目では変更しない）:

- U3 の境界色（`#6b7280` 等）は**変更しない**。現状の border-less 見た目を維持し、focus ring で補完する（推奨に記載した選択のうち「境界を変えない」側を選択。変更するには別途承認）
- `←` / `→` による検索ページのページ送り（判断保留8）→ 未承認・実施しない
- U10（accessible name 欠如）→ `docs/ai/known-issues/KI-20261105-08-search-form-controls-missing-accessible-names.md` のまま（別項目）
- viewer 側・TN/full cache・route・prefetch ロジック・server 変更なし。`data-search-id`・card height contract・`.side`/`.main` 規則は維持

合格条件・検証方法は個別タスク記録の記載（テスト影響・画面確認）に従う。正典 criteria の `form#search`「単一行・wrap なし」項目は、本項目の行契約（search 行・pager 行・filters 行・各行必要幅 ≤ `.side` 可用幅 `465px`）へ re-anchor する（新しい decision entry で記録。criteria 本体は書き換えない）。

### 実装（2026-11-05 14:17 着手・案B refined 承認済み）

変更ファイル: `app/search.html` のみ（検索 form の DOM 構造と CSS。viewer 側・route・prefetch ロジック・server は無変更）。

CSS（正典の `form` / `input` / `button` 規則は維持し、検索 form へ scoped）:

- `form#search { gap: .5rem; }`（制御行 gap `8px`）
- `form#search .search-row, form#search .filter-row { flex-basis: 100%; display: flex; flex-wrap: nowrap; gap: .5rem; align-items: center; }`（行 wrapper = 単独視覚行。行内 wrap なし）
- `form#search .pager { flex-shrink: 0; display: flex; flex-wrap: nowrap; gap: .5rem; align-items: center; }`、`form#search .pager button { min-width: 40px; }`（単独 flex item・`前`/`次` の隣接を構造で保証）
- `form#search input, form#search button { box-sizing: border-box; min-height: 31px; }`（U6）
- `form#search input:focus-visible, form#search button:focus-visible { outline: 2px solid #8ab4f8; outline-offset: 2px; }`（U2）
- `form#search button:disabled { opacity: .5; cursor: not-allowed; }`（U1。transform 不使用・bounds を動かさない）
- `form#search #query { flex: 1 1 160px; min-width: 0; }`、`form#search #search-page-status { margin-left: auto; }`

判断（段階1・scoped）: U1/U2/U6 の selector を page global ではなく `form#search` へ scoped。根拠: 承認範囲は「検索 form の DOM 構造と CSS のみ」であり、global 適用は `.card .badge button`（card height contract 正典）、`nav` の `#jump-first-status`、`form#saved-search` へ波及する。正典の card height contract・`.side`/`.main` 規則・境界色（U3 現状維持）を壊さないために scoped。実測で scoped の効き目（`min-height 31px`・focus ring・disabled）を確認した。

DOM（`form#search` 内・DOM 順 = 視覚読み順）:

- search 行 wrapper `.search-row`: `#query` → `#lang` → `#search-submit`
- pager 行: `<span class="pager">`（`前` → `#page` → `次`）→ `#search-page-status`（同じ行の右。form 内へ移動）
- filters 行 wrapper `.filter-row`: 4 checkboxes（`status-viewed` → `read` → `saved` → `favorite`）
- `#page` は `form#search` 内に残す（`<span class="pager">` 内）。`form#search` 内 button は 3 個（`#search-submit` のみ `type="submit"`）→ `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit` の「3 buttons」照合は維持。既存 test ファイルは無変更（test 追加なし）。

正典 criteria の `form#search`「単一行・wrap なし」項目は 3 行契約へ re-anchor（新しい decision entry: `docs/ai/decisions/2026-11-05-search-form-row-contract-re-anchor.md`。criteria 本体は書き換えない）。

## Verification

- 正典読了（作業前チェック）は上記ファイル名で記録済み。
- 測定は canonical exe（byte 照合合格 2 passed）と正典 server（port 8731・PID 27220）で実施。UI 表示変更なし（調査のみ）のため画面確認合格条件は未適用。
- 実測値（fresh tab・actual window size）: inner `1920x897`、`.side` `480px`、`.side` 可用幅 `465px`、`form#search` height `68px`（2 視覚行）、prev `x=442..475`（行1右端）、next `x=16`（行2左端）、children 必要幅 `844.6px`。
- クローン probe 測定（DOM 操作は measurement のみ・ファイル変更なし）: 案A 単一行 fit 必要幅 `449px`（`min-width:0` 必須・`471px` では wrap）、案B 各行 `396.8px` / `221.2px` / `217px` ≤ `465px`。
- スクリーンショット（通常スクリーンショット正典・Playwright browser 画面）: `docs/ai/tmp/search-prev-next-layout-current.png`（現行の分割位置を記録）。
- direct server baseline: `GET /api/search?query=furry&lang=all&page=1&limit=25` → `200`、`X-Search-Timing: search=1751;record=36;json=1`、ids 25。
- console error は `favicon.ico` 404 のみ（本項目と無関係）。

再検討（2026-11-05 13:58・ui-ux-pro-max）で追加実測:

- server: 正典 exe のまま。session-scoped process が session 切り替えで消えたため正典 command で再起動（PID `28056`・lifetime persistent・port 8731 listening 確認）。canonical build freshness 正典の byte 照合は `python -m pytest tests/test_canonical_build_freshness.py -v` = 2 passed。
- focus indicator: `#search-next` focus 時 computed outline `rgb(16, 16, 16) auto 1px`、`boxShadow: none`（規則なし＝UA既定）。`#101010` vs button surface `#2f343b` = `1.52:1`（scratch `docs/ai/tmp/prev_next_contrast3.py`）。`button { cursor: pointer }` は disabled に適用され `cursor: not-allowed` 相当がない（`button:disabled` 規則なし）。
- control 境界 contrast（scratch `docs/ai/tmp/prev_next_contrast.py`・`prev_next_contrast2.py`）: `#2f343b` `1.44:1`、input surface `#1d2126` `1.12:1`、text `#e8eaed` on `#14161a` `15.03:1`、on `#2f343b` `10.40:1`。3:1 を満たす境界は `#6b7280`（`3.75:1`）/ `#606a73`（`3.28:1`）。focus ring `#8ab4f8` は `8.59:1`（page 比）・`5.95:1`（button surface 比）。disabled（opacity .5）は境界 `1.45:1`・text `3.89:1`。
- tab order 照合: DOM order `query, lang, page, search-prev, search-next, search-submit, status-viewed, status-read, status-saved, status-favorite`。現行 visual order（行1 = query/lang/page/前、行2 = 次/検索/labels）と一致。
- mock（measurement のみ・ファイル変更なし・測定後に DOM から除去）: 3 行 + gap 8px + min-height 31px + pager 単独 item で、row1 `465px`（query 305px flex fill）・row2 `465px`（pager group `164.7px` + status `292px`）・row3 `306px` ≤ `465px`、form 全体 `102px`。`前`/`次` `40px`、`#page` `69px`、`#search-submit` `48px`。
- mock スクリーンショット: `docs/ai/tmp/search-prev-next-layout-mock-proposed.png`（案の mock。実装確認ではないので screen confirmation 合格条件は未適用）。
- accessible name 欠如（U10）は範囲外 problem として `docs/ai/known-issues/KI-20261105-08-search-form-controls-missing-accessible-names.md` に記録。

実装確認（2026-11-05 14:23・案B refined 実装後）:

- 正典 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（`app`）で canonical exe を更新。
- `go vet . && go test -count=1 -timeout 300s .`（`app`）= clean / `ok hitomi-hub/app 24.716s`。`TestEmbeddedUIHasSearchPrevNextButtons`・`TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit` 合格（既存 test ファイルは無変更・test 追加なし）。
- `python -m pytest tests/ -v`（hitomi-hub root）= 53 passed（`test_canonical_build_freshness.py` 2 passed 含む。`test_search_prefetch.py`（`#search-next` exact string）・`test_search_visible_priority.py`・`test_read_lists_debounce.py`・`test_strip_rebuild.py` 影響なし）。
- canonical server: 記録上の PID `28056` は hub.exe ではなく `kilo.exe`（PID 再利用）で実 server として停止済み。実 listener は PID `26460`（正典 command だが pre-change exe）＝ stale。正典 tracked entry を stop し、実 listener `26460` も child tree として停止（無関係 listener は停止していない）。新 exe で正典 command を起動: PID `31028`・lifetime persistent・port `8731` listening（`viewer: http://127.0.0.1:8731`）。
- direct server baseline（先行）: `GET http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25` → `200`・`Cache-Control: no-store`・`X-Search-Timing: search=1609;record=51;json=1`・`Date: Thu, 05 Nov 2026 14:23:08 GMT`・`Content-Length: 690`（stale shape `84` でない）・`total=32918`・`ids.length=25`。page 7 baseline: `200`・`X-Search-Timing` あり・`Content-Length: 974`・`ids.length=25`。
- actual UI event（fresh tab・人工 resize 不使用）: `#search-next` click → `#page` `7`・hash `#/q/furry/page/7`・foreground fetch `/api/search?query=furry&lang=all&page=7&limit=25`・`#results` card 数 `25` = `ids.length` `25`・`#count` `32918 件・page 7`・`#search-page-status` `page 7 / 1317・total 32918`。`#search-prev` click → `page` `6` へ戻り（`setSearchPage` 非破壊）。fresh target `http://127.0.0.1:8731/search#/q/furry/page/1`（page 1）: `#search-page-status` `page 1 / 1317・total 32918`・`#count` `32918 件・page 1`・`#search-status` `検索完了・カード表示済み 25件`・`#results` card 数 `25`。console error `0`（`favicon.ico` 404 なし）。
- 3 行契約実測（actual window size outer `1663x1020`・content viewport inner `1650x927`・`devicePixelRatio` 2。`.side` `480px`（shrink 不可）、`clientWidth` `465px`、scrollbar `15px`）:
  - search 行 `y=119` `h=31` `w=465`（`#query` `307`・`#lang` `96`・`#search-submit` `46`・gap `8`×2）→ 必要幅 ≤ `465px`・行内 wrap なし
  - pager 行 `y=158`（`<span class="pager">` `w=165`・`flex-shrink: 0`・`flex-basis: auto`・行内 `flex-wrap: nowrap`）: `前` `x=16..56`（`min-width: 40px`）、`#page` `x=64..133`（`69px`）、`次` `x=141..181`（`40px`）→ `前` と `次` が `#page` を挟んで隣接（gap `8px`）。`#search-page-status` `x=314..481`（`167px`・pager 行の右端・同じ視覚行）
  - filters 行 `y=197` `w=465`・行内 wrap なし（必要幅 `306px` ≤ `465px`）
  - `form#search` height `116px`（`31 + 31 + 38 + gap 8×2`）。re-anchor 後の行契約で合格（criteria の `67〜70px` 単一行数値は本項目の 3 行契約へ re-anchor 済み: `docs/ai/decisions/2026-11-05-search-form-row-contract-re-anchor.md`）
  - pager 内 DOM 順 = `search-prev` → `page` → `search-next`。form children 順 = `search-row` → `pager` → `search-page-status` → `filter-row` = 視覚読み順（tab order = visual order）
- `:focus-visible`（keyboard Tab focus で実測）: `#search-next` = `outline rgb(138,180,248) solid 2px`・`outline-offset 2px`。`#page` focus でも同 ring。disabled button は sequential focus で skip（Tab 順: `#page` → `#search-next`）。
- disabled visual state（page 1 実測）: `#search-prev` `disabled=true`・`opacity 0.5`・`cursor not-allowed`・`transform none`（bounds を動かさない）。`#search-next`（enabled）= `opacity 1`・`cursor pointer`。click disabled guard: disabled button は click を dispatch しない（page 1 で `前` click は handler 到達不可）。
- スクリーンショット（Playwright browser 画面画像・通常スクリーンショット正典。OS foreground capture 不使用）: `docs/ai/tmp/search-prev-next-layout-after-20261105.png`（fresh target page 1。3 行構成・`前` disabled dim・focus ring・status 行右）。filters 行 checkbox 描画の証拠（element screenshot）: `docs/ai/tmp/search-prev-next-filter-row-20261105.png`。
- Drive task folder `I:\マイドライブ\kilo\2026-11-05-search-prev-next-button-layout` へ copy 済み。SHA256 一致で保存確認: `1298FB8D542E88E80905679590B3E1FF65D2F20155632B2E296278488106F42A`（local == Drive の `search-prev-next-layout-after-20261105.png`）。filter row 証拠 `14248E912D1B9BE9593B87D8923FE172E80F6FEF427CF03F95C472C31593C655`。
- 正典 `kilo-drive-confirm.ps1` 確認 record（承認内容どおりの content 文・合格条件は上の実測で充足）: `I:\マイドライブ\kilo\2026-11-05\search-prev-next-button-layout-confirmation-2026-11-05.txt`。

段階3 持ち帰り（この項目で解決していない判断）:

- 承認された制御規則 `input, button { box-sizing: border-box; min-height: 31px; }` を text input・button へ適用すると、filters 行の `input[type=checkbox]` が `13x31` の縦長 box として描画される（filters 行 `h=38`）。承認 mock（`docs/ai/tmp/search-prev-next-layout-mock-proposed.png`）は filters 行高さ `20px`・checkbox は通常の四角で、視覚上食い違う。checkbox を `min-height` 適用から除外するか現状を許容するかが判断未確定。実測（上）と記録確認では確定できず、ユーザーに見える操作感の変更（段階3）として持ち帰る。この項目の code は承認規則どおり（checkbox 除外なし）で commit 済み。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
