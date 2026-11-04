# 検索ページの閲覧名残とレイアウトの検討（assess-20261104-1004）

**Status**: 完了
**Date**: 2026-11-04
**Next Action**: 実装（案A+案B）・フォアグラウンド委譲1件（合格条件・検証方法は本記録の記載）

## 意図 / 背景

実行固有ID: `assess-20261104-1004`（発番 2026-11-04 10:04）

ユーザー指示: 「検索画面に閲覧画面があった時の名残がある。よりよいUIを検討」。

A1（2ページ分割・コミット `727d9fb`）で `app/ui.html` を `app/viewer.html` / `app/search.html` へ分割した。分割は領域の分離のみで、検索ページの DOM・CSS・click handler は単一ドキュメント時代（閲覧ペインが同一 DOM にあった）の形のまま。本検討は名残の特定とレイアウト案まで。ソースは無変更（記録のみ）。

正典読了（viewer/search に触る項目の作業前チェック）:
- `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

## 調査結果 / 変更内容

### 名残（検索ページ）

| # | 名残 | 証拠 |
|---|---|---|
| S1 | 閲覧ペイン用の `.main` 半面が空。`.side`（30rem 固定）に form・`#results`・一覧が全部入り、`.main` は「閲覧」section の 2 行（hint・error）だけ | `app/search.html:9-11,74-80` |
| S2 | `#gallery` は reader 由来の id。検索ページでは静的 hint 1 行（`カードをクリックすると viewer で開きます`）のみ | `app/search.html:77` |
| S3 | `#note` は error 表示専用（reader の note の名残）。検索側では `runSearch` と saved-search でしか書かれない | `app/search.html:78,388,420` |
| S4 | `searchCardCache` は write-only。単一ドキュメント時代は `openGallery()` の高速起動に使われており、viewer 分離後に参照元が消えた | declare `app/search.html:108`、set `:172`、参照（`.get`）ゼロ |
| S5 | `handleCardClick` と `data-id`/`data-page`。reader が同一 DOM にいた時代の in-page 遷移 shim。検索カードは常に page 1 で、plain anchor（`route()`＋`targetAttr()`）だけで遷移可。modifier-key passthrough も plain anchor が自然に担保 | `app/search.html:179,232,378,398-409,422` |
| S6 | `route(id, page, second)` の `second` 引数は検索側で不使用（viewer hash 形式の流用） | `app/search.html:123-126` |
| S7 | `newTabHref` が検索側で未呼出。fragment-only 遷移で `target` が効かない時代用。cross-page 絶対リンクでは plain anchor + `target='_blank'` で十分 | `app/search.html:119-121`（call site なし） |
| S8 | `.cover` CSS が未使用（検索 DOM は `.search-card-cover`のみ） | `app/search.html:25-26` |

### 名残（閲覧ページ）

| # | 名残 | 証拠 |
|---|---|---|
| V1 | `.card` / `.card .text` / `.cover, .search-card-cover` / `--search-card-height` CSS が viewer に未使用（viewer は card を render しない） | `app/viewer.html:23-27,32`、`class='card'` なし |
| V2 | `.side` CSS と fullscreen 規則 `html.fullscreen .side` / `body.pseudo-fullscreen .side` が viewer に未使用（viewer に `class="side"` 要素がない） | `app/viewer.html:10,85` |
| V3 | `api` helper の `data._x_search_timing` が viewer で書き込みのみ・読取ゼロ（perf 表示は検索側） | `app/viewer.html:158` |
| V4 | viewer に `/search` への恒常的な nav がない。`#gallery` の静的 hint 内のリンクは gallery を開いた時点で meta 書き込みで消える | `app/viewer.html:108`（`showGalleryMetaFromCache` が `#gallery` を上書き） |

### よりよいUI案

案A（最小・名残除去＋レイアウト是正）:
1. `#results` を `.main` へ移し、`.side` は form・保存検索・お気に入り/未読のみ（左 sidebar = 入力と一覧、右 = 結果）。CSS は `.side`/`.main` を維持 → `TestEmbeddedUIIndependentScrollPanels` の照合は維持。
2. 「閲覧」section を削除。header（`h1` 行）に cross-page nav（検索側 `→ 閲覧`、viewer 側 `→ 検索`）を1本。`#gallery`・「閲覧」見出しを削除。
3. error 表示を status 行へ寄せる（`#note` を削除し `#search-status` の1行、または `#error` へ改名）。
4. 削除: `searchCardCache`、`data-id`/`data-page`、`handleCardClick`、`route` の `second` 引数、`newTabHref`（検索側）、`.cover`。
5. viewer 側: `.card` 系 CSS・`--search-card-height`・`.side` CSS・fullscreen `.side` 規則・`_x_search_timing` を削除。
6. 検索カードは plain anchor のみ（`/search` の `#/q/...` hash は維持）。`data-search-id` は lazy cover 用に残す（`TestEmbeddedUIShowsSearchExecutingStatus` L1608 の selector）。

案B（案A + 読みやすさ）: 案Aに加え
- 卡片に既読/お気に入り badge。`showLists` が既に `/api/lists` の `favorites`/`unread`（`gallery_id`・`last_page`・`fully_read`）を持つため、server 変更なしで client 側 map から表示可。正典（一覧の所有は検索ページ・ページ間同期なし・表示更新は手動で許容）と整合。
- 「一覧更新」ボタン（手動 refresh）を `showLists()` の再実行として追加（正典の「表示更新は手動で許容」の具現化）。

案C（任意）: form sticky 化、結果リスト virtualization。今回不要（正典の fast nonblocking policy と衝突しないが規模対効果Thin）。

### テスト影響（re-anchor 6〜8件）

- `app/viewer_test.go` L1529（`newTabHref` を両ファイルで assert）、L1547-1578（`handleCardClick` を両ファイルで assert）→ 検索側は plain anchor + `targetAttr()` へ書き換え。
- L1927（`searchCardCache` 存在）→ 案Aでは削除 assert へ、案Bでは badge 用 map へ re-anchor。
- L2094・L2470（`function route(id, page, second) {` を両ファイルで assert）→ 検索側の签名変更。
- L2567・L2570（fullscreen `.side` 規則）→ viewer 側削除なら該当 anchor 書き換え。
- L1592（`.side`/`.main`）→ 案Aは CSS 維持で pass。
- `tests/read_lists_debounce_harness.mjs` L72 の `note` mock は viewer read helper 用（検索 lists block は `queries`/`favorites`/`unread` のみ）→ 検索側 `#note` 改名・削除は harness 影響なし（未確認点: 案Bで badge 表示を追加すると harness の mock element 追加が必要）。
- `tests/test_canonical_build_freshness.py`: 2 html byte 照合 → 変更後再実行＋canonical 画面確認（viewer 基準＋search served 画面）が必要。

### 判断保留（ユーザー承認が必要）

1. レイアウト: 案A（結果を右 `.main`、一覧を左 sidebar）か、現行のまま（結果を 30rem 列）か。
2. error 表示: `#note` を status 行へ寄せるか、`#error` として独立させるか。
3. 案B（既読 badge・手动 一覧更新ボタン）を第1版に含めるか。
4. `handleCardClick` を検索側から削除して plain anchor 一本にするか（新タブ policy は `targetAttr()` で維持）。

実装開始指示は未受領。ソースは無変更。

### 承認（ユーザー 2026-11-04 10:10）

案A+案B を選択。正典: `docs/ai/decisions/2026-11-04-search-page-layout-and-vestige-cleanup-approved.md`。上記1〜4は同 decision の設計項目として確定（レイアウト是正・error を status 行へ・badge・一覧更新ボタン・plain anchor 一本化）。実装開始指示は command `implement-items`（2026-11-04 10:10）で受領。

## 合格条件（実装）

1. `app/search.html`: `#results` が `.main` 直下。`.side` は form・保存検索・お気に入り/未読のみ。`.cols`/`.side`/`.main` の CSS 規則は維持し、`1280x900` で `.side` 実測幅 `480px`（30rem）・shrink 不可、`form#search` は単一行（`67〜70px`）。
2. 削除: `searchCardCache`、`data-id`/`data-page`、`handleCardClick`、`route()` の `second` 引数、検索側の未呼出 `newTabHref`、未使用 `.cover`、`#gallery` hint section、`#note`（error は status 行、色規則は `.error`）。
3. header に恒常的な cross-page nav: search → `/`（閲覧）、viewer → `/search`（検索）。viewer 側は gallery meta 書き込みで nav が消えない（meta と nav を分離）。
4. 案B: `#results` 卡片に既読/お気に入り badge（`/api/lists` の `favorites`/`unread` から client 側 map、server 変更なし）。「一覧更新」ボタンは `showLists()` の再実行（BroadcastChannel 入れない・表示更新は手動で許容）。
5. `app/viewer.html`: 未使用 CSS（`.card`、`.card .text`、`.cover, .search-card-cover`、`--search-card-height`、`.side`、`html.fullscreen .side`/`body.pseudo-fullscreen .side`）と `api` helper の `_x_search_timing` 書き込みを削除。fullscreen chrome 非表示は nav 行と `#reader-controls` を明示 selector で。
6. 正典の不变条件を維持: TN は search-card cover 専用（full objectURL 優先なし、`currentPriorityDone` 待ちなし、TN display fallback 禁止）、永続 full cache 経路と `imageProxy` 順序、`no-store` fetch、`x-cache` 判定、世代 guard、SQLite 単一接続、`embed` 同梱（追加ファイルなし）。card height contract（cover `130px`、`--search-card-height` が card background を制御）は検索側で維持。
7. テスト: re-anchor 済みの `go vet . && go test -count=1 -timeout 300s .`（app）、`python -m pytest tests/ -v`、`node tests/strip_rebuild_dom_harness.mjs`、`node tests/read_lists_debounce_harness.mjs` がすべて合格。
8. 画面確認（canonical build・正典条）: 正典 exe の 2 html byte 照合合格。search served 画面で card 数 = server `ids.length`、cover は TN（`/api/thumb/...&variant=bigtn` HTTP 200、`/api/image` 呼出しなし）、badge 表示、一覧更新ボタン動作。viewer 画面で reader img 表示（`naturalWidth/Height` 実測）と nav 双方向リンク。キャプチャは Google Drive のタスク別フォルダへ保存。
9. 作業前チェック（viewer に触る項目）: 3 decision（`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`）読了を記録に明記。CLI download で実在 gallery の attempted = written。`/api/image/<hash>?format=webp` が 200・`x-cache: miss|cache|local`・`no_full` でないこと（バイト数記録）。

## 検証方法

- `go vet .`、`go test -count=1 -timeout 300s .`（`app`）
- `python -m pytest tests/ -v`
- `node tests/strip_rebuild_dom_harness.mjs`
- `node tests/read_lists_debounce_harness.mjs`
- canonical build + canonical server `http://127.0.0.1:8731` での served 画面確認（fresh tab・resize 前・server baseline 先行）
- CLI download（canonical exe・実在 gallery・`--limit 1`）


## Verification

正典読了（作業前チェック・ファイル名明記）:
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

実装（案A+案B）:
- `app/search.html`: `#results` を `.main` 直下へ、`.side` は form・保存検索・お気に入り/未読のみ、header に恒常的な nav（`/` へ）、`#note`・`#gallery` hint・閲覧 section 削除、`.note` → `.error`、`setStatus(status, message, isError)` で error を status 行、未使用 `.cover` selector 削除、`searchCardCache`・`data-id`/`data-page`・`handleCardClick`・`route(id, page, second)`・`newTabHref`（検索側）削除、plain anchor `viewerLink(id, page)` + `targetAttr()`、`readStateByGallery`/`favoriteByGallery`・`badgeText()`/`badgeRow()`/`renderReadBadges()`、`#lists-refresh`（`form#search` の外）
- `app/viewer.html`: header に恒常的な nav（`/search` へ）と `#gallery` meta 書き込みの分離、`#note` 削除（error は `setPrefetchStatus`/`displaySource`）、未使用 CSS 除去、fullscreen chrome selector を `nav.page-nav` と `#reader-controls` へ、`api` helper の `_x_search_timing` 削除
- `app/viewer_test.go`: L1529・L1547-1578・L1927・L2567-2570 re-anchor、`.cover` selector 除去の assert、新 test 3件
- `tests/read_lists_debounce_harness.mjs`: 無変更（badge helper は `if (!results) return;` guard。mock element 追加不要）

結果（2026-11-04）:
- `go vet .`（app）: 合格（出力なし）
- `go test -count=1 -timeout 300s .`（app）: ok（15.342s。初回実行で `mainBody` の raw string に `\n` が実改行でなく slice 失敗 → interpreted string へ修正後合格）
- `node tests/strip_rebuild_dom_harness.mjs`: 18 checks passed
- `node tests/read_lists_debounce_harness.mjs`: 16 checks passed
- `python -m pytest tests/ -v`: 50 passed・1 failed（`test_canonical_build_freshness_rule_exists`。AGENTS.md 正典条のポインタ化 2026-11-04 による既存の不一致で本項目 scope 外。AGENTS.md は write-protected）。`test_canonical_executable_embeds_current_pages`（正典 exe の 2 html byte 照合）は PASSED（8.58s）

正典ビルド・実機確認（canonical server `http://127.0.0.1:8731`、root `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh`）:
- 正典 exe 再ビルド後、fresh tab（viewport 1280x900・resize 前）で `.side` 実測 480px、`form#search` height 67.7734375px（単一行）、`#results` card 25 = server `ids.length` 25、`.main` 直下、cover 130x130、`.card` computed height 130px（`--search-card-height` が card background を制御）、`#search-page-status` `page 1 / 1317・total 32902`、`#count` `32902 件・page 1`、X-Search-Timing 一致、stale shape `84` でない
- 検索 page の TN: `/api/thumb/...&variant=bigtn` 全て 200、`/api/image` 呼出し 0 件
- badge 実測: `fav --id 4303732` → card `お気に入り・p1`、`read --id 4303592 --page 3` → card `p3`（`#lists-refresh` クリックで反映）。保存検索 save は `POST /api/query` 200。prev/next で page 2 card 25（page 2 baseline ids.length と一致）
- error path（`#page` に 0 を submit）: `#search-status` class `status error`・color rgb(240,164,65)・`検索エラー: 400 /api/search?query=furry&lang=all&page=0&limit=25`
- viewer（fresh tab）: nav `→ /search`、`#gallery` meta 書き込み後も nav 残存、viewer に `.side`/`#note` なし、reader img naturalWidth/Height 1440x2036（objectURL）、`prefetch-status` `actual pixels 13/13 | local direct 0/13 | objectURL cache 13/13 | no full 0/13 | TN display fallback disabled`、双方向 nav クリックで `/search` と viewer へ遷移
- `/api/image/e0478dc469b6f5a38d32f216283c3738396e4f07f94ac47224a315acefcfcf17?format=webp`: 1回目 200 `X-Cache: miss`、2回目 200 `X-Cache: cache`・Content-Length 269410（`no_full` でない）
- CLI download（正典 exe・`--id 4303732 --limit 1`）: attempted 1 = written 1（skipped 0・not_evaluated 1）

Google Drive 確認結果: `I:\マイドライブ\kilo\2026-11-04`
- `search-page-layout-and-vestige-cleanup-search-2026-11-04.png`
- `search-page-layout-and-vestige-cleanup-viewer-2026-11-04.png`
- `search-page-layout-and-vestige-cleanup-confirmation-2026-11-04.txt`

未確認点（持ち帰り）:
- decision L2094・L2470 は `function route(id, page, second) {` を両ファイル assert と記録だが、実テストは `uiText(t, "viewer.html")` の viewer のみ。検索側は新 test で `viewerLink(id, page)` を assert した（記録と実コードの不一致）。
- fullscreen chrome の nav/control 非表示は selector と固定 test で確認。実際の fullscreen 実行での nav 非表示は browser-linked criteria の実測では確認していない。

### 残件検証（2026-11-04・実作業は検証のみ・ソース無変更）

1. browser-linked fullscreen 実測（normal OS screen capture）: **合格**
- 環境: canonical server `http://127.0.0.1:8731`（正典 exe `output/hitomi-hub/hub.exe`・`view --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh`）。fresh normal browser window・fresh profile（Edge `--new-window`・`--user-data-dir=...\edge-fresh-20261104-fullscreen-1052`）、初期ウィンドウ `1280x900`（outer 実測 1280x900・inner 1259x810）、物理 screen `1920x1080`、runtime `window.resize` 不使用。server baseline 先行: `GET /api/gallery/4303732` → 13 files・format webp・local 1（UI 表示 `Dancer's Bond japanese / doujinshi`・`13 files・local 1 (format webp)` と一致）。
- fresh profile の first-run 同期ダイアログは before capture 前に UI OK クリックで閉じた（記録）。
- before（`f` key 押下前）: browser chrome 可见（tab strip・address bar `127.0.0.1:8731/#/g/4303732/page/1-2`）で fullscreen ではない。`fullscreenElement` null・`fullscreenEnabled` true・reader class `reader double`・img 2（naturalWidth 1440）・`nav.page-nav` display block・`#reader-controls` display block・activeElement BODY。
- 操作: reader 画面内容 (650,550) への OS レベル mouse click（activeElement BODY 確認）→ OS key injection（SendKeys）で `f` を page へ delivery（address bar への入力ではない）→ fullscreen 遷移即（0ms）。
- after 実測: `document.fullscreenElement === documentElement` true。after capture で browser chrome 隠れ（tab strip・address bar なし。Edge の一時的 fullscreen 通知 toast のみ）、page content が全 physical screen（1920x1080）を占める。`#reader` rect [0,0,1920,1080]（fullscreen padding 12px）。double display: img box left [12,12,948,1056]・object-position `100% 50%`（右寄せ）、right [960,12,948,1056]・`0% 50%`（左寄せ）、`object-fit: contain` 双方・aspect ratio 維持、center seam gap `960 − (12+948) = 0`。本項目追加 chrome selector: fullscreen 時 `nav.page-nav` display none・`#reader-controls` display none（before は block、capture 目視一致）。
- 退出: `Escape` で actual fullscreen exit（state sync）→ last layout mode `reader double` 復旧。criteria 10（fullscreen 中 ArrowLeft/Right）は本残件スコープ外。
- 上記により「未確認点」の browser-linked fullscreen 実測残件は解消（合格）。
- Google Drive 保存先（タスク別フォルダ `I:\マイドライブ\kilo\2026-11-04`）:
  - `search-page-layout-and-vestige-cleanup-fullscreen-before-2026-11-04.png`（chrome 可见・非 fullscreen・before）
  - `search-page-layout-and-vestige-cleanup-fullscreen-after-2026-11-04.png`（chrome 隠れ・全 physical screen 占拠・seam gap 0・after）
  - `search-page-layout-and-vestige-cleanup-fullscreen-confirmation-2026-11-04.txt`（手続き。normal OS screen capture（CopyFromScreen）による before/after 2 枚＋確認結果。Headless/Playwright screenshot・proxy 不使用。DOM 数値 read は測定のみの CDP evaluate）

2. `test_canonical_build_freshness_rule_exists`（FAILED）の性質確認: 既存不一致（本項目 scope 外）で確定
- テスト assert（`tests/test_canonical_build_freshness.py:13-26`）は AGENTS.md 本文の直接記載を文字列照合: (a) `tests/test_canonical_build_freshness.py`、(b) 正典 build コマンド literal `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`、(c) `UI confirmation`（case sensitive。AGENTS.md 見出しの `UI Confirmation` は不一致）。
- 実測再実行: FAILED（assert (a) で fail: "canonical build freshness test rule is not documented"）。
- 現在の AGENTS.md `## Canonical Latest Build / UI Confirmation Test Rule` 条はポインタのみ（参照先 `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`）。3 literal のいずれも不在 → 失敗原因は AGENTS.md の Canonical rule 条のポインタ化（`docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`・ユーザー承認 2026-11-04 09:32・コミット `9c6161b`）由来。
- `git show --stat 01baa01`: 5 ファイル（`app/search.html`・`app/viewer.html`・`app/viewer_test.go`・decision・本タスク記録）で `AGENTS.md` を含まない → 本項目の変更由来ではない。
- 判定: 性質 = ポインタ化による既存の test-rule 不一致（decision が条内容を持ち、テストは AGENTS.md 本文文字列を引き続き要求）。修正（テストを decision ポインタへ re-anchor、または AGENTS.md 直接記載の扱い変更）は正典 rules 側の変更・AGENTS.md write-protected・本項目承認範囲外 → 修正せず持ち帰り。本項目合格条件（pytest byte 照合項目）には影響なし（`test_canonical_executable_embeds_current_pages` は 2026-11-04 実測 PASSED）。


### 残件処理（2026-11-04・ユーザー承認 11:12）

- 残件2: `docs/ai/decisions/2026-11-04-search-page-layout-and-vestige-cleanup-approved.md` の記録補足（既記載のまま）を単独コミット `acbf262`。push 済み。
- 案1 re-anchor（`tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`）: AGENTS.md 本文3 literal 照合 → 次へ変更
  1. AGENTS.md の正典 decision 参照行（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`）照合
  2. 参照先 decision の実質内容（項目4の3点 literal: `app/viewer.html`・`app/search.html`・`失敗メッセージには両ファイル名`・`served 画面`）照合
  3. build command 正典記載を decision へ統一: `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md` へ `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` 追記・テストは同 decision 照合。`docs/ai/usage-memo.md` は照合対象外
  - 失敗メッセージは対象ファイル名明示。byte 照合・`test_canonical_executable_embeds_current_pages` は無変更。AGENTS.md 無変更（write-protected）。design determination: `docs/ai/decisions/2026-11-04-freshness-rule-test-re-anchor-to-pointer.md`（方法追随・supersede ではない）
- 検証: `python -m pytest tests/ -v` = 51 passed（10.94s・re-anchor 後の該当 test PASSED 含む、`test_canonical_executable_embeds_current_pages` PASSED）。`git diff` で AGENTS.md 変更なし、`*.db`・`tools/*.exe`・scratch/ の混入なし。UI 表示変更なし（documentation-presence 照合）のため画面確認不要・実施せず。
- コミット: `acbf262`（記録補足単独）／`6d76279`（test re-anchor + decision 追加・本記録追記）。push 済み（`e1e0cb6..6d76279`）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
