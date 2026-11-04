# 検索ページのレイアウト是正と閲覧名残の除去：案A+案B（assess-20261104-1004）

**Status**: 設計方針承認済み（ユーザー 2026-11-04 10:10・案A+案B 選択）
**Date**: 2026-11-04
**実行固有ID**: `assess-20261104-1004`
**調査記録**: `docs/ai/tasks/2026-11-04-ui-vestige-review-search-page.md`

## Supersede 対象

なし。`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` の決定（一覧の所有は検索ページ、小道具のページ別複製、hash namespace 単独、2ファイル照合）をそのまま前提とする。本記録は同じ方針上のレイアウト・名残除去の設計選択のみ。TN display policy（`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`）と browser-linked fullscreen criteria（`docs/ai/decisions/2026-11-02-browser-linked-fullscreen-criteria.md`）は変更しない。

## 承認された設計

### 検索ページ（`app/search.html`）

1. レイアウト: `#results` を `.main`（`flex: 1`）へ移す。`.side`（`30rem`・shrink 不可）は検索 form・保存検索・お気に入り/未読のみ。CSS の `.cols`/`.side`/`.main` 規則は維持（`getBoundingClientRect().width = 480px` 基準と `.side` shrink 禁止はそのまま）。
2. 「閲覧」section と `#gallery` hint を削除。`h1` 行に恒常的な cross-page nav（検索側 → `/` 閲覧、viewer 側 → `/search` 検索）を1本置き、viewer 側では gallery meta 書き込みで nav が消えない構造にする（`#gallery` の meta 書き込みと nav を分離）。
3. error 表示を status 行へ寄せる。`#note` を削除し `#search-status`（検索）・`#view-state`/`#prefetch-status`（viewer）の既存 status 行で表示。`.note` 色規則は error 用に `.error` へ改名。
4. 名残除去: write-only の `searchCardCache`、`data-id`/`data-page`、`handleCardClick`、`route()` の `second` 引数、未呼出の `newTabHref`（検索側）、未使用の `.cover`。
5. カード遷移は plain anchor 1本（`/#/g/<id>` + `targetAttr()`）。modifier-key passthrough と新タブ policy は plain anchor と `target='_blank'` で担保。`data-search-id` は lazy TN cover 用に残す。

### 案B（第1版に含める）

6. 卡片の既読/お気に入り badge: `/api/lists` の `favorites`（`gallery_id`・`last_page`・`fully_read`）と `unread` から client 側 map を作り、`#results` の既存カードへ表示。server 変更なし（正典「一覧の所有は検索ページ」上）。
7. 「一覧更新」ボタン: `showLists()` の再実行として追加。ページ間同期機構（BroadcastChannel）は入れず、正典の「表示更新は手動で許容」を具現化する。viewer で既読が変わった直後の staleness は手動更新で解消する前提。

### 閲覧ページ（`app/viewer.html`）

8. 未使用 CSS 除去: `.card`、`.card .text`、`.cover, .search-card-cover`、`--search-card-height`、`.side`、`html.fullscreen .side` / `body.pseudo-fullscreen .side`。`.card` 系契約（`130px x 130px` cover・`--search-card-height` が card background を制御）は検索ページ側で維持。
9. `api` helper の `data._x_search_timing` 書き込みを削除（読取元が検索側にしかない）。
10. fullscreen 時の chrome 非表示は header nav と control 行を明示 selector で隠す（`.side` 参照が消えるため、`h1`・`section h2`・`.meta` に加え nav 行と `#reader-controls` を chrome selector で显示的に非表示）。フルスクリーンでは nav が隠れることを許容（非必須 chrome 基準と整合）。

## 画面確認基準への影響（弱めない）

`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` の initial display 基準は `.side` = `480px`、`form#search` 単一行、`#results` card 数 = server `ids.length`、card height contract をそのまま適用。変更点は測定の DOM 位置のみ: `#results` は `.main` 直下、`.side` は form・一覧のみ。canonical build freshness 正典（`docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md` が指す条）で viewer 基準＋search served 画面確認が必要。

## テスト影響（re-anchor 6〜8件・実装時に反映）

- `app/viewer_test.go` L1529（`newTabHref` を両ファイル assert）→ 検索側は plain anchor だけで合格する形へ。
- L1547-1578（`handleCardClick` を両ファイル assert）→ viewer 側だけ handler、検索側は anchor 基準へ書き換え。modifier-key passthrough 基準は viewer 側で維持。
- L1927（`searchCardCache` 存在）→ 削除。案B の read/fav map（`readStateByGallery`/`favoriteByGallery`）へ re-anchor。
- L2094・L2470（`function route(id, page, second) {` を両ファイル assert）→ viewer は3引数維持、検索側は `viewerLink(id)` へ。
- L2567・L2570（fullscreen `.side` 規則）→ viewer 側は chrome selector（nav 行・`#reader-controls`）へ re-anchor。
- L1592（`.side`/`.main`）→ 検索側 CSS 維持で pass。viewer 側から `.side` が消えるため、両ファイル横断の assert は検索側の規則で成立（テストは文字列照合のため、維持側を明示）。
- `tests/read_lists_debounce_harness.mjs` L72 の `note` mock は viewer read helper 用（検索 lists block は `queries`/`favorites`/`unread` のみ参照）。検索側 `#note` 削除で harness は無変更。badge 表示を `showLists` block 内で行う場合は mock に `results` 追加が必要（block 外 helper なら不要）。
- `tests/strip_rebuild_dom_harness.mjs`: viewer 側 anchor（`renderStrip` 終端）は無影響。
- `tests/test_canonical_build_freshness.py`: 変更後 2 html byte 照合を再実行。

### 記録補足（実測 2026-11-04・コミット `01baa01`）

- L2094・L2470 の記述（両ファイルで `function route(id, page, second) {` を assert）は事実と異なる。実テストは `uiText(t, "viewer.html")` の viewer のみで、検索側は新 test で `viewerLink(id, page)` を assert した。設計の意図（viewer は3引数維持・検索側は単独 link helper）は変更なし。
- fullscreen chrome の nav/control 非表示は selector と固定 test で確認済み。browser-linked fullscreen criteria の実測（normal screen capture の before/after）は未完了（残件）。

## 実装開始

実装開始指示は未受領（2026-11-04 10:10 時点で設計方針の選択のみ）。ソース・テストは無変更。着手には明示的な実装開始指示が必要。
