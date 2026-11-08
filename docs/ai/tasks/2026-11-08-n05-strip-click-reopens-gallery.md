# N-05: strip クリックで同一 gallery を開き直している

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: strip のクリックで同じ gallery を開き直している。`handleCardClick` が `openGallery` を呼ぶため、全 prefetch を abort して queue を作り直し、`/api/viewed` も再送し、strip も再構築する。同一 gallery なら `current.page` を更新して `showPage()` するだけで済む。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-05 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（実測で確認）

- `handleCardClick`（`app/viewer.html:1376-1388`）: modifier guard → `newTabMode` 分岐 → `await openGallery(id, page)`。
  同一 gallery か否かの分岐はない。
- `openGallery`（`app/viewer.html:1178-1208`）: `++openGeneration` → `stripGroup`/`stripRenderedGallery`/`stripRenderedGroup` を null →
  `prefetchAbort.abort()`・`backgroundSignal.abort()`・`backgroundGalleryId = null` → route 書き込み → `post("/api/viewed")` → cached branch → `showPage()`。
- `showPage`（`app/viewer.html:1155-1177`）は page 移動として lightweight（`refreshStrip()` は group window が同じなら highlight-only、
  queue は `backgroundGalleryId !== current.id` のときだけ再計算）。
  つまり「page 移動として軽い経路」は既に showPage 側に存在し、strip クリックは `openGallery` を経由することでその経路を回避している。
- `setGalleryMeta` は cached card の `read_state` から `current` を再構築するが、同一 gallery では値は同一（`syncReadStateCache` が cache を更新しているのでズレない）。

### 有効性: 有効（実測で閉じた）

同一 click（page 3 → page 17・同一 group window・gallery cache 済み）での呼出回数。
node harness（`docs/ai/tmp/assess-20261108-1332/r05/harness_n05.mjs`・`harness_lib.mjs` の `bodyIn` で `app/viewer.html` の実ブロックを実行・外部 API なし・
計数用の `counter.X++;` 1 行のみ挿入、ブロックロジックは原文のまま）:

| 指標 | 現行（real） | 推奨（同一 gallery 分岐） |
|---|---|---|
| `openGallery` 呼出 | 1 | 0 |
| `abort()` 呼出 | 3（同一 controller 1 本） | 0 |
| new AbortController（queue 再構築） | 1 | 0 |
| `resetBackgroundPrefetch`（queue 再構築） | 1 | 0 |
| `post("/api/viewed")` | 1 | 0 |
| strip full rebuild（`#thumbs` innerHTML） | 1（img node 再生成） | 0 |
| `highlightStrip`（highlight-only） | 1（rebuild 経由） | 1 |
| strip handler attach / controls render | 1 / 1 | 0 / 0 |
| `renderReader` | 1 | 1 |
| priority run | 1 | 1 |
| background worker 再起動 | 1 | 0 |
| route 書き込み | 2 | 1 |
| `runBackgroundRead` helper | 1 | 1 |
| `setGalleryMeta` | 1 | 0 |

差分: HTTP 1 件（`/api/viewed`）、strip DOM 再生成 1 回（50 node  window 分）、AbortController 1 個、in-flight prefetch の放棄 1 回、route 書き込み 1 回。
（`queueLenDelta -10` は harness の setup 順序由来で、実測差分ではなく「queue を最初から再導出する」ことの副作用として読む。）

served 画面（正典 exe・fresh instance pid `11360`・isolated origin `n05-assess-20261108`・window outer `1678x1025` / inner `1665x932`・人工 resize なし・外部 API 不使用・gallery `4305294`＝local 8 ファイル）:

- 初回 open: `POST /api/viewed`（request `#3`）→ click 後にも `POST /api/viewed`（request `#14`）。`/api/read` は `#13`（open 後）と `#23`（click 後）。
- `#thumbs` の img node が click 後に再生成（before に付けた probe 属性が消失・WeakSet membership `false`）→ full rebuild 確認。page highlight は 1-2 → 5-6。
- reader 表示: `naturalWidth=512` の actual full pixels（`sourceType=local-direct`）。TN display fallback なし・generation guard・x-cache/no-store/objectURL 表示意味は非接触。
- server 応答 `read_state`（4305294）: `viewed=1`・`viewed_at=2026-11-05T03:57:10Z`（初回 timestamp を維持＝click 再送は idempotent・state 変化なし）・`last_page=6`（click 後の debounced read で更新）。
- console: `TypeError: Cannot read properties of undefined (reading 'entry')` を 2 件観測（`app/viewer.html:1114` の worker catch）。click 前の初回 read 時点で既に 2 件 → click 由来とは確定できず。症状は `KI-20261105-04`（queue item undefined race）と同一。

修正後の served 画面: 未確認（正典 source を変更できないため）。

### 正典照合

- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`（実在確認済み）:
  rule 5「Background prefetch should keep a stable queue per gallery and should not recompute all items for every page movement」。
  strip クリックは page 移動なので、現行経路（abort＋queue 再構築＋strip full rebuild）は rule 5 と衝突。TN display fallback 禁止・generation guard 条件は推奨修正で維持。
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`（実在確認済み）: TN は search-card cover 専用・reader/strip 表示源にしない。本項目の修正は表示源・display path を触らない（`renderReader` は 1 回のまま）。
- 既存テストの固定（変更しない・実測で通過確認）:
  - `app/viewer_test.go:1774-1796`: guard 位置、new-tab 分岐が `await openGallery(id, page);` より前であること、`#thumbs` への wiring。handler への分岐追加は new-tab 分岐の後に置けば通過。
  - `app/viewer_test.go:2300-2311`: `openGallery` 内の abort（route 前）、route 書き込み、`post("/api/viewed")` の ordering（cross-gallery 経路を非変更なら通過）。
  - `app/viewer_test.go:2325-2330`: `openGallery` 内の `backgroundGalleryId = null;`（非変更）。
  - `app/viewer_test.go:1994/2001`: 「gallery target が変わった時だけ queue 再計算」「page 移動で already-prefetched を reset しない」という意図 — 現行 click 経路は `backgroundGalleryId = null` 强制でこの意図と不整合（テスト文言自体は通過）。推奨修正は意図に整合。
  - `tests/strip_rebuild_dom_harness.mjs`・`tests/test_strip_rebuild.py`: `openGallery` の rebuild state reset と `showPage` の highlight-only を固定。同一 gallery 分支（案 A）は `openGallery` を呼ばないので通過。

### 対応の実現可能性と適切な修正方法（推奨）

推奨（案 A）: `handleCardClick` の `newTabMode` 分岐の後に同一 gallery 分支（4 行）。

```js
if (id === current.id) {
  const gen = ++openGeneration;   // generation guard は維持
  current.page = page;
  await showPage();
  if (gen !== openGeneration) return;
  return;
}
await openGallery(id, page);
```

- abort・queue 再構築・viewed 再送・strip full rebuild を呼ばない。`showPage` が highlight-only strip 更新と current priority を維持する。
- `++openGeneration` を置かない案（案 B）は不可: pending の cross-gallery open が後から完成して最新の click を上書き（harness 実測・noGuard: final gallery `202` / page `1`、latest target 失われる）。正典条件「stale async writes は generation guard で無視」を満たさない。
- `openGallery` 内 early return（案 C）は route/viewed ordering の意味とテスト固定を動かすため、分岐は handler 側が小さい。

実装時、同一 gallery 分支を固定する新テスト（abort 0・viewed post 0・rebuild 0）を追加するのが妥当（本項目ではテストを変更しない）。

### 副作用・影響範囲

- `/api/viewed` 再送の省略: `setViewed`（`app/library.go:93-101`）は `viewed_at` を初回のみ書き、reopen は idempotent。
  実測（4305294）で click 後も `viewed_at` は初回 timestamp を維持 → 状態意味は変わらない（承認不要）。
  `lastShown`/`complete`（`runBackgroundRead`）は click 後も `post("/api/read")` で更新される（実測 `last_page=6`）→ 既読意味は両経路同一。
- 承認案件（分離）: `POST /api/reset` `target=viewed` で `viewed=0` にした直後、同一 gallery 内の strip クリックが viewed を再点灯させる現行 observable 挙動。
  approved design（`docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md`）は viewed を「viewer document が gallery を開いた事件（route 入力時点）」とし、gallery 内の page 移動は開事件ではない。
  推奨修正ではこの再点灯がなくなる → 検索 card の「表示済み」 badge 意味（ユーザーに見える）に触れる論点として承認を要る。
  この点の DB 実測はユーザーの実際の read_state 行を書き換えるため実施していない（`library.go` と handler からの code-derived として記録。実測済みの初回 timestamp 維持とは区別）。
- prefetch abort 省略: server 側 gate（FIFO・`bg_grace_ms` 300・interactive search priority）の算術は変わらない（decision `2026-11-05-phase4-two-lane-gate-implementation.md`・`2026-11-06-tn-image-lane-60ms-approved.md`）。
  abort によって in-flight 要求が捨てられると既に確保した slot が無駄になる。省略は priority を弱めない（`currentPriorityDone` は新 displayed pair に対して引き続き設定される）。
- strip group: クリック対象は描画済み window 内の node なので group は同一（harness で group 判定変化なし）。`stripGroup` を保持しても表示結果は同一。
- `currentGalleryObjectUrlKeys`（objectURL 保護集合）: rebuild をスキップすると last render 時の集合が残る（同一 gallery・同一 window）→ eviction 意味不変。
- reader 表示の待ち意味（display path は read/lists を await しない）は不変。

推定規模: 小（`handleCardClick` に 4 行・テスト 1 件追加は任意）。

## Verification

- `python -m pytest tests/ -v` → `59 passed / 1 failed`（`13.47s`）。
  不合格は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来の既存 failure・R-10/R-12 報告済み）で本項目非接触。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet 合格（output なし）・`ok hitomi-hub/app 25.474s`（計 `28.34s`）。コード非変更。
- node harness: `node docs/ai/tmp/assess-20261108-1332/r05/harness_n05.mjs` → `0.4s`（数値表は上。結果ファイル `docs/ai/tmp/assess-20261108-1332/r05/result.json`）。
  harness は `tests/harness_lib.mjs` の `bodyIn` で `app/viewer.html` の実ブロック（handleCardClick・openGallery・showPage・renderStrip・highlightStrip・refreshStrip・ensureBackgroundPrefetch・resetBackgroundPrefetch・stripItems・stripGroupPage・setRoute・route）を読み、
  `new Function` で実行。葉関数は計数 stub、計数用に `counter.X++;` を関数先頭へ 1 行挿入（ロジックは原文）。leaf stub の `stripThumbSource` は node 計数用の placeholder（TN 表示意味の判定には使わない）。
- 画面確認（正典 exe served・fresh instance・isolated origin・window size 基準）: before/after 同一条件（window outer `1678x1025`・inner `1665x932`・viewport 全体・Playwright screenshot）。
  - Drive 保存先: `I:\マイドライブ\kilo\2026-11-08-n05-strip-click-reopens-gallery\`
    - `n05-strip-click-before-2026-11-08.png`（click 前: page 1-2・strip 8 node・`#search` なし reader local-direct）
    - `n05-strip-click-after-2026-11-08.png`（click 後: page 5-6・strip node 再生成・reader actual pixels）
    - `n05-strip-click-confirmation-2026-11-08.txt`（手順・実測値）
  - local copy（project `scratch\n05-assess-20261108\` の 2 枚）は正典 `kilo-safe-remove` でゴミ箱へ送った。
    Playwright output 直下（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`）の 2 枚は正典 tool の project root 外削除禁止でゴミ箱へ送れていない（残件）。
  - 修正後の served 画面は未確認（正典 source 変更不可）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

（本項目は調査のみ。known-issues 新規記録は行っていない。viewed 再点灯の意味変更論点は承認案件として本記録に分離記録済み。既存 known issue `KI-20261105-04`（undefined queue item race）は症状一致のみ・本項目で解決していない）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
