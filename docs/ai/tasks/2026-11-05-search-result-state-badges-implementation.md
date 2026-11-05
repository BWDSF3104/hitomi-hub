# 検索結果画面の3状態 badge と手動リセットの実装

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 画面確認サイズをwindow size基準へルール化済み。commit/push後、人工resizeなしでfresh window size基準の actual confirmationを行い、本項目の合格条件を最終判断

## 意図 / 背景

- 検索結果画面の card に 表示済み / 既読 / 保存済み を表示し、表示済みと既読を手動リセット可能にする。
- 案検討・計画の正典: `docs/ai/tasks/2026-11-05-search-result-state-badges.md`
- 設計判断の正典（ユーザー承認 2026-11-05 03:29）: `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md`
- 実行固有ID: `search-result-state-badges`（親の統括のみ。実装・検証実作業は子へフォアグラウンド委譲）

## 承認範囲（この項目で変更してよいもの）

- `schema.sql` の `read_state` へ `viewed INTEGER NOT NULL DEFAULT 0`・`viewed_at TEXT` 追加、`app/main.go` の `schemaVersion` 5→6 と非破壊迁移（列追加のみ・既存行削除しない）
- `app/library.go`: `readStateRow` の viewed/viewed_at、`setViewed`（viewed_at は初回のみ）、`resetReadState(db, id, target)`、`setReadState` の viewed 保持、`setFavorite` は viewed を立てない
- `app/viewer.go`: `searchResult` へ `viewed` / `saved` 追加（既存 `entryLocalFormats` ループで計算。追加 I/O なし）、`POST /api/viewed`・`POST /api/reset` ルート追加
- `app/viewer.html`: `openGallery()` の route 入力時点（`app/viewer.html:1113`）で viewed 記録（display path を await しない・generation guard 維持）、最終 page 到達で debounced read helper 内から `complete: true`
- `app/search.html`: `badgeText` を 3フラグ + お気に入りへ拡張、badge 行 inline に reset button（`data-reset="viewed|read"`）、card 応答の state で maps を補完
- `app/main.go` CLI: `read` へ `--reset-viewed` / `--reset-read`
- 該当テストの re-anchor と新規テスト（`app/*_test.go`、`tests/`）
- Memory Bank（本タスクファイル、該当する decision/known-issues）

変更しない: 検索/閲覧のページ分離、TN display policy、full image 経路（`AGENTS.md`「絶対に壊してはいけない条件」）、`listRows("unread")` の「未読」意味、card height contract（130px）、viewer nav への reset 追加、`clicked` フラグ新設。

## 合格条件と検証方法

1. `go vet . && go test -count=1 -timeout 300s .`（app）合格
2. `python -m pytest tests/ -v` 合格（`tests/test_canonical_build_freshness.py` 含む）
3. schema 迁移: user_version 6、既存 read_state 行が保持される（非破壊）。Python 参照実装は schema.sql 正典を共有するため `tests/test_scan.py` がそのまま合格
4. 状態意味のテスト:
   - `setViewed` は初回のみ `viewed_at` を書き、reopen で上書きしない
   - `setFavorite` が作った `last_page=0` 行は viewed にならない
   - `reset-viewed` → viewed/fully_read/last_page クリア、`reset-read` → fully_read のみ
   - card 応答に `viewed` / `saved` があり、部分保存は `saved=0`（meta の `local N/M`）
   - 最終 page 到達で `fully_read=1`（`no_full` でも到達のみで既読）
5. 正典 build（`output/hitomi-hub/hub.exe`）で served 画面確認。`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 準拠:
   - 目標 viewport 1280x900 を測ってから resize、fresh tab、direct server baseline と UI response 一致
   - 実データ3状態（①viewed・未既読 ②既読 ③保存済み）で badge 表示
   - reset button の click で API→DB 行変化を direct SQL で照合
   - card height contract（130px）維持、`.side` 480px、既存回帰（TN cover・generation guard・no-store fetch・full image path・reader nav・favorite/unread）
   - 画面キャプチャは Google Drive の task-specific screenshot フォルダへ保存し、保存先・ファイル名・確認結果を本記録へ記入

## 調査結果 / 変更内容

### 親の機械的確認（委譲サイクル 1）

- 委譲前: HEAD `25cf419`・clean。委譲後の変更は本項目の承認範囲9ファイルのみ（`app/library.go`・`app/main.go`・`app/main_test.go`・`app/search.html`・`app/viewer.go`・`app/viewer.html`・`app/viewer_test.go`・`schema.sql`・`tests/read_lists_debounce_harness.mjs`）。本項目分のコミットはまだ無く、task Status は子の報告（進行中・画面確認 blocker）と一致。

### 画面確認方法の判断（親が段階1/2で処理・子へ差し戻し 2026-11-05 03:33）

- 正典 `bin/kilo-fresh-browser.ps1` の変更・コピーは承認範囲外（段階3）なので不許可。OS click が `#search-submit`（x=55）へ届かない問題は、正典 CDP 経路で実測して閉じる: fresh browser を CDP target（正典 `kilo-cdp-measure.ps1` が使う port `9344`）として起動し、target tab を測れることを最初に実測で確認する。
- `docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 項目2は「`#search-submit` click **または** `form#search` submit must fire」を許容する。OS click が届かない場合は `form#search` の submit 発火を実測で確認する（query input への Enter、または CDP element click）。CDP click は trusted input event として actual UI event を満たす。どちらの機構を使ったかは記録に明記する（fresh tab・resize 前測定・direct server baseline と DOM/network 一致の基準は同じ）。
- actual submit/click event を実測でも確認できない場合は画面確認を未検証として合格にしない。正典 tool 制限は `known-issues/`（KI-20261105-）へ記録し、画面確認の残件だけ段階3として持ち帰る。Go test・pytest・正典 build・DB direct SQL 照合で取れる証拠は先に集める。

### 機械確認差し戻し（2026-11-05 05:32）

- `initial display`（resize 前 target viewport）確認は未検証。
- existing tool target fresh tab の resize 前測定値は `innerWidth=1920`, `innerHeight=953` だった。その後 `browser_resize` で `1280x900` へ変更した。
- strict criteria項目5（1280x900 を測ってから resize、resize dependency 禁止）を満たさないため、initial display確認を合格扱いしない。
- resize後の測定で `.side` width `480px`、card computed height `130px`（bounding rect は padding 込みで `140.59375px`）を観測したが、これは resize後の測定であり、initial display確認の証拠として採用しない。

### 実行・実測

- `go vet .`：合格（output なし）
- `go test -count=1 -timeout 300s .`（app）：合格 `ok hitomi-hub/app 18.028s`
- `python -m pytest tests/ -v`：合格 `53 passed in 9.30s`。`tests/test_canonical_build_freshness.py` 含む。
- canonical build state：正典 server pid `32632` を stop し、`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` を実行、canonical server pid `29792` を restart した。freshness test は pytest 内 green。

### Direct SQL / schema

- 正典 runtime DB：`S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh\hitomi.db`
- `PRAGMA user_version`：`6`
- `read_state` columns：`gallery_id`, `last_page`, `fully_read`, `opened_at`, `favorite`, `viewed`, `viewed_at`
- `4305294` before final pair：`last_page=8`, `fully_read=0`, `viewed=0`, `viewed_at=2026-11-05T03:57:10Z`（既存行保持）
- actual UI final pair request `#15` body：`{"gallery_id":4305294,"page":8,"complete":true}`
- `4305294` after final pair before reset：`last_page=8`, `fully_read=1`, `viewed=1`, `viewed_at=2026-11-05T03:57:10Z`
- actual reset-read after click：`4305294` `last_page=8`, `fully_read=0`, `viewed=1`, `viewed_at=2026-11-05T03:57:10Z`
- actual reset-viewed after click：`4305294` `last_page=0`, `fully_read=0`, `viewed=0`, `viewed_at=2026-11-05T03:57:10Z`
- `4305205` existing row preserved：`last_page=2`, `fully_read=0`, `viewed=1`, `viewed_at=2026-11-05T04:20:07Z`

### direct server baseline と UI response

- direct baseline command：`curl -s -i "http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25"`
- direct baseline result：HTTP `200`、`X-Search-Timing: search=0;record=58;json=1`、`Date: Thu, 05 Nov 2026 05:25:11 GMT`、`content-length=791`、`cache-control: no-store`、`total=32916`、`ids.length=25`
- UI response actual click：`#search-submit` CDP trusted click。request `#195`。
- UI response result：HTTP `200`、`X-Search-Timing: search=0;record=48;json=1`、`Date: Thu, 05 Nov 2026 05:25:15 GMT`、`cache-control: no-store`、`content-length=791`、`total=32916`、`ids.length=25`。direct baseline と一致。
- DOM card count after UI response：`25`。status：`検索完了・カード表示済み 25件`。

### badge 3状態 / `#lists-refresh`

- actual UI `#lists-refresh` click：request `#273` `GET /api/lists => 200`。
- card `4305294` badge：`表示済み・既読・保存済み`、meta `local 8/8`、TN cover source `/api/thumb/...&variant=bigtn`、reset buttons `viewed` / `read`。
- card `4305205` badge：`表示済み`、reset button `viewed`。
- actual reset buttons：`li[data-search-id='4305294'] button[data-reset='read']`、`li[data-search-id='4305294'] button[data-reset='viewed']` via existing tool CDP trusted click。Direct SQL 照合済み。
- reader route actual events：fresh target viewer page、keyboard events via `browser_press_key`。final effective sequenceは `r`で `ltr`、`d`で double、`ArrowRight` ×3で `#/g/4305294/page/7-8`。`openGallery` route input時 `POST /api/viewed` request `#2`。viewer network filter `lists` は none。

### confirmation mechanism / known issues

- existing tool `kilo-playwright_browser_evaluate` で target viewport・`.side`・card height・DOM を read-only measured。actual UI events は existing tool CDP trusted click/keyboard。
- 正典 `kilo-cdp-measure.ps1` は port `9344` で current targetに connect できなかった。正典 `kilo-fresh-browser.ps1` は fixed center-click で `#search-submit`（x=55）を確定できない。
- known issues：
  - `docs/ai/known-issues/KI-20261105-03-search-result-state-badges-confirmation-tool-limit.md`
  - `docs/ai/known-issues/KI-20261105-04-search-prefetch-worker-undefined-entry-race.md`

- 本プロジェクトルール：`docs/ai/rules/normal-screenshot-rule.md`
  - 通常スクリーンショットの被写体は Playwright browser画面の画像。
  - `canonical_initial_display_fresh_target_clean.png` の full virtual screen capture は、例外理由がない通常 visual confirmation evidence として無効。
  - 2026-11-05 06:27 に `taskkill /IM msedge.exe /F /T` を実行し、起動中 Edge を停止した。

### Minimum Playwright screen confirmation re-run（2026-11-05 06:51）

- Target: Playwright current target `http://127.0.0.1:8731/search#/q/furry/page/1`.
- before-resize target viewport exact `1280x900`: `.side` `480px`, `form#search` `67.7734375px`, `#results` cards `25`, `.card` computed height `130px`.
- Direct server baseline via Playwright fetch: HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=25;json=1`, `total=32916`, `ids.length=25`, `results.length=0`.
- actual UI event: `#search-submit` click. UI `/api/search` request response: HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=554;json=1`, `total=32916`, `ids.length=25`, `results.length=0`. Direct baseline and UI response match on total/ids/no-store/timing.
- DOM/status before screenshot: `#search-page-status` `page 1 / 1317・total 32916`, `#count` `32916 件・page 1`, `#search-status` `検索完了・カード表示済み 25件`.
- Minimum visual screenshot: Playwright viewport screenshot `docs/ai/tmp/minimum-playwright-viewport-2026-11-05.png`. It shows first card `保存済み`, `local 8/8`, TN cover, target status fields, and no OS full-screen capture was used.
- OS full-screen capture was not used. Normal screen confirmation is Playwright browser viewport screenshot only.

### visual confirmation

- capture：`kilo-screen-capture.ps1 -WindowTitle "hitomi hub search" -OutFile "docs\ai\tmp\badge-card-and-lists-refresh.png"`
- Drive：`I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\2026-11-05\badge-card-and-lists-refresh.png`
- visual read：same target windowで card `Mimi expansion` badge `表示済み・既読・保存済み`, second card `表示済み`, TN cover, `local 8/8`, `#lists-refresh` visible同時。
- ただし initial display（resize前 target viewport）未検証のため、この visual readは actual badge visual evidenceとして記録するが、strict criteria項目5の initial display確認合格として扱わない。

### Canonical fresh target re-measurement（2026-11-05 05:34-05:51）

- Fresh CDP target launched by canonical `kilo-fresh-browser.ps1` with `--window-size=1280,900`, profile `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh\edge-fresh-browser-9344`, port `9344`, URL `http://127.0.0.1:8731/search`.
- Canonical `kilo-cdp-measure.ps1` direct `pwsh` call failed with `SendAsync` ambiguous overload. Same canonical script via `powershell -NoProfile -File` succeeded and measured the fresh target before any resize.
- Measured target before resize:
  - `outerWidth=1280`, `outerHeight=900`.
  - `innerWidth=1259`, `innerHeight=810`.
  - `.side` actual `getBoundingClientRect().width=480px`.
  - `form#search` height `67.7734375px` (`67px`-`70px` range).
  - `#results` card count `25`.
  - `.card` computed height `130px`.
  - `#search-page-status` `page 1 / 1317・total 32916`.
  - `#count` `32916 件・page 1`.
  - `#search-status` `検索完了・カード表示済み 25件`.
  - `#perf-status` `perf: /api/search 301ms・server search=0;record=290;json=1・cards 25 17012ms・cache cache=25`.
- Direct server baseline after initial measurement:
  - `curl -s -i "http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25"`.
  - HTTP `200`, `X-Search-Timing: search=0;record=1369;json=1`, `Date: Thu, 05 Nov 2026 05:47:52 GMT`, `Cache-Control: no-store`, `Content-Length: 793`, `total=32916`, `ids.length=25`.
- Actual `#search-submit` UI event on fresh target: existing tool range CDP trusted input mouse click at `#search-submit` center `x=78`, `y=160`. A new `/api/search?query=furry&lang=all&page=1&limit=25` performance resource entry appeared: HTTP `200`, `decodedBodySize=792`, `initiatorType=fetch`; `#perf-status` showed server timing `search=0;record=311;json=1`. Card count remained `25`, `#count`/`#search-page-status` matched direct baseline.
- `#lists-refresh` actual UI event: existing tool range CDP trusted input mouse click at `#lists-refresh` center `x=52`, `y=470`. New `GET /api/lists` performance resource entry appeared: HTTP `200`, `decodedBodySize=1313`. `#queries`/`#favorites`/`#unread` lists updated.
- Badge and reset actual events on fresh target:
  - Before reset, `li[data-search-id="4305205"] .badge` text `表示済み reset`, `button[data-reset="viewed"]` present.
  - `4305205` direct SQL before actual `button[data-reset="viewed"]` CDP trusted click: `last_page=2`, `fully_read=0`, `viewed=1`, `viewed_at=2026-11-05T04:20:07Z`.
  - Direct SQL after actual click: `last_page=0`, `fully_read=0`, `viewed=0`, `viewed_at=2026-11-05T04:20:07Z` preserved.
  - `4305294` direct SQL during same fresh flow: `last_page=0`, `fully_read=0`, `favorite=0`, `viewed=0`, `viewed_at=2026-11-05T03:57:10Z`. Its badge text was `保存済み`; `data-reset="read"` button was not present because `fully_read=0` at this stage. The prior `4305294` final-pair/reset-read direct SQL evidence remains in this task record.
- Existing-feature regression checks on fresh target:
  - `#search-next` CDP trusted click changed `#page` to `2`, hash to `#/q/furry/page/2`, `#search-page-status` to `page 2 / 1317・total 32916`, and card count stayed `25`.
  - `#search-prev` CDP trusted click returned `#page` to `1`, hash to `#/q/furry/page/1`, `#search-page-status` to `page 1 / 1317・total 32916`.
  - `form#saved-search` `save` button CDP trusted click fired `form#saved-search` submit; `#queries` added `canonical-initial-click female:furry click`. Performance entries did not record the POST resource, but the list response reflected the save.
  - `form#query` is not present; `input#query` remains distinct from `form#saved-search`.
- Visual confirmation on same fresh target:
  - Canonical capture: `kilo-screen-capture.ps1 -WindowTitle "hitomi hub search" -DriveFolder "I:\マイドライブ\kilo\2026-11-05-search-result-state-badges" -FileName "canonical_initial_display_fresh_target_clean.png" -Root "S:\Documents\Kilo_Commons\hitomi-hub"`.
  - Local file: `docs/ai/tmp/canonical_initial_display_fresh_target_clean.png`.
  - Drive file: `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\canonical_initial_display_fresh_target_clean.png`.
  - Visual read: same fresh target window shows `127.0.0.1:8731/search#/q/furry/page/1`, `#results` TN card covers, `card` backgrounds, `Mimi expansion` badge `保存済み`, `#lists-refresh` visible, `#count` and `#perf-status` agree with actual total/card processing, and `#search-status` shows the actual `reset: #4305205 viewed` event. TN is used only as search-card cover source in `#results`; no TN display fallback was observed.
- Strict criteria judgment:
  - Direct baseline, UI response, DOM card count, `#count`, `#search-page-status`, `#search-status`, card height, `.side`, `#lists-refresh`, reset click direct SQL, and visual read are confirmed on the same fresh target.
  - `#perf-status` `cards 50` after the actual `#search-submit` event is the app's cumulative card-processing counter across the initial search and the actual submit event. The actual card DOM count is `25`, and `#count`/`#search-status` match the direct baseline.
  - Initial display target viewport remains not confirmed as exact `1280x900` because actual content viewport was `1259x810`; `outerWidth/outerHeight` was `1280x900`. If project policy accepts canonical `--window-size` as target, all measured strict criteria pass. Otherwise exact inner target requires a tool change or policy decision and remains unverified.

### Playwright exact-target continuation（2026-11-05 07:12）

- Target after explicit `browser_resize`: inner viewport exact `1280x900`, URL `http://127.0.0.1:8731/search?newtab=1#/q/furry/page/1`.
- Before actual UI event, same target direct server baseline via Playwright fetch: HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=51;json=1`, `total=32916`, `ids.length=25`, `results.length=0`.
- Actual UI event `#search-submit` click: network response `HTTP 200`, `cache-control: no-store`, `x-search-timing: search=0;record=52;json=1`, `total=32916`, `ids.length=25`. Direct baseline and UI response match.
- Same target layout after actual UI event: `.side` `getBoundingClientRect().width=480px`, `form#search` computed height `67.7734px`, `#results` card count `25`, `.card` computed height `130px`. Card bounding rect `737x140.59375px` is padding-inclusive.
- Actual UI route input for `4305294`: clicked target card title link, opened fresh viewer tab, `POST /api/viewed => 200`, direct SQL `viewed=1`.
- Actual final-page UI event in viewer tab: `ArrowRight` to `page/7-8`; `POST /api/read` response `{"gallery_id":4305294,"last_page":8,"fully_read":1}`. Direct SQL after event: `last_page=8`, `fully_read=1`, `viewed=1`, `viewed_at=2026-11-05T03:57:10Z`.
- Actual reset-click confirmation on target card `4305294`:
  - `button[data-reset="read"]` click: UI `表示済み・保存済み`, `button[data-reset="viewed"]` remains; direct SQL `last_page=8`, `fully_read=0`, `viewed=1`.
  - `button[data-reset="viewed"]` click: UI `保存済み`, no reset button remains; direct SQL `last_page=0`, `fully_read=0`, `viewed=0`; `viewed_at` preserved.
- Post-reset actual UI restore: `#search-submit` click, route input, final page, direct SQL restored to `4305294 last_page=8, fully_read=1, viewed=1`, UI badge `表示済み・既読・保存済み` with `reset read` and `reset viewed` buttons.
- Actual `#lists-refresh` click on same exact target: `GET /api/lists => 200`; `#lists-refresh` visible; UI badge remained `表示済み・既読・保存済み`, buttons `viewed`/`read`; second target `4305205` showed `表示済み`, `button[data-reset="viewed"]`, direct SQL `last_page=1`, `fully_read=0`, `favorite=0`, `viewed=1`, `viewed_at=2026-11-05T04:20:07Z`.
- Playwright screenshots copied to Google Drive:
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\search-state-badges-before-reset-2026-11-05.png`
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\search-state-reset-read-2026-11-05.png`
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\search-state-reset-viewed-2026-11-05.png`
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\exact-target-1280x900-search-state-badges-2026-11-05.png`
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\exact-target-1280x900-search-state-badges-after-lists-refresh-2026-11-05.png`
- Strict criteria note: this is valid actual UI badge/reset confirmation on the exact current Playwright target after explicit resize. It is not the initial display pass, because strict criteria require target measurement before any `window.resize` on a fresh isolated tab. A fresh `browser_tabs new` tab measured `1920x953`, so exact `1280x900` initial display remains pending policy/tool confirmation.

### 追加要件の実装（2026-11-05 07:20）

- 追加要件の正典: `docs/ai/decisions/2026-11-05-search-result-card-status-frame-and-summary-reset.md`。この decision は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` の `data-reset="viewed|read"` 個別 button 配置のみ supersede する。既存の `reset-read` / `reset-viewed` cascade 意味は維持し、その上位として `target=all` を追加した。
- `app/search.html` 変更:
  - `.card.has-status { outline: 2px solid #8ab4f8; border-radius: 6px; }` を追加。layout height contract を壊さないため `outline` を使用。
  - `hasStatus(id)` は viewed / fully_read / saved / favorite のどれかが true なら status frame の対象とする。
  - `hasResetStatus(id)` は persisted user status（viewed / fully_read / favorite）があるときだけ summary reset button の対象とする。`saved` はファイル由来なので file delete なしの summary reset ではクリアしない。
  - `cardStatusClass(id)` を render の card class へ反映し、`renderReadBadges()` で `li.classList.toggle("has-status", hasStatus(id))` も更新する。
  - badge row は `data-reset='viewed'` / `data-reset='read'` の個別 button ではなく、`<button type='button' data-reset='all'>reset</button>` のまとめ button 1つへ変更。
  - reset handler は `/api/reset` response の `favorite` から `favoriteByGallery` を update/delete する。
- `app/library.go` 変更:
  - `resetReadState` に `case "all"` を追加。`last_page = 0`・`fully_read = 0`・`viewed = 0`・`favorite = 0` をまとめてクリアする。`saved` はファイル由来なので DB 削除しない。
- `app/viewer.go` 変更:
  - `POST /api/reset` の `target` は `viewed` / `read` / `all` を許可。
- Test re-anchor:
  - `app/main_test.go`: `reset-all` が read / viewed / favorite をまとめてクリアすること。
  - `app/viewer_test.go`: `target=all` handler、`hasStatus` / `hasResetStatus` / `cardStatusClass`、`.card.has-status` exact CSS、single summary reset button、per-flag reset 禁止、renderReadBadges で frame refresh、`favoriteByGallery.delete(id)`。

### 追加要件 actual UI confirmation（2026-11-05 07:28）

- Exact current Playwright target after explicit `browser_resize`:
  - URL `http://127.0.0.1:8731/search?newtab=1#/q/furry/page/1`
  - before resize fresh target measured `1920x953`; this exact `1280x900` is after explicit `browser_resize`, so it is not strict initial display pass.
  - after exact target resize: `.side` `480px`, `form#search` height `67.7734375px`, `#results` card count `25`, first card computed height `130px`.
- Direct server baseline via Playwright fetch before actual UI event:
  - HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=161;json=1`, `total=32916`, `ids.length=25`, `results.length=25`.
- Actual UI event `#search-submit` click:
  - UI `/api/search` response `HTTP 200`, `cache-control: no-store`, `x-search-timing: search=0;record=421;json=1`, `total=32916`, `ids.length=25`. Direct baseline and UI response match on total/ids/no-store.
- Exact target visual confirmation before summary reset:
  - card `4305294`: `class="card has-status"`, badge `表示済み 既読 保存済み`, single `button[data-reset='all']`, `hasStatus=true`.
  - Direct SQL before reset: `gallery_id=4305294`, `last_page=8`, `fully_read=1`, `favorite=0`, `viewed=1`, `viewed_at=2026-11-05T03:57:10Z`.
- Actual summary reset click on `4305294`:
  - Direct SQL after `target=all`: `last_page=0`, `fully_read=0`, `viewed=0`, `favorite=0`; `viewed_at=2026-11-05T03:57:10Z` preserved.
  - UI after reset: `card has-status`, badge `保存済み`, no reset button remains. `saved` remains because it is file-derived.
- Actual summary reset click on `4305205`:
  - UI before reset: `card has-status`, badge `表示済み`, `button[data-reset='all']`.
  - Direct SQL before reset: `last_page=1`, `fully_read=0`, `favorite=0`, `viewed=1`, `viewed_at=2026-11-05T04:20:07Z`.
  - Direct SQL after reset: `last_page=0`, `fully_read=0`, `favorite=0`, `viewed=0`.
  - UI after reset: `card`, no badge, no reset button, `hasStatus=false`.
- Status restore through actual UI route:
  - Click card `4305294` title link to viewer; route input actual event opened viewer tab and direct SQL after route/final-page actual event returned `4305294` to `last_page=8`, `fully_read=1`, `viewed=1`.
  - After `#search-submit` UI event, `4305294` again shows `card has-status`, badge `表示済み 既読 保存済み`, single `button[data-reset='all']`.
- Playwright screenshot copied to Google Drive:
  - `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\badge-status-frame-and-summary-reset-2026-11-05.png`

### Visual clipping correction（2026-11-05 07:35）

- 確認した問題: `outline` は card 外へ出るため、`.main` の overflow と `#results` の edge で左右が切り取られる可能性があった。window size 単独の原因ではなく、`outline` の外側描画がクリップされる問題として扱った。
- `app/search.html` を `.card.has-status { background-color: #252a31; box-shadow: inset 0 0 0 2px #8ab4f8; border-radius: 6px; }` へ変更。card height contract（130px）を壊さないため layout に影響する `border` / `padding` は使わない。
- `app/viewer_test.go` の exact CSS assertion を同じ new CSS へ変更。
- Canonical build after visual correction: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` 合格。canonical server pid `22316` で `http://127.0.0.1:8731/` を restart した。
- Exact current Playwright target: `http://127.0.0.1:8731/search?newtab=1&v=gray-card-20261105#/q/furry/page/1`。fresh target was initially `1920x953`; exact `1280x900` is after explicit `browser_resize`, so this is not a strict initial display pass.
- Direct server baseline via Playwright fetch before actual UI event: HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=798;json=1`, `total=32916`, `ids.length=25`.
- Actual UI event `#search-submit` click: UI `/api/search` response HTTP `200`, `cache-control: no-store`, `x-search-timing: search=0;record=41;json=1`, `total=32916`, `ids.length=25`. Direct baseline and UI response match on total/ids/no-store.
- Same target layout: `.side` width `480px`, `form#search` height `67.7734375px`, `#results` card count `25`, first card computed height `130px`.
- First status card exact visual properties: `class="card has-status"`, `background-color: rgb(37, 42, 49)`, `box-shadow: rgb(138, 180, 248) 0px 0px 0px 2px inset`, badge `表示済み 既読 保存済み`, summary reset button `data-reset='all'`. `#results` rect left/right was `512px` / `1249px`.
- Playwright screenshot saved locally: `docs/ai/tmp/gray-status-card-summary-reset-2026-11-05.png`.
- Google Drive screenshot copied: `I:\マイドライブ\kilo\2026-11-05-search-result-state-badges\gray-status-card-summary-reset-2026-11-05.png`

### ルール変更（2026-11-05 07:47）

- 画面確認に固定目標サイズを設けない。以後の基準は実際に表示している window sizeとする。
- 正典追加: `docs/ai/rules/screen-confirmation-size-rule.md`、`docs/ai/decisions/2026-11-05-window-size-based-screen-confirmation-rule.md`。
- `docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` の exact `1280x900` target-size項目を supersedeする。fresh target、direct server baseline、actual UI event、network/DOM/visual一致、既存回帰チェックは維持する。
- 本項目の残件は「固定 exact target」ではなく、人工 resizeで特定サイズへ合わせずに、fresh window sizeで actual confirmationし、合格条件を最終判断することとして更新した。

## Verification

- Commands:
  - `go vet .`：合格
  - `go test -count=1 -timeout 300s .`：追加要件変更後に `ok hitomi-hub/app 18.685s`。visual clipping correction後に `ok hitomi-hub/app 18.120s`。
  - `python -m pytest tests/ -v`：追加要件変更後に `53 passed in 10.16s`（canonical build rebuild 後）
  - `task_manager.py` Memory Bank update：Status `進行中` / Next Action「strict initial display（fresh isolated tabでresize前 exact 1280x900）は policy/tool決定が必要。追加要件 actual UI confirmationは exact current Playwright targetで完了し、canonical build rebuild・server restart・Google Drive screenshot保存済み」
- Canonical build after visual change:
  - `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`：合格
  - canonical server pid `29792` を stopし、fresh canonical exe から pid `30388` で `http://127.0.0.1:8731/` を restart した。visual clipping correction後の canonical serverは pid `22316`。
- Direct SQL helper used for row read: `python docs\ai\tmp\badges_db_check.py "S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh\hitomi.db" <gallery_id>`
- initial display：canonical fresh target re-measurement completed before resize at outer target `1280x900`; actual content viewport `1259x810`; `.side` `480px`, `form#search` `67.7734375px`, card count `25`, `.card` computed height `130px`, `#count`, `#search-page-status`, `#search-status`, `#perf-status`, direct server baseline, actual `#search-submit` CDP trusted click, `#lists-refresh`, reset button, visual read, and Drive confirmation all confirmed. Exact strict inner target `1280x900` remains tool limitation and is not treated as pass until policy/tool explicitly accepts canonical window-size as target.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
