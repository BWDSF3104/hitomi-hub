# N-10: titleGroups を毎カード localStorage へ全量書き込み

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 画面確認（served）不合格のまま: 共用 browser の占有解消後、記録の実装 plan（debounce＋同値 skip・named callback 流儀）と scratch の harness コピーを再適用し served 画面確認を再実行 → 正典検証 → task_manager end。実装分は不合格により破棄済み（HEAD d80ca07 復元）

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `titleGroups` を毎カード localStorage へ全量書き込んでいる。`setSearchTitleGroup` は既存が日本語版でない場合や日本語版自身の再描画のたびに `saveTitleGroups()` を呼ぶ。1 ページで先読み含め最大 75 回、同期で map 全体を stringify。map は見た title 全部が貯まり続ける。debounce して件数上限を設ける案。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-10 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装と判断根拠（コード確認）

- 机构は項目原文どおり（実コードで確定。源码は非変更・記録のみ）。
  - `app/search.html:167` `TITLE_GROUPS_STORAGE_KEY = "hitomi_hub_title_groups"`、`:181` `titleGroups = new Map()`（上限量なし・`loadTitleGroups()` は起動時 `:996` のみ）。
  - `saveTitleGroups()`（`:222-226`）は呼出ごとに `JSON.stringify(Object.fromEntries(titleGroups))`（map 全量）＋ `localStorage.setItem`。例外は `catch` で握りつぶす（上限超過で保存が黙って止まる: `docs/ai/tasks/2026-11-02-external-code-review-f501e2b.md` 照合済み・確認）。
  - `setSearchTitleGroup`（`:228-236`）は条件 `!existing || existing.language !== "japanese" || r.language === "japanese"` のとき map 更新＋`saveTitleGroups()`。呼出口は `renderSearchCard`（`:400-401`・毎カード）と `applyPrefetchCardKnowledge`（`:745-748`・先読みカード毎）。
  - ページ 25 件（`limit=25` `:710`）＋先読み `PREFETCH_AHEAD_PAGES = 2`（`:779`）→ 1 検索で最大 25+25+25 = 75 呼出。
  - skip case（save が走らない）は「group が既に日本語解決済み＋非日本語カード」のみ。新 key・非日本語 group の上書き・日本語カードの再描画（条件3）は呼出ごとに save。page 移動で再描画された page の条件該当カードは毎回再 save。

### 有効性

**有効（機構確定＋実測で閉じた）**。判断保留なし。原文数値の補正: 「1 ページ 75 回」は cold・全 title 新規の上界として実測確認（75/75）。steady state では条件クラス별の実測値へ下がる（下表）。

### 実測（node harness・mock localStorage・harness_lib `bodyIn` で app/search.html 実ブロック抽出・new Function 実行）

scratch: `docs/ai/tmp/assess-20261108-1332/r10/r10_measure.mjs`（実ブラウザ storage 非汚染）。呼出/save/壁時間は mock 実測。実 browser の `setItem` は mock に加えて同期 storage write がかかる（未測定・区別）。

呼出回数（1 ページ・cold・全 title 新規）: 呼出 75 → save 75（75 回は上界で成立）。

条件クラス（warm map で page 25 再描画・25 呼出の内訳）:

| 条件 | save 走否 | 実測（混成 page） |
|---|---|---|
| 新 key | 走る | 5 |
| 既存 group 非日本語（上書き） | 走る（再描画のたび） | 5 |
| 日本語カード（group 日本語・再描画） | 走る（再描画のたび） | 5 |
| group 日本語解決済み＋非日本語カード | skip | 10 |

warm 全 distinct page の round trip（page 移動での再描画）: 25/25 save（新 key なしでも再描画で毎回 save）。

stringify 壁時間（1 呼出・map warm・caller 側）と累计（75 呼出）:

| n | median ms/呼出 | max ms/呼出 | 累计 ms（75） | payload/回 |
|---|---|---|---|---|
| 1,000 | 0.858 | 5.704 | 64.35 | 118,793 B |
| 10,000 | 9.964 | 16.237 | 747.28 | 1,216,671 B |

debounce（呼出の map 更新は同一順・persist だけ page 末 1 回へ束ねる）と件数上限（bounded・cap 1024＝C-05 convention）の simulated 比較:

- cold 50 カード: sync 50 write・0.457 ms → debounce 1 write・0.067 ms。
- warm n=10,000 の page: sync 75 write・約 747 ms（上表）→ debounce 1 write・11.051 ms。
- bounded cap 1024（実測・no re-access 条件）: 保持 1,024（unbounded 10,000 と対比）・flush payload 124,929 B・flush 5.709 ms。
- 淘汰順は re-access があるときに差が出る（実測 check: k0 re-access 後、last-used は k1 を淘汰、insertion（作成順）は k0 を淘汰）。

localStorage 上限（外部仕様）: origin あたり約 5 MB 級。n=10,000 で payload 約 1.2 MB、growth 継続で超過 → 現行は catch 握りつぶしで persist が黙って止まる（2026-11-02 照合）。

### 正典 condition 照合（破らない）

- `setSearchTitleGroup` の既存意味（group 決定条件・同一 page 内での group 決定順）を変えない形で debounce は成立（map 更新は呼出ごと同一順、persist だけ束ねる）。実測はこの前提で実施。
- 既存テスト照合（変更なし・全通過）: `app/viewer_test.go`（`TestEmbeddedUIHasSameTitleJapaneseAnchor`）は storage key literal（`:3463`）、`loadTitleGroups`/`saveTitleGroups`/`setSearchTitleGroup`/`sameTitleJapaneseAnchor` の関数名（`:3466`）、`Object.fromEntries(titleGroups)`・`localStorage.setItem(TITLE_GROUPS_STORAGE_KEY`（`:3469`）、renderSearchCard 内 `setSearchTitleGroup(r);`（`:3478`）を pin。`tests/search_prefetch_harness.mjs`（`:20`）は patch helper block 内 `setSearchTitleGroup`・`appendChild`・innerHTML なしを pin。`tests/search_manual_jp_harness.mjs`（`:44`）は自前の setSearchTitleGroup コピー（save なし）で group 判定意味を pinし、manualBlock の `localStorage` 禁止（`:20`）と schedulerBlock の `localStorage` 禁止（prefetch `:34`）で block 境界を pin。→ 実装時は関数名・key・anchor を保持し、debounce helper を manualBlock/patchHelperBlock/schedulerBlock の境界外（persist helper 群の箇所）へ置けば全テスト通過（既存テスト変更不要）。呼出ごとに save する同期性・debounce の有無を固定するテストはなし。

### 淘汰順の照合（正典流儀の分岐・記録）

- C-05（memory cache 流儀）: last-used（access order）順 — `boundedCachePut`（`app/hitomi.go:92-107`）は hit で `lastAccess` 更新・`lastAccessed()` 最小を淘汰（flaky tie 教訓 `docs/ai/known-issues/archive/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`）。
- R-15（disk 流儀）: 作成順 — `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` R-15（作成順）・`docs/ai/decisions/2026-11-08-r15-cache-entries-table-supersede.md`（disk 層は access order を持たない、created_at ASC, rowid ASC）。
- 正典流儀同士が分岐（memory=last-used、disk=作成順）。localStorage の titleGroups は client 永続 storage で正典流儀の直接適用例なし。R-15 disk  analogy で**作成順推奨**（Map insertion order から free に読める・lastAccess field 追加＝payload 増なし）。C-05 memory analogy（last-used）を採用する場合は entry へ lastAccess 記録が必要。順選択・件数上限は「以前見た title の group が消える」意味変更の一部なので承認案件（下記）。

### 副作用・影響範囲（現挙動 → 変更後・承認案件の分離）

- debounce 側: 意味変更なし（group 決定順・判定・key 不変。persist 束ねのみ）。追加窓: flush 前での tab close/crash で当該 run の write 消失窓（現行は即 save・窓なし・小）。
- 件数上限（bounded）側: 「見た title 全部を覚える」意味を失う。ユーザーに見える影響: 以前見た title の group が消える → `同名の日本語版: #id` link（badge 相当）が消える、再検索で日本語版判定が変わりうる（group 再解決）。これはユーザーに見える意味変更 → **承認が必要として分離**。順（作成順/last-used）と cap 値（正典 convention 値から選択・invented number 禁止: C-05=1024 entry、R-15 disk=8000 件）も同一承認案件の一部。
- 現行機能リスク（機構確定）: 全量 growth で storage 上限超過 → catch 握りつぶしにより persist が黙って止まる（2026-11-02 照合）。bounded 化はこのリスクの解消側（意味変更なし部分）。

### 画面確認（served）

- **未確認**（測定不能・理由記録）。cold hygiene 3 条件は測定準備時点で満たした（fresh canonical `hub.exe`（pytest freshness embed test 合格・source 非変更）・fresh server instance・isolated origin）。window size 実測 1678x1025（content viewport 1665x932・人工 resize なし）。
- 不合格理由: Playwright browser が並行 session（N-06）と共用で、tool call 間で私の isolated tab（context `n10-own-assess-20261108`・instance port 8739）が 2 回乗っ取られ、counter install 前に外部 run の search + prefetch（furry・page 2-3 先読み）が私の origin へ書き込んだ。per-run 書入回数測定と before/after（操作前後・同一条件）が成立しない。Drive 保存なし（有効な capture なし）。
- 観察（私の制御下測定ではない・区別記録）: 同一 served app の別 session run 後、storage map が 74 group keys（1 run + 先読み 2 page）＝上界 75 と整合（1 件 duplicate key skip）。cold では save 回数 = 条件該当呼出数（機構どおり）。
- 残件（project root 外は削除せず記録）: Playwright 出力（snapshot yml・console log）が `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\` に残る。scratch dataDir（`docs/ai/tmp/assess-20261108-1332/r10/served_data*`）に server 生成の DB/cache ファイルが残る（gitignore 内）。project root 内の local copy（scratch 内 screenshot）はなし → `kilo-safe-remove` 該当なし。

### 実測（mock）・実測で閉じた点と推測の区別

- 実測確定: 機構・条件クラス・呼出回数・stringify 壁時間・payload 件数・debounce/bounded 比較（上表）。
- 未確認: 実 browser での `setItem` 同期 storage write の壁時間（mock は assignment のみ・実測不能）、served 画面上の per-run 書入回数（上記不合格）、実 data での actual storage 件数（ユーザー実 storage 非汚染のため未読）。

### 推奨（複数案中の採用案・記録のみ）

1. debounce（呼出を run/page 単位で 1 write へ束ねる）: 意味変更なし側として有効（実測 75→1 write）。
2. 件数上限（bounded）: C-05 convention 値 1024 entry（invented number なし）。意味変更側（上限・淘汰順）は承認案件として分離済み。順は R-15 analogy で作成順推奨（実装簡易・payload 増なし）、C-05 analogy（last-used）は alternative。
3. 既存テスト・正典 condition は保持制約（関数名・key・block 境界・setSearchTitleGroup 呼出口）で通過可能（照合済み）。

推定規模: 小〜中（search.html helper 群 約 30–60 行＋新テスト。既存テスト変更なし。UI 意味変更なし部分のみ実装可、bounded 側は承認待ち）。

### 優先順位付けに効く要素

- 重要度: low〜mid（per-page CPU 実測は cold worst 約 64 ms（n=1,000）・steady は sub-ms〜10 ms/呼出。機能リスクは storage 上限超過時の silent persist stop（機構確定・発火は長期使用規模））。
- 緊急度: low（現規模（観察 74 keys）では体感影響なし）。
- リスク: low〜mid（debounce は低。bounded は意味変更の承認要・淘汰順の正典流儀分岐）。
- 依存: `setSearchTitleGroup` の正典 condition、既存テスト pin（関数名・key・block 境界）、C-05 convention 値、R-15 作成順決定、2026-11-02 known 照合（catch 握りつぶし）。単独実施可（debounce 部分）。

## Verification

- node harness（scratch・mock・実ブロック抽出）: `node docs/ai/tmp/assess-20261108-1332/r10/r10_measure.mjs` → 6 checks passed（実行時間約 1 s・stringify 計測は 50 iterations 含む）。
- `python -m pytest tests/ -v` → 59 passed / 1 failed（17.66 s）。failed は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来の既存 failure `docs/ai/known-issues/KI-20261106-04-canonical-build-freshness-pointer-missing.md`・本項目非接触）。既存テスト変更なし。
- `node tests/search_manual_jp_harness.mjs` → 20 checks passed（約 0.1 s）。`node tests/search_prefetch_harness.mjs` → 42 checks passed（約 0.1 s）。
- `go vet .` → clean。`go test -count=1 -timeout 300s .`（app）→ ok 31.797 s（titleGroups pin テスト `TestEmbeddedUIHasSameTitleJapaneseAnchor` 含む全通過・コード非変更）。
- 画面確認: 未確認（served・共用 browser の並行 session 乗っ取りで per-run 書入回数と before/after が成立せず。準備条件（fresh canonical exe・fresh instance・isolated origin・window size 1678x1025/viewport 1665x932）は記録どおり）。Drive 保存なし。local copy（project root 内）該当なし。project root 外残件は上欄。
- 源码変更なし（変更は本記録のみ）。実装は未実施（実装開始指示待ち）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（親の `task_manager` 側）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし — storage 肥大の照合元は `docs/ai/tasks/2026-11-02-external-code-review-f501e2b.md`（task 記録・known-issues ではない））
- [ ] `backlog.md` の完了項目を削除（該当なし — N-10 は claude review findings 記録の項目）

## 2026-11-09 再調査（assess-20261109-1320）

**実行固有ID**: assess-20261109-1320（記録のみ。実装開始指示待ち）

### 有効性（再確認）

**有効（不変・実測で閉じた）**。現行コードに mitigation 未導入: debounce なし・変更時のみ保存（同値 skip）なし。

### 現行コード状態（line 番号は現行 2026-11-09）

- titleGroups 保存経路に debounce 未導入。`readListsDebounceMs = 250`（`app/search.html:181`）と `debounceLists()`（`:699-710`）は `/api/lists` fetch（read/lists 項目）専用で titleGroups と無関係。`saveTitleGroups()`（`:233-237`）は呼出ごとに全量 stringify + `localStorage.setItem`（同期）。
- 「変更があったときだけ保存」: 言語条件 skip（`setSearchTitleGroup` `:239-247`）のみ。同値再設定 skip なし — warm 周回 trip で再設定 25/25 save（harness scenario 3 再確認）。
- 毎カード呼出は実在: `renderSearchCard` 内 `setSearchTitleGroup(r);`（`:412`）、`applyPrefetchCardKnowledge`（`:833`）。補足: manual search 行の呼出口（`:360`）は 5d87f4c（2026-11-06）で既存（前批列挙漏れ — 機構矛盾ではなく列挙の不完全。操作ごとに save、毎カードではない）。
- map 上限なし: 不変（起動時 `loadTitleGroups()` `:1083` のみ）。

### 前批記録との矛盾照合

- 矛盾なし。前批後の commit（b2641df・90a8efc の Type filter 系）で line 番号が +11〜+87 シフトしたが機構同一（diff に titleGroups 触手なし確認）。
- 前批 harness を現行源码（実ブロック抽出）で再実行 → 前批と整合（壁時間は機械分散）: cold 全新規 75 呼出→75 save（上界）、warm 混成 page 25 呼出→15 save（skip 10）、round trip 25/25、n=10,000 median 9.146 ms/呼出・累计 685.9 ms、debounce sim 75→1 write（flush 10.555 ms）、bounded cap 1024 sim（flush 5.709 ms・保持 1,024）— 前批数値と同一機構。
- 意味変更側（件数上限・淘汰順）の分離は維持: 現行コードに bounded・淘汰順・同値 skip 未導入（debounce 側＝意味変更なし・単独実施可、bounded 側＝承認案件のまま）。推奨不変: debounce（map 更新同一順・persist 束ねのみ）→ 採用案。順は R-15 analogy 作成順推奨（前批どおり）。

### 文書矛盾の处理（日付優先）

- `test_canonical_build_freshness_rule_exists` は前批で failure（known issue KI-20261106-04）→ 現行は合格。正典文書同士の状態記述（前批記録=未実施）が現行実測と矛盾 → 日付優先で現行実測（2026-11-09）を採用し、前批の failure 記述を根拠にしない。現行 AGENTS.md pointer（`docs/ai/rules/confirmation-test-rule.md`）と `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md` を適用文書として採用。

### 検証（再実行・記録のみで源码変更なし）

- `node docs/ai/tmp/assess-20261108-1332/r10/r10_measure.mjs` → 6 checks passed（約 1 s）
- `python -m pytest tests/ -v` → 60 passed（10.01 s。freshness test 合格＝canonical `hub.exe` 現行 bytes 埋め込み）
- `go vet .` clean・`go test -count=1 -timeout 300s .`（app）→ ok 25.639 s（titleGroups pin テスト通過・テスト変更不要）

### 画面確認（served）

- **未確認**（前批と同因で再試行不合格）。準備条件は満たした: fresh canonical `hub.exe`（freshness 合格）＋ fresh server instance（port 8747・scratch dataDir `docs/ai/tmp/assess-20261109-1320/served_data`・config/schema コピー。実 DB・完成フォルダ・正典 config 非接触）＋ isolated origin 新規 4 回。
- 不合格理由: 共用 Playwright browser の並行 session（main busy）が新規 isolated context 作成直後の数秒内で外部 run（furry search）を書き込み、4 context すべてで測定 install 前に hash `#/q/furry/page/1`・storage keys（filters/last_search/title_groups）・map 74 が成立。per-run 呼出回数と before/after（操作前後・同一条件）が帰属確立不能。有効な capture なし → Drive 保存なし（`I:\マイドライブ\kilo\` 配下にフォルダ作らず・スクリーンショットなし・local copy 掃除該当なし）。
- 観察（汚染 origin・確認合格でないことを区別・自己帰属分のみ）: 実 browser の単一同期 `setItem` write 0.20 ms（n=74・payload 6,819 B・私が呼出した calls として帰属成立）。実 window size 1679x1025（content inner 1666x932・人工 resize なし）。`--search-card-height: 130px` 契約の card rect 約 140.6 px 観察は不合格条件下のため確認結果として採用しない。
- server 停止済み。残件（project root 外・記録）: Playwright 出力（snapshot yml・console log）が `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`。project root 内（gitignore）残件: `docs/ai/tmp/assess-20261109-1320/served_data/`（cache・hitomi.db・db-shm・db-wal・config.example.json・schema.sql コピー）と前批 diff 転写 2 ファイル。

### 優先順位付けに効く要素（差分）

- 重要度 low〜mid・緊急度 low・リスク low〜mid（前批どおり。機構確定の機能リスク＝storage 上限超過時 silent persist stop）。
- 新要素: 画面確認が共用 browser の乗っ取りで実況確立不能（前批・今回同じ）。実装前の画面テスト合格は共用 browser の占有解消に依存。debounce 部分（意味変更なし）は単独実施可のまま。

推定規模: 小〜中（search.html helper 群 約 30–60 行＋新テスト。既存テスト変更なし）— 前批と不変。

## 2026-11-09 実装委譲（項目#10・実装サブエージェント・assess-20261109-1320 分）

**実行固有ID**: assess-20261109-1320（実装委譲）。合否: **不合格（画面確認未検証・帰属確立不能）→ 項目進行中のまま・作業ツリーは委譲前 HEAD d80ca07 へ復元（実装分は破棄・本記録に残置）**。修正・再検証試行: 実装 1 回（合格条件のうち harness＋正典検証は合格・画面確認のみ不合格）。画面確認試行 3 回（すべて共用 browser の乗っ取り）。

### 採用した判断（推奨案・ユーザー同意済み 2026-11-09）

- debounce（既存 debounce helper 流儀＝window 末 1 write）＋同値 skip（flush 時 payload を last persisted と比較、同値なら write skip）のみ。件数上限・淘汰順（bounded 側）は承認案件分離のまま未実施。`readListsDebounceMs`（/api/lists 正典意味）は流用せず、localStorage 側は別実装（`titleGroupsPersistDebounceMs = 250` — 既存 debounce convention 250 ms を新定数として採用・流用ではない）。
- `loadTitleGroups()` は保存済み raw 文字列を同値 skip の baseline として seeds（解析済みオブジェクトではなく raw 文字列を保持）。

### 実装内容（破棄済・再実行用に要点）

`app/search.html`（persist helper 群・manualBlock/patchHelperBlock/schedulerBlock 境界外）:

```js
const titleGroupsPersistDebounceMs = 250;
let titleGroupsPersistTimer = null;
let lastTitleGroupsPersisted = null;

function loadTitleGroups() { /* raw 文字列を lastTitleGroupsPersisted へ seeds */ }
function persistTitleGroups() { // payload = JSON.stringify(Object.fromEntries(titleGroups));
  // payload === lastTitleGroupsPersisted なら skip。setItem 成功時のみ baseline 更新（catch 握りつぶし挙動維持）
}
function flushTitleGroups() { titleGroupsPersistTimer = null; persistTitleGroups(); }
function saveTitleGroups() { // clearTimeout→reschedule（既存流儀）
  titleGroupsPersistTimer = setTimeout(flushTitleGroups, titleGroupsPersistDebounceMs);
}
```

- `setSearchTitleGroup` の group 決定条件・呼出点は不変（呼出ごとに map 更新同一順・persist 束ねのみ）。
- named callback 採用理由: anonymous arrow の `setTimeout(() => {` 文字列 pin を read_lists_debounce_harness の lists helper 群順序判定（clearTimeout < setTimeout）より前に置かないため（試行中に literal 衝突を 1 回検出し、テスト側変更なしで named callback 流儀へ統一して解決）。
- 合格条件が必要とするテスト更新（`app/viewer_test.go` pin）は不要だった（全 pin 保持・既存テスト全通過）。テスト追加のみ: `tests/title_groups_persist_debounce_harness.mjs`（正典流儀・mock・外部 API なし・20 checks）＋ `tests/test_title_groups_persist_debounce.py`（pytest 統合）。両ファイルは不合格により `tests/` から除去し、再実行用のコピーを `docs/ai/tmp/n10-impl-20261109/`（gitignore scratch）に残置。

### 検証（実施結果）

- `node tests/title_groups_persist_debounce_harness.mjs` → 20 checks passed（約 1 s）。決定論動作: cold 75 呼出→window 前 write 0・window 末 write 1（payload 75 件）・warm round trip 25 schedule→同値 skip（write 0）・load baseline seeds・condition classes（schedule 15・skip 10）保持。
- 改善数値再実測（n=10,000 warm・mock）: sync 75 write 累计 **447.3 ms** → debounce 1 write flush **5.2 ms**（run により 596.9→5.6 / 434.2→5.6 / 447.3→5.2。前批 747→11.1 と同一機構・機械分散・bounds >300/<100 合格）。実測で閉じた（壁時間区別: mock の setItem は assignment のみ）。
- 正典検証: `go vet .` clean・`go test -count=1 -timeout 300s .`（app）→ ok 26.691 s（`TestEmbeddedUIHasSameTitleJapaneseAnchor` pin 全保持）。`python -m pytest tests/ -v` → **65 passed**（25.18 s・freshness test 含む・既存テスト変更なし）。既存 harness: read_lists_debounce 29 checks・manual_jp 20・prefetch 42 → 全通過。

### 画面確認（served）— 不合格（帰属確立不能・3 試行）

- 準備条件は満たした: fresh canonical `hub.exe`（現 app 最終状態から build・freshness test 合格）＋ fresh server instance（pid 14720・port 8753・scratch dataDir `docs/ai/tmp/n10-impl-20261109/served_data`（schema.sql・config.example.json コピー。実 DB・正典 config・完成フォルダ非接触）＋ isolated origin（新 context 3 個）。window size 実測 inner 1666x932（人工 resize なし・自然）。
- 試行1（context `n10-impl-20261109`）: navigate 後即 evaluate で hash `#/q/furry/page/1`・storage keys（filters/last_search/title_groups）・groups 74・status「検索完了・カード表示済み 25件」= 測定 install 前に外部 run が my tab を乗っ取り search 完了。
- 試行2（context `n10-own-impl-20261109`）: navigate 呼び出し内で hash `#/q/furry/page/1` 出現（即汚染）。
- 試行3（context `n10-own-impl-t3`）: navigate 呼び出し内で hash `#/q/furry/page/1`、直後 evaluate で storage keys＋groups 74（帰属確立不能）。前批（assess-20261108-1332・assess-20261109-1320 調査）と同因: 共用 Playwright browser の並行 session（main busy）が my isolated context/tab を数秒内で乗っ取る。per-run 書入回数・before/after（操作前後・同一条件）が成立せず → capture 不合格。
- 合格条件の served 部分（保存結果＝title groups 表示の正しさ）は**未検証**。Drive 保存なし（`I:\マイドライブ\kilo\` 配下へフォルダ作らず・スクリーンショットなし）。local copy（project root 内 scratch）該当なし → `kilo-safe-remove` 該当なし。server 停止済み（pid 14720）。自分の tab 閉鎖済み（残 tab は共用 browser の他 session 分・非接触）。
- 残件（gitignore・project root 内 scratch）: `docs/ai/tmp/n10-impl-20261109/`（served_data（server 生成 DB/cache あり・scratch 内・実 DB 非接触）＋ harness/test の再実行用コピー＋ debug_scoping.mjs）。

### 作業ツリーの扱い（不合格時の手順）

- `app/search.html` を `git restore` で d80ca07 へ復元。`tests/` の追加 2 ファイルは正典 `kilo-safe-remove` でゴミ箱へ送り、scratch へコピーを残置（再実行用）。委譲前状態（clean HEAD d80ca07）へ復元し、本記録のみ追記コミット（record only）。
- 影響疑い（步骤6）: 実装分は pin 全保持・既存 suite 全緑で確認（破棄済み）。破棄した実装分は他項目の合格済み結果へ影響なし。read_lists_debounce_harness は非変更（literal 流儀の衝突は app 側 named callback で解決）。

### 次の対応（parent）

- 共用 browser の占有解消後、同一合格条件で served 画面確認を再実行。実装 plan（上）と harness（scratch コピー）そのまま再適用可。実装→画面確認合格→正典検証→`task_manager end` の順。
- Status/Next Action は正典 task_manager で更新（本項目は不合格のまま進行中）。

コミット: record only（本項目分）。実装・テスト分は不合格によりコミットしない。

## 2026-11-10 実装委譲再実行（項目#10・実装サブエージェント・assess-20261109-1320 分の再実行）

**合否: 合格（画面確認含む・帰属確立）・項目終了**。修正・再検証試行: 3 回（画面確認試行として。内訳下記）。

### 採用した判断（推奨案・ユーザー同意済み 2026-11-09・記録の実装 plan どおり）

- `app/search.html` persist helper 群（manualBlock/patchHelperBlock/schedulerBlock 境界外）に `titleGroupsPersistDebounceMs = 250`（新定数・`readListsDebounceMs` は流用不変）＋ `lastTitleGroupsPersisted`（raw baseline・`loadTitleGroups()` が raw 文字列で seeds）＋ `persistTitleGroups()`（同値 skip・setItem 成功時のみ baseline 更新・catch 握りつぶし挙動維持）＋ `flushTitleGroups()` ＋ `saveTitleGroups()`（clearTimeout→reschedule 既存 debounce helper 流儀・named callback）。
- `setSearchTitleGroup` の group 決定条件・呼出口・key・保存内容・データ構造・UI 挙動は不変（persist 束ねのみ）。件数上限・淘汰順（bounded 側）は承認案件のまま未実施。
- named callback 流儀維持（前回の literal 衝突解決どおり。read_lists_debounce_harness の `clearTimeout < setTimeout(() => {` 順序判定と衝突しない）。既存 pin 全保持（テスト変更不要）。
- harness 2 ファイルは scratch 残置コピー（`docs/ai/tmp/n10-impl-20261109/`）を `tests/` へ再現（正典流儀・mock・外部 API なし）。

### 実測（harness・再実測）

- `node tests/title_groups_persist_debounce_harness.mjs` → **20 checks passed**（約1 s）。改善数値: sync 75 write 累计 **435.2 ms** → debounce 1 write flush **5.4 ms**（機械分散・前回 447.3→5.2・前批 747→11.1 と同一機構・bounds >300/<100 合格）。決定論動作: cold 75→window 末 1 write・warm round trip 同値 skip（write 0）・load raw seeds・condition classes（schedule 15・skip 10）保持。

### 正典検証（合格）

- `go vet .` clean・`go test -count=1 -timeout 300s .`（app）→ ok 36.240 s（`TestEmbeddedUIHasSameTitleJapaneseAnchor` pin 全保持）。
- 正典 build fresh（`go build -o ...output/hitomi-hub/hub.exe .`）。`python -m pytest tests/ -v` → **66 passed**（27.39 s・freshness test 含む）。既存 harness: read_lists_debounce 29・manual_jp 20・prefetch 42 → 全通過。

### served 画面確認（合格・帰属確立）

- 準備: fresh canonical `hub.exe`（実装後 build・freshness 合格・同一 exe から served）＋ fresh server instance（pid 23212・port 8761・scratch dataDir `docs/ai/tmp/n10-impl-20261110/served_data`・schema/config コピー。実 DB・正典 config・完成フォルダ非接触）＋ isolated origin（context `n10-impl-20261110`・自 tab のみ）＋ window size 自然 inner 1666x932（outer 1679x1025・人工 resize なし・card 切り取りなし）。
- 試行1: isolated origin 作成直後に測定 install 前へ state（hash `#/q/furry/page/1`・3 keys・74 groups）出現 → 帰属確立不能（不合格）。**帰属訂正（本実行・実測で閉じた）**: このパターンは app 自身の起動時挙動で説明できる — 起動時 `runSearch()`（`app/search.html` 1122-1123）＋ `#query` 既定値 `furry`（`:105`）＋ `history.replaceState`（`:837`）が同一 origin へ hash/storage を書く。install（write/呼出 counter）を起動 run の write 前に入れ、clean を確認すれば帰属確立可能（前批記録の「共用 browser 乗っ取り」帰属は本実行では成立せず。前批記録は書き換えず本実行の区別記録のみ）。
- 試行2: clean start 確認済みだが clean 判定の誤検知（patch した `setItem` instance property が `Object.keys(localStorage)` に混入）で測定前に exit（不合格）。
- 試行3（合格）: 単一 tool call 内で marker 確認 → storage clear → install（呼出/書入 counter）→ clean 確認（install 時 hash ""・keys 空）→ 測定。帰属確立（測定チェーン同一文書・call 内外部変更なし）。
  - schedules（`saveTitleGroups` 呼出）累计 286 → title_groups key 実 write 14（window 束ね。content 変更 window のみ write）。同値 skip served 実証: 同一 content の再 search/page 移動 flush で title_groups write 追加なし（last_search/filters のみ）。cold page 1（呼出 25）は fetch wave ごとの window write（1349/1909/3128 ms）。
  - storage 内容 == 画面 map（storageEqualsMap true・groups 121・japanese 52）。pages 1-4（100 cards）で表示↔storage 照合 mismatches 0。
  - positive link 表示: page 4 English カード「Twitter Archive」に「同名の日本語版: #4298714」link（href `/#/g/4298714` == storage bestId 4298714）。
  - negative 観察（区別）: page 2 manual click（操作）で該当 term の日本語版見つからず link 化なし。
  - 保存結果正しさ: storageEqualsMap 全測定点 true・同値 skip・window 束ね（write ≤ window 数 < schedules）。

### Drive 保存（フォルダ名＝項目 slug・規則準守）

- `I:\マイドライブ\kilo\2026-11-10-hitomi-hub\` へ confirmation txt + 10 png（before/after 各組): `n10-t3-cold-before-20261110.png` / `n10-t3-cold-after-20261110.png` / `n10-t3-page2-before-20261110.png` / `n10-t3-page2-after-20261110.png` / `n10-t3-manualclick-before-20261110.png`（negative 観察）/ `n10-t3-page3-before-20261110.png` / `n10-t3-page3-after-20261110.png` / `n10-t3-page4-before-20261110.png` / `n10-t3-page4-after-20261110.png` / `n10-t3-link4298714-card-20261110.png`（positive link card 被写体）。
- local copy cleanup: project root 内の `n10-t3-link4298714-card-20261110.png` は正典 `kilo-safe-remove` でゴミ箱へ送った（Drive 側保持）。Playwright 出力 png（project root 外 `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`）は正典削除の承認範囲外 → 残存＝追加承認要（正典条の例外として記録）。

### ブラウザ

- 自 isolated tab（`n10-impl-20261110`）を閉じた（閉じた事実は結果報告時にも維持）。server（pid 23212）停止済み。

### commit（本項目分のみ）

- `app/search.html`（persist helper 群）・`tests/title_groups_persist_debounce_harness.mjs`（新規）・`tests/test_title_groups_persist_debounce.py`（新規）・本タスク記録追記・`current-task.tsv` 本項目参照行（`task_manager end`）。commit・push は commit message 記載（コミット後ハッシュは結果報告に残す）。
