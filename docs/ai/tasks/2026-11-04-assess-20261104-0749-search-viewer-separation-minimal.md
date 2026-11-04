# 案A（2ページ分割）の最小版：検索と閲覧の完全分離（assess-20261104-0749 項目1）

**Status**: 完了
**Date**: 2026-11-04
**Next Action**: A1 実装の A1 変更のみコミット/push。AGENTS.md の Canonical rule 条文章変更は write-protected 領域のまま個別承認待ち。Google Drive 画面確認スクリーンショットは I:\マイドライブ\kilo\assess-20261104-0749-search-viewer-separation-minimal\ に保存済み。

## 意図 / 背景

実行固有ID: `assess-20261104-0749`（発番 2026-11-04 07:49）

対象項目一覧（1件）:

| 項目ID | 項目 | 個別タスク記録 |
|---|---|---|
| A1 | 案A（2ページ分割）の最小版 | `docs/ai/tasks/2026-11-04-assess-20261104-0749-search-viewer-separation-minimal.md` |

項目A1の原文（ユーザー確認済みの結論 2026-11-04）:

- 結論: 案A（2ページ分割）の最小版。`app/viewer.html` に閲覧系（reader/strip/nav/acquisition）、`app/search.html` に検索系（form・card list・TN cover・viewer へのリンク）を置き、`embed` に両方を同梱して単体 exe を維持する。
- 既定 landing は viewer（`GET /`）、search は `GET /search`。ハッシュ namespace はページごとに単独（viewer `#/g/...`、search `#/q/...`）。
- objectURL cache はドキュメントごとに閉じる（共有しない）。read_state / favorites / queries の正典は DB（API）で単一。ページ間同期機構（BroadcastChannel）は v1 で入れない。
- 分離により消える結合点（調査対象）: `updateThumbCache()` の `#results` cover 書き込み、`searchCardCoverSource()` の full objectURL 返却、`revokeFullObjectUrls()` による検索 cover revoke、`loadSearchCardCover()` の `currentPriorityDone` 待ち、`hashchange`・起動分岐の 2 namespace 混在。
- 正典チェックポイント: tag `search-viewer-separation-checkpoint-2026-11-04` → `b1ef252`（分離前の状態）。

開始時点の既存未コミット変更: なし（`git status --porcelain` 空・HEAD `b1ef252`）。

## 調査結果 / 変更内容

（子調査 2026-11-04。HEAD `b1ef252`。ソースは未変更、本記録のみの更新）

### 現在の実装

- 単一ドキュメント `app/ui.html`（1834 行）。`<body>` 内で `.side`（検索 form `#search` L108-115・`#results` L120・保存検索 `#queries` L129・`#favorites`/`#unread` L133-134）と `.main`（閲覧: `#gallery` L140・reader controls・acquisition L156-161・`#reader` L166・`#nav` L167・strip `#thumbs` L182）が同一 DOM に共存。
- `app/viewer.go`: `//go:embed ui.html`（L22-23）、`index` ハンドラ（L210-222）は `GET /` のみ（route 登録 L114）。それ以外は 404。実機で `GET /search` => 404 を確認（分割前なので当然）。

### 結合点（原文列挙の5点すべて現行コードに存在・行番号確認）

1. `updateThumbCache()` ui.html:317-323 — objectURL を `#thumbs img[data-key]`（viewer）と `#results img[data-key]`（search card cover）の両 DOM へ書き込み。
2. `searchCardCoverSource()` ui.html:1347-1354 — `fullCache` の full objectURL を優先し TN へ fallback。TN policy 正典（TN は search-card cover 専用）上、分離後は pure TN（server の `thumbnail`（bigtn）のみ）になる。
3. `revokeFullObjectUrls()` ui.html:904-918 — viewer 側 objectURL revoke で `currentSearchObjectUrlKeys`（L911）を clear。search 側 `markSearchObjectUrlKey` は `renderSearchCard()` L1396・`restoreSearchState()` L1452 から viewer 側 LRU を汚す。
4. `loadSearchCardCover()` ui.html:1405-1413 — `await currentPriorityDone`（L250 定義、runPriority L1029、L1078/1089 代入、L1136 待ち）。search cover が viewer の priority queue に依存。
5. `hashchange` ui.html:1799-1809 — 単一ハンドラで `parseReaderHash`（L698）と `parseSearchHash`（L1415）の 2 namespace を解析。起動分岐 L1816-1831: 無 hash 時 `runSearch()`（L1829）= 現行ランディングは search 側。A1 の landing viewer 化は挙動変更（実機確認済み: fresh tab 無 hash => search が実行される）。

### 追加の結合点（原文外・調査で発見）

6. `openGallery()` ui.html:1269-1312 が `showLists()`（L1313-1330、search 側 DOM `#queries`/`#favorites`/`#unread` 書き込み）を呼ぶ。`searchCardCache`（L1278）を高速起動に使用。`runBackgroundReadAndLists` 経由でも showLists を呼ぶ。分離により viewer 側は card cache 短縮起動を失う。
7. リンク生成が単一ドキュメント前提: `route()` L263-266・`newTabHref()` L260-262・showLists リンク L1324-1329・renderSearchCard L1402・sameTitleJapaneseAnchor L1388。分離後は cross-page 絶対リンク（`/#/g/...`）が必要。
8. localStorage key（同一 origin で共有): `SEARCH_STORAGE_KEY` L1423-1434、`TITLE_GROUPS_STORAGE_KEY` L1360-1372、`SPREAD_DIRECTION_STORAGE_KEY` L743-751。read_state/favorites/queries の正典は既に DB API（`POST /api/read` L1184/L1615、`GET /api/lists`、`POST /api/fav`）で単一 — 衝突なし。

### 有効性

有効（5結合点すべて原文どおり存在。上記3点の追加結合点を加算）。

### 判断保留（判断付かない点）

- 保存検索/お気に入り/未読 lists（`showLists`）の所有ページ: search.html か viewer.html か（原文は form・card list・TN cover・viewer へのリンクのみ明示）。テスト harness の anchor もこの判断に依存。
- 共有 core（escapeHTML・api/post・generation guard・TN URL helper）をドキュメントごとに複製するか共有ファイルとして embed するか。
- 現行ランディング（search）→ A1（viewer）の UX 挙動変更の明示承認是否需要（結論自体はユーザー確認済み）。

### 実現可能性と適切な修正方法

- `viewer.go`: 単一 embed を multi-file 化（`//go:embed viewer.html search.html`）、`index` は viewer.html を `GET /`、`GET /search` ルート追加（mux L113-128 の既存 API は無変更）。
- search 側: TN-only cover（server は既に `thumbnail`=bigtn を返す。`TestSearchResultUsesCoverOnlyThumbnail` L1432-1445 で確認済み）。full objectURL 優先と `currentPriorityDone` 待ちを削除。
- objectURL cache / fullCache / LRU（`currentSearchObjectUrlKeys` 含む）は viewer.html 側のみ残る。generation guard はドキュメント単位で現状維持。

### テスト影響

- `tests/test_canonical_build_freshness.py`: `UI_PATH` 単一（L9）、hub.exe の byte 一致 assert（L44/48）→ 両 html の byte 一致検証へ改修必要。
- `app/viewer_test.go`: `uiPage.ReadFile("ui.html")` assertion **37件**。関数 body を単一文字列 anchor で slice する test（例 L1519-1524、L1676、L2102-2126、L2403、L3069-3075、L3166）は viewer/search 両ファイルへの re-anchor が必要（15〜20 test 影響）。消える結合点を「存在」として assert している test（L1566 hashchange、L2289-2317 LRU search keys、L2338/2379 searchCardCoverSource+currentPriorityDone、L2349、L1677 openGallery→showLists）は不在 assert へ書き換えか TN-only 挙動への再_target（7〜9 test）。
- `tests/strip_rebuild_dom_harness.mjs`: viewer 側抽出（highlightStrip/renderStrip/refreshStrip L146）＋ `openGallery` body の end anchor が `showLists`（L151）→ 所有判断により cross-file anchor 改修。path を viewer.html へ。
- `tests/read_lists_debounce_harness.mjs`: `showLists` block 抽出（L25）→ 所有判断に依存。path 変更。

### 副作用・影響範囲

- search card の「full objectURL 表示 upgrade」が失われる（pure TN 化）。TN policy 正典上は整合的だが現行挙動の変更（LRU 保護 test がこれを「存在」として assert している）。
- cross-page は SPA hashchange ではなく full navigation。fast boot 短縮（searchCardCache）と same-tab 遷移の連続性が失われる。BroadcastChannel なし（v1 設計）→ ページ間 lists/read_state の staleness 許容。
- landing 変更（search→viewer）はユーザー可視の挙動変更。

### 画面確認（実施・合格）

正典 exe（freshness test 合格＝現行 ui.html byte 一致）から canonical server `http://127.0.0.1:8731` を起動。UI 確認前にサーバー直 baseline: `GET /` => 200（76503 bytes・ui.html）、`GET /search` => 404。fresh 新規タブ（hash なし）で `GET /` → 起動分岐 runSearch 実行、hash `#/q/furry/page/1`、`#results` card 25 = server ids 25（total 32901）、cover = TN bigtn（`/api/thumb/...&variant=bigtn`）、reader 側は gallery 未選択の空表示。resize 前・fresh context・重複 id なし。合格。
保存先（Google Drive・タスク別フォルダ）: `I:\マイドライブ\kilo\2026-11-04-assess-20261104-0749-search-viewer-separation-minimal\`（`search-viewer-separation-landing-single-doc-20261104.png` + `search-viewer-separation-landing-single-doc-confirmation-20261104.txt`）。

## 優先順位（親・assess-20261104-0749）

根拠は子の要約と本記録の記載のみ（親は再調査しない）。

- 項目A1: 優先順位 1（対象項目1件のため順位は単一）
  - 重要度 中〜高: TN policy 正典との整合、LRU・priority subsystem の簡素化
  - 緊急性 低〜中: blocker なし、方式の結論はユーザー確認済み（2026-11-04）、分離前 tag `search-viewer-separation-checkpoint-2026-11-04`（`b1ef252`）確保済み
  - リスク 中: テスト表面（freshness 正典ルール変更 + `viewer_test.go` 37 assertion の 15〜20 件 re-anchor / 7〜9 件 rewrite + mjs harness 2件）、landing UX 挙動変更、search card の full objectURL upgrade 喪失、cross-page 遷移の SPA 連続性喪失
  - 推定規模 中: viewer ~1100-1200 行 / search ~600-700 行 + 共有 core 50-100 行、Go 側 embed multi-file 化と `GET /search`（小）、主成本はテスト改修
- 対応不要項目: なし。A1 は対応対象として実行固有ID `assess-20261104-0749` とともに次段階へ引き継ぎ。
- 判断保留（次段階で確定が必要・親は判断していない）:
  1. `showLists`（保存検索・お気に入り/未読）の所有ページ（search か viewer か）
  2. 共有 core（escapeHTML・api/post・generation guard・TN URL helper）をドキュメントごとに複製するか embed 共有ファイルにするか
  3. 既定 landing を search → viewer へ変更する挙動変更の承認是否需要
  4. `tests/test_canonical_build_freshness.py` の正典ルール（exact bytes 照合）を 2 ファイルへ変更する承認

### 判断保留の解決（ユーザー承認 2026-11-04 08:22・推奨どおり採用）

正典: `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`

1. 一覧の所有ページ → 検索ページのみ。閲覧ページは `showLists` を呼ばない。ページ間同期機構（BroadcastChannel）は第1版なし。
2. 小道具（`escapeHTML`・読み込み helper・世代番号 guard・TN URL 組み立て） → ページごとに複製（自己完結）。共有ファイル embed 案は不採用。
3. 起動ページ → `GET /` = viewer、`GET /search` = search。hash namespace はページごとに単独、回送分岐なし。search URL は `/search#/q/...`。既定 landing を search とする案は不採用。
4. freshness test → `app/viewer.html`・`app/search.html` の2本照合、失敗メッセージに両ファイル名、画面確認は viewer 基準＋search の served 画面確認を追加。`AGENTS.md` の条文章変更は write-protected 領域のため個別の明示承認が必要（方向承認済み・ファイル編集未実施）。

`docs/ai/decisions/2026-11-01-viewer-single-page-web.md` の「1 ページ Web」の点は上記により部分 supersede（冒頭に Superseded by を追加。native GUI 不採用・`embed` 同梱・bind `127.0.0.1`・local-first 画像経路・SQLite 単一接続は維持）。

### 実装（2026-11-04）

- `app/ui.html` を削除（ゴミ箱送済み）。`app/viewer.html`・`app/search.html` を追加。
- `app/viewer.go`: `//go:embed viewer.html search.html`、`GET /` は `viewer.html`、`GET /search` は `search.html`。
- `app/viewer.html`: viewer 自己完結（reader・strip・nav・acquisition・view mode・objectURL LRU・generation guard）。search-owned `showLists`・`searchCardCache`・search objectURL keys は持たない。`prefetchPageImage` は `galleryId !== current.id || backgroundGalleryId !== galleryId` を維持。
- `app/search.html`: search 自己完結（form・card list・saved searches・favorites/unread・TN-only cover・viewer への絶対リンク `/#/g/...`）。`searchCardCoverSource(card)` は `card.thumbnail` または TN `bigtn` URL のみ。objectURL cache・`currentPriorityDone` 待ちなし。
- `app/viewer_test.go`: `uiText(t, "viewer.html")` / `uiText(t, "search.html")` へ振り分け、search TN-only・viewer-only objectURL LRU・viewer-only read/favorite sync・cross-page route を再_target。
- `tests/test_canonical_build_freshness.py`: `app/viewer.html` と `app/search.html` の byte 一致を正典 `hub.exe` へ照合、失敗メッセージに両ファイル名。
- `tests/strip_rebuild_dom_harness.mjs`: `app/viewer.html`、`openGallery` end anchor を `function renderStrip() {` へ変更。
- `tests/read_lists_debounce_harness.mjs`: viewer read helper と search lists helper を分離して実行。

## Verification

- `go vet .`（app）=> 問題なし
- `go test -count=1 -run '^TestEmbeddedUI' .`（app）=> `ok hitomi-hub/app 0.165s`（UI test 全件）
- `go test -count=1 -timeout 300s .`（app）=> `ok hitomi-hub/app 17.337s`
- `python -m pytest tests/ -v` => 51 passed（8.92 s）
- `node tests/strip_rebuild_dom_harness.mjs` => 18 checks passed
- `node tests/read_lists_debounce_harness.mjs` => 16 checks passed
- CLI download（canonical exe・real gallery `4252697`・`--limit 1`）: `attempted=1`, `written=1`, `skipped=0`。保存 file `scratch/p3_canonical_fresh/Galleries/4252697/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.webp`、`357106` bytes。
- Canonical UI/network confirmation（canonical server `http://127.0.0.1:8731`）:
  - `/api/image/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=webp` => HTTP 200、`x-cache: miss`、`content-type: image/webp`。
  - viewer page `/#/g/4252697/page/1`: reader display img 1件、`naturalWidth=2880`、`naturalHeight=1942`。
  - search page `/search`: 25 cards、25 TN covers、`/api/thumb/...?format=webp&gallery=...&variant=bigtn` HTTP 200、blob objectURL なし、`/api/image` resource なし。
  - Google Drive task-specific folder: `I:\マイドライブ\kilo\assess-20261104-0749-search-viewer-separation-minimal\`（`viewer_confirmation.png`, `search_confirmation.png`）。
- `AGENTS.md` の Canonical rule 条文章は未変更。既存テストは `tests/test_canonical_build_freshness.py` 条、canonical build command、UI confirmation 文字列を保持し、照合対象のみ 2 html へ変更済み。


## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
