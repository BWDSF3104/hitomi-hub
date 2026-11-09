# N-14: perfStatus が検索をまたいで累積し数値が食い違う

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `perfStatus` が検索をまたいで累積している。`cardsLoaded` と `cardTotalMs` は検索ごとにリセットされない。「計測」行の `searchMs` は今回分なのにカード数と時間は累計で、数値が食い違う。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-14 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 有効性

- 有効（機構確定・harness と served 画面の両方で実測して閉じた）。

### 現行実装と判断根拠

- `perfStatus` は `app/search.html:180`（module レベル）で1回だけ初期化。リセット・再初期化の文はファイル内不存在（harness 静的 check: 初期化1箇所・リセット文なし）。
- 累計側: `cardsLoaded++`・`cardTotalMs +=`（`processCard` 成功側 `app/search.html:581-582`、catch 側 `595-596`）、`cacheDistribution` 累計（`recordPerfCache` `475`）。
- 今回分側: `runSearch` の再代入（`perfStatus.searchMs =` `712`、`serverTiming =` `713`）。
- 「計測」行（`renderPerfStatus` `486`）は両方を同一行で表示 → 一行内に今回分（searchMs・サーバー）と累計（カード・キャッシュ）が混在。

### 実測（harness・mock・外部 API なし・同一 page instance で 2 runs）

- `node docs/ai/tmp/assess-20261108-1332/r14/r14_measure.mjs`（`tests/harness_lib.mjs` の `bodyIn` で `app/search.html` 実ブロック抽出・`new Function`・決定論的 clock）:
  - search 1 後: `計測: /api/search 40ms・サーバー search=7;record=35;json=2・カード 25 2180ms・キャッシュ cache=26`
  - search 2 後: `計測: /api/search 120ms・サーバー search=7;record=35;json=2・カード 50 5450ms・キャッシュ cache=52`
  - searchMs 40→120（累計 160 ではなく今回分へ更新）、cardsLoaded 25→50、cardTotalMs 2180→5450、cache 26→52（累計が残る）→ 項目原文の食い違い再現。9 checks passed。
  - 注記: `cardTotalMs` は 8 worker の scheduling を反映（25 card × 20 ms の下界 500 ms に対し 2180 ms）。claim は累计関係のみで使用。

### served 画面確認（合格）

- 正典 exe（freshness test `test_canonical_executable_embeds_current_pages` 合格・コード非変更）・fresh instance（pid 21148・port 8742・本確認の要求のみに応答）・isolated origin `n14-assess-20261108-1332`・window size 実測 outer 1678x1025 / inner 1665x932（人工 resize なし・before/after 同一条件・同一被写体 `#perf-status`・同一取得方法）。
- before（search 1 = load 時 runSearch・既定 query `furry`）: `計測: /api/search 1602ms・サーバー search=1509;record=88;json=1・カード 25 2862ms・キャッシュ miss=25`
- after（search 2・query `kemonomimi`・`#search-submit` click）: `計測: /api/search 1259ms・サーバー search=1231;record=26;json=1・カード 50 18814ms・キャッシュ miss=49 cache=1`
- before → after: searchMs は今回分へ更新（1602→1259）、カード 25→50・時間 2862→18814ms・cache 25→50 件は累计維持 → served 画面で食い違い確認。両 run「検索完了・カード表示済み 25件」。
- 保存先: `I:\マイドライブ\kilo\2026-11-08-n14-perf-status-accumulates-across-search\`（`n14-perf-row-search1-before-20261108.png`・`n14-perf-row-search2-after-20261108.png`・`n14-perf-row-confirmation-2026-11-08.txt`）。local copy（project root 内 scratch）は正典 `kilo-safe-remove` でゴミ箱へ送った。Playwright の snapshot（.yml）は project root 外のため削除せず残件として記録。
- 留意: `cache=1` は共有 runtime root（`p3_canonical_fresh`）の galleryinfo disk cache ヒット（prior run 由来）。本項目の claim は cache 状態依存ではない。console error は `favicon.ico` 404（無関係）。card fetch の外部 API 呼出しは decision `2026-11-01-hitomi-hub-tests-may-call-external-api.md` の許可範囲。

### 正典照合（「計測」行の意味固定）

- 参照先: `docs/ai/decisions/2026-11-01-p3-implementation-decisions.md`（UI は `performance.now()` で search/card fetch 時間を `#perf-status` として表示 — 累计意図の記述なし）、`docs/ai/tasks/2026-11-03-a05-search-card-sequential.md`（per-card `+=` 計測機構の維持記録 — search をまたぐ累计意味の固定なし）、`docs/ai/decisions/2026-11-06-ui-polish-jp-copy-and-anchors.md`（label `perf`→`計測` の表示文字列のみ）。
- 既存テスト `app/viewer_test.go:3430`（`TestEmbeddedUIHasSearchMeasurementStatus`）の pin: `perfStatus.searchMs = performance.now() - searchStart;`（今回分形）と `perfStatus.cardTotalMs += performance.now() - cardStart;`（per-card 累计機構）。跨 search の累计「値」は固定していない。ラベル文字列に累计語なし。
- 判定: 「累計表示が意図」という正典記録は不存在 → 「今回分表示が意図（label と現行今回分形）」として採用。リセット案は意味変更ではなく bug 修正と判定（根拠: 正典記録なし・テスト pin は今回分形の literal・同一行内で searchMs/server が今回分形であること）。ユーザーに見える現在値（累計表示）の変更側は追加指示どおり承認案件として分離。

### 対応の実現可能性・修正方法（推奨）

- 案 A（推奨・小）: `runSearch` 冒頭で `cardsLoaded`・`cardTotalMs`・`cacheDistribution` を初値へリセット（`searchMs`・`serverTiming` は既に今回分）。約4行。`cardTotalMs +=` literal を維持するので pin テスト（viewer_test.go:3453）通過・既存テスト変更なし。
  - generation guard を壊さない: stale run の card 増分は增量前 guard check（`560`/`574`/`580`/`594`）で停止済み。`renderPerfStatus` guard（`481`）・per-card 即時表示維持。card fetch 優先度（`takeNextSearchCard`・interactive priority）非接触。
- 案 B（意味追加・代替・承認不要): 同一行で per-run と累計を両方表示（例「カード 25/50・15952/18814ms」）。既存 literal・テスト維持、捨てられる意味なし。行が長くなる。
- 承認案件として分離: 案 A はユーザーに見える数字を累計→今回分へ変える（意味変更側の実施には承認が必要）。累计表示が意図だった場合は正典変更を要る方針変更。

### 副作用・影響範囲

- 「計測」行の消費は表示のみ（機能影響なし）。`cacheDistribution` は案 A で cards と同じく今回分へ（同一行の整合）。`serverTiming` は文字列で累计不可・今回分維持（変更なし）。
- `tests/search_visible_priority_harness.mjs:55` の stub `perfStatus` は自 scope・非影響。`tests/r12_startup_single_run_harness.mjs` の `cardsLoaded` check は単発 run（案 A で結果同一・非変更通過）。
- 代替案（機構変更なし）: 表示のみ案 A（最小）。累计を残すなら案 B。

### 推定規模・優先順位要素

- 規模: 小（案 A 約4行・案 B 小〜中）。
- 重要度: mid〜low（ユーザー誤読・observability 表示 bug。機能危害なし）。緊急度: low（再現性はあるが blocker なし）。リスク: low〜mid（承認要論点は意味変更側のみ。guard・優先度・テスト pin は非触）。依存: viewer_test.go:3430 の literal pins、confirmation-test-rule、意味変更の承認ゲート。外部依存なし・単独実施可。

## Verification

- `python -m pytest tests/ -v` → 59 passed / 1 failed（10.75 s）。failed は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来の既存 failure・KI-20261106-04・本項目非接触）。
- node harness `node docs/ai/tmp/assess-20261108-1332/r14/r14_measure.mjs` → 9 checks passed（0.089 s・外部 API なし）。
- 既存 harness 非変更・通過: `node tests/search_visible_priority_harness.mjs` → 24 checks passed（約0.1 s）、`node tests/r12_startup_single_run_harness.mjs` → 16 checks passed（約0.1 s）。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean・ok 25.436 s（`TestEmbeddedUIHasSearchMeasurementStatus` 含む全通過・コード非変更）。
- served 画面確認合格（上欄。Drive 保存・local copy ゴミ箱済み）。
- 変更したのは本タスク記録と `docs/ai/tmp/assess-20261108-1332/r14/`（スクラッチ）のみ。`src/`・`app/` の変更なし、既存テストの変更なし。Status と Next Action は変更なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 2026-11-09 再調査（assess-20261109-1320）

### 有効性

- 有効（不変）。現行コードにリセット未導入で、served 画面で食い違いを再実測して閉じた。

### 現行実装と判断根拠（再確認）

- 現行 `app/search.html` にリセットは入っていない（未解決）。`perfStatus` 初期化は `191`（module レベル・1 箇所）のみ。`runSearch` 冒頭（`774-775`）は `searchCardsCompleted = false;` のみで `cardsLoaded` / `cardTotalMs` / `cacheDistribution` のリセット文なし。ファイル内リセット文の静的 check（harness 再実行・現行ファイル対象）→ 初期化 1 箇所・リセット文なしで 9 checks passed。
- 呼出経路: `runSearch` 呼出は `523`（restoreSearchState・load 時）・`963`（form submit）・`1062`・`1079`・`1091`・`1094`（route/boot）。前批と同一構造（単入口・リセット経路なし）。

### 前批記録との矛盾確認

- 内容の矛盾なし、line シフトのみ（+11〜+92）。前批 → 現行: 初期化 `180`→`191`、`recordPerfCache` `475`→`491`、`renderPerfStatus` `486`→`497`/`501-502`、card 増分成功側 `581-582`→`662-663`、catch 側 `595-596`→`676-677`、`runSearch` 再代入 `712-713`→`796-797`。card 経路の searchGen guard（現行 `661`・`675`）と `renderPerfStatus` の空表示 guard（`497`）は前批記述どおり維持。
- 軽微な表示 drift: harness（`docs/ai/tmp/assess-20261108-1332/r14/r14_measure.mjs` check 86）のラベル文字列が「line 180」を示す。check 自体は occurrence 回数とリセット文の不在のみを検定しており、現行 `191` と矛盾しない（ラベルの旧 line 参照は cosmetic）。
- テスト pin（`TestEmbeddedUIHasSearchMeasurementStatus`）: 現行 `app/viewer_test.go:3584`・`3593`・`3596`（前批 3430/3453 相当の line シフト）。pin 内容同一（今回分形 literal と per-card `+=` 累计機構）→ 案 A（runSearch 冒頭リセット）で通過維持、テスト変更不要。

### 正典照合（「今回分が意図」判定）

- 参照先（前批と同じ）: `docs/ai/decisions/2026-11-01-p3-implementation-decisions.md`（`#perf-status` 表示・累计意図の記述なし）、`docs/ai/tasks/2026-11-03-a05-search-card-sequential.md`、`docs/ai/decisions/2026-11-06-ui-polish-jp-copy-and-anchors.md`（label `perf`→`計測` の表示文字列のみ）。現行 decisions・rules に「累计表示が意図」を固定する新規記録なし → 前批判定（今回分が意図・リセット案は bug 修正）は現行正典と矛盾なし。意味変更側（ユーザーに見える数字の累計→今回分）の承認案件分離も維持。
- 文書矛盾処理: 前批は pytest で `test_canonical_build_freshness_rule_exists` の failure（KI-20261106-04）を記録。現行実測では同 test 合格（2 passed）。日付優先規則により現行実測を採用（古い状態記述は根拠にしない）。

### served 画面確認（合格・2 回目検索の表示数値）

- 正典 exe（freshness test 合格・コード非変更）・fresh instance（pid 15736・port 8731・本確認の要求のみに応答）・isolated origin `n14-assess-20261109-1320`・window size 実測 outer 1679x1025 / inner 1666x932（人工 resize なし・before/after 同一条件・同一被写体 `#perf-status`・同一取得方法）。
- before（search 1・load 時 runSearch・query `furry`）: `計測: /api/search 1518ms・サーバー search=1474;record=39;json=1・カード 25 3544ms・キャッシュ miss=25`（検索完了・カード表示済み 25件）
- after（search 2・`kemonomimi`・`#search-submit` click）: `計測: /api/search 2529ms・サーバー search=1207;record=1319;json=1・カード 50 20386ms・キャッシュ miss=50`（検索完了・カード表示済み 25件）
- before → after: `searchMs`・`serverTiming` は今回分へ更新、`カード` 25→50・`カード時間` 3544→20386ms・`キャッシュ` miss=50 は search 2 単独分（25）ではなく 2 回の累計表示 → 食い違いを served 画面で再確認。fresh instance で search 1 が miss=25 → miss=50（=25+25）であり、累計は cache 状態非依存の機構（`cardsLoaded++`・`cardTotalMs +=`・`cacheDistribution`）で確定。
- 保存先: `I:\マイドライブ\kilo\2026-11-09-n14-perf-status-accumulates-across-search\`（`n14-perf-row-search1-before-20261109.png`・`n14-perf-row-search2-after-20261109.png`・`n14-perf-row-confirmation-2026-11-09.txt`）。local copy（project `scratch/`）は正典 `kilo-safe-remove` でゴミ箱へ送った。共用 browser に他の完了済み委譲の残存 tab があったが、isolated origin は本確認専用で、server instance は起動後本確認の要求のみに応答（#2 の 8731 使用分は停止後の別 instance）。

### 対応の実現可能性・修正方法（推奨・不変）

- 案 A（推奨・小・約 4 行）: `runSearch` 冒頭で `cardsLoaded`・`cardTotalMs`・`cacheDistribution` を初値へリセット（`searchMs`・`serverTiming` は既に今回分）。pin テスト通過（現行 go test で確認）。guard・card 優先度・既存テスト非接触（前批判定維持）。
- 案 B（代替・意味追加・承認不要): 同一行で今回分と累計を併記。前批判定維持。

### 副作用・影響範囲

- 前批の範囲記述のまま（表示のみ・`serverTiming` 文字列で累计不可・stub harness 非影響・r12 harness 単発 run 通過）。追加の影響なし。

### 推定規模・優先順位要素

- 規模: 小（案 A 約 4 行）。
- 重要度: mid〜low（observability 表示 bug・ユーザー誤読・機能危害なし）。緊急度: low（blocker なし）。リスク: low〜mid（承認要論点は意味変更側のみ）。依存: viewer_test.go pin、意味変更の承認ゲート。外部依存なし・単独実施可。

### Verification（本再調査）

- `python -m pytest tests/ -v` → 60 passed（27.59 s・freshness test 含む。前批 failure KI-20261106-04 は現行合格）。
- `node docs/ai/tmp/assess-20261108-1332/r14/r14_measure.mjs`（現行ファイル・mock・外部 API なし）→ 9 checks passed。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean・ok 25.293 s（`TestEmbeddedUIHasSearchMeasurementStatus` 通過）。
- served 画面確認合格（上欄。Drive 保存・local copy ゴミ箱済み）。card fetch の外部 API 呼出しは decision `2026-11-01-hitomi-hub-tests-may-call-external-api.md` の許可範囲（search 1 回・search-card 25 件×2）。正典 DB・完成フォルダ・config は直接変更なし。
- 変更したのは本タスク記録（本見出し）と Drive 保存、スクラッチ（生成後ゴミ箱へ送った）のみ。`src/`・`app/` の変更なし、既存テストの変更なし。Status・Next Action は変更なし（親管理）。
