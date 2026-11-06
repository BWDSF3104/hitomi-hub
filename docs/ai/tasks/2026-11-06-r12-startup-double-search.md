# R-12: 起動時に restoreSearchState と runSearch が二重に走りカード取得が捨てられる

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: ページロードで card fetch が 1 回だけ（searchGeneration が restore 分を進めない）。cold 確認の汚染源としても記録。記録: R-12

## 意図 / 背景

- 外部指摘（Claude 2026-11-06・読み取り検証）の R-12。`restoreSearchState()` が `loadSearchCards(ids, searchGeneration)` を呼ぶ直後に `runSearch()` が `searchGeneration` を進めるため、restore 分の card fetch が worker の generation guard で捨てられる。每次 page load で card fetch が 2 回走る（restore 分は無駄）。cold 確認の汚染源にもなる（`docs/ai/rules/confirmation-test-rule.md` §3 の既知汚染源）。
- 本記録は現行コードの調査・実測・修正方法の推奨まで。コード変更なし（変更したのは個別タスク記録と `docs/ai/tmp/assess-20261106-1056/` のスクラッチのみ）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠（行番号は現物で確認）

- `app/search.html:178` `let searchGeneration = 0;`、`app/search.html:168` `const SEARCH_CARD_CONCURRENCY = 8;`。
- 起動列（`app/search.html:997-1009`）: `loadTitleGroups(); showLists();` → `restoreSearchState(); runSearch();`（hash 分岐の両側で同一構成）。
- `restoreSearchState()`（`app/search.html:450-469`）は render 後に `loadSearchCards(ids, searchGeneration)` を呼ぶ（`app/search.html:468`）。await されない fire-and-forget で、worker は synchronous 部分で即 `api("/api/search-card/<id>")` を発火する（`app/search.html:574-617`、worker 数 `min(8, ids.length)`）。
- 直後の `runSearch()` が `const searchGen = ++searchGeneration;`（`app/search.html:699`）で generation を進め、自身の card run（`app/search.html:725`）を走らせる。restore 側の worker・processCard は guard（`app/search.html:575`、`581`、`605`）で停止し、すでに発火した fetch の結果はすべて捨てられる（render されない・`processed` にも入らない）。
- 条件: 二重実行は saved state がある page load で起こる（typical）。state なし isolated origin では `restoreSearchState()` が早期 return（`app/search.html:452`）し単発。指摘の「每次 page load」は saved state 前提で有効。

### 実測（harness・既存流儀 tests/harness_lib.mjs 方式・node）

- `node docs/ai/tmp/assess-20261106-1056/r12_startup_card_fetch_harness.mjs` → 17 checks passed。
- 現行コード（saved state ids=25・concurrency=8）: startup card requests = 33 = restore 分 8（`/api/search` 応答前）+ search run 25。restore 分 results は guard で全棄て（render 0、`cardsLoaded` 25 = search run のみ）。二重実行を数値で確定。
- 推奨修正適用（スクラッチコピー上・正典 source は未変更）: restore 分 card fetch 0、startup card requests = 25（単発）。二重実行が消える: requests 33 → 25、runs 2 → 1、無駄 8 → 0。処理・表示結果は修正前と同一。

### 実測（served 画面・canonical build・window 1665x932・before/after・Drive 保存）

- 画面確認は `docs/ai/rules/confirmation-test-rule.md` の流儀（fresh canonical build + fresh server instance・同一 window size・full page 同一被写体）で実施。測定前に応答済みの instance だったため cold claim はせず warm として記録。
- Load A（isolated origin・localStorage clear 後初回ロード・state なし）: startup card fetch 1 run（card requests 25、順序 search → card）。restore 経路不发火。
- Load B（reload・state あり）: startup card requests 33（restore 分 8 が `/api/search` 応答前、search run 25、先頭 8 id が二重 fetch）。`perf-status` は「カード 25・cache=25」、status「検索完了・カード表示済み 25件」→ restore 分が棄てられていることを served 画面で確認。
- served 画面での「単発」確認は未確認（正典 source を変更していないため canonical build は現行二重実行を served する。単発の数値は harness スクラッチのみ）。実装後に実装段階で served 確認が必要。
- Google Drive 保存先: `I:\マイドライブ\kilo\2026-11-06-r12-startup-double-search\`（`r12-startup-single-run-before-20261106.png`・`r12-startup-double-run-after-20261106.png`・`r12-startup-card-fetch-confirmation-20261106.txt`）。local copy は `kilo-safe-remove` でゴミ箱へ送った。

### 有効性

- 有効（確定・実測済み）。二重実行と guard による棄ては機構として確定し、実測数値（33 requests・restore 分 8・render 0・cardsLoaded 25）で閉じた。

### 対応の実現可能性と適切な修正方法

- 推奨（案 A): `restoreSearchState()` の card fetch 呼び出し（`app/search.html:468`）を削除し、render 専にする（startup の card fetch は `runSearch()` の単発 run のみ）。小規模（1 行削除相当）。ユーザーに見える挙動は現在の状態と同一（現状でも card の完成は search run 待ち）。cold 汚染源（rule §3）はこの削除で直接消える。
- 実装時テスト影響（re-anchor が必要): `tests/search_visible_priority_harness.mjs` の check「restoreSearchState uses the same loadSearchCards path」（exact string `loadSearchCards(ids, searchGeneration);`）と pytest wrapper の required label。意味は「startup で card fetch 単発・restore 分不发火」へ更新。Go 固定テスト `TestEmbeddedUIPreservesSearchStateAcrossReload`（`app/viewer_test.go:2369-2407`）は `restoreSearchState(); runSearch();` と guard 文字列の維持を要求するため案 A と両立（initBody 不変）。単発回数を実測する harness test を既存流儀で追加するのが合格条件に appropriate。
- 代替案（案 B・非推奨): hash・saved state 一致時に startup を skip して restore を単発 run とし、runSearch の generation を進めない（B-3 指摘の「skip on hash match」の另一半）。単発になるが、fresh 結果 refresh を user に見える意味で変える（stale ids 即時表示）+ generation を進めない流儀は guard-first 正典（`docs/ai/decisions/2026-11-03-judgment-waiting-reduction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-required-function-verification.md`）より UX 意味変更の承認を要る。B-3 の「searchCard cache 再利用」の半分は searchCardCache 廃止（guard `app/viewer_test.go:2157-2162`・R-08 照合）で無効。

### 正典照合（矛盾記録)

- 追加指示が挙げた B-03/B-06 のパス（`docs/ai/tasks/2026-11-03-b03-search-card-generation-guard.md`・`docs/ai/tasks/2026-11-03-b06-startup-double-search.md`）はプロジェクト内に存在しない（Kilo_Commons 全体を検索。同名実在ファイルは別項目: `2026-11-03-b03-fav-state-cache-rollback.md`・`2026-11-03-b06-vt-empty-results.md`）。
- 辿れた正典採用: `docs/ai/tasks/2026-11-02-external-code-review-findings.md` B-3 startup（同一件・実装未開始・対応「hash 一致で skip、searchCard cache 再利用」）、`docs/ai/tasks/2026-11-05-assess-20261105-0006-phase1-visible-priority.md`（既知未修复として申し送り・page 1 で restore 由来 card fetch 実測 5 件・concurrency 4 当時の数値）、`docs/ai/rules/confirmation-test-rule.md` §3（cold 汚染源として起動時 fetch を明記）。日付新しい文書（2026-11-05・2026-11-06）が古い B-3 と矛盾せず、案 A（B-3 の skip 側の現代表現）と一致。B-06 引用文（「runSearch の generation を先行させない」）は案 B に相当し、guard-first 正典より UX 承認が要る論点として分離した。

### 副作用・影響範囲

- data lane gate 消費が load ごとに 8 件減る（R-11 gate 意味には接触なし・発火回数の減少のみ）。
- guard・IntersectionObserver・perf 指標・TN cover・prefetch・DB 書き込み口・list debounce は非接触。`armPrefetchIdle()` は search run 完了後にのみ走る（現状どおり）。
- cold 確認 hygiene に positive（rule §3 の汚染源が解消）。rule 変更は不要。

### 推定規模・優先順位付けに効く要素

- 推定規模: 小（search.html 1 行削除相当 + harness check re-anchor + 単発回数テスト追加）。
- 重要度: 中（効率・測定 hygiene）。緊急度: 低〜中。リスク: 低（ユーザー表示挙動不変・guard 不変）。依存: 固定テスト re-anchor 1 箇所。単独で実施可（他の R 項目と独立）。

## Verification

- `node docs/ai/tmp/assess-20261106-1056/r12_startup_card_fetch_harness.mjs` → 17 checks passed（現行 33 requests・2 runs・restore 分 8 棄て、修正後 25 requests・1 run・restore 分 0）。スクラッチ harness は既存テスト・正典 source を変更していない。
- `python -m pytest tests/ -v` → 54 passed・13.75s、1 failed。failed は `test_canonical_build_freshness_rule_exists`（R-12 範囲外・本調査で変更していない既存不整合: AGENTS.md Rules pointer が 2026-11-06 の `docs/ai/rules/confirmation-test-rule.md` 集約で単一 pointer 化され、test が要求する `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` pointer 文字列を持たない。freshness の byte embed test `test_canonical_executable_embeds_current_pages` は合格）。推測ではなく実測結果の記録として区別する。
- canonical build（fresh）+ canonical server（port 8731・p3_canonical_fresh・pid 25208・稼働継続・decision 2026-11-01-server-exe-keep-running）で served 画面確認。window size 1665x932（実測・before/after 同一）。cold claim はせず warm として記録。served 実測: state なし load = card requests 25（単発）、state あり reload = 33（restore 分 8 が search 応答前、先頭 8 id 二重）。served 画面の単発確認は未確認（正典 source 未変更・実装段階）。
- Drive 保存先: `I:\マイドライブ\kilo\2026-11-06-r12-startup-double-search\`（before/after png + confirmation txt）。local screenshot copy は `kilo-safe-remove` でゴミ箱へ送った。repo に binary 保存なし。
- コード・既存テスト変更なし。変更は本記録と `docs/ai/tmp/assess-20261106-1056/`（スクラッチ）のみ。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
