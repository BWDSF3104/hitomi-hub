# R-10: ページ送りのたびに searchAllIDs の全件 intersection を再計算

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。案: (query, lang) 単位で最終結果を短 TTL キャッシュし page移動・prefetch・firstStatus で共有。既存 term 単位 searchCache は維持。記録: R-10

## 意図 / 背景

- 外部指摘（Claude）R-10 の読み取り確認（findings R-10 欄）を受け、現行コードの機構照合と実測（mock・外部 API 呼出しなし）で有効性を確定する。
- 現行 cache は term 単位の dataset（`searchCache`）だけ。page 移動で call ごとに copy + map 再計算が発生（`app/searchhtml.go:373-458`）。(query, lang) 単位で最終結果を短 TTL cache する案の実現可能性、cache key の order 扱い（D-09）、firstStatus との共有可否、既存テストの固定、freshness 正典との照合を記録する。
- 本調査は read-only（コード変更なし。変更したのは本タスク記録とスクラッチ `docs/ai/tmp/assess-20261106-1056/` のみ）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- cache は term 単位の dataset のみ。key は area/tag/language 基準で、order dataset を選ぶ場合のみ key に order を含む（`app/searchhtml.go:116-119`・D-09）。entry は `searchCache`（`app/hitomi.go:119-126`、bounded LRU・C-05/A-14）。
- `searchAllIDs` は call ごとに: term cache hit でも dataset copy（`copyInts`・`app/searchhtml.go:85-89`、hit 返却 copy `app/searchhtml.go:131`）→ positive/exclude の `map[int]bool` 構築 + filter（`app/searchhtml.go:185-211`）、or group union（`app/searchhtml.go:213-225`）を毎回再実行。page slice は末尾（`app/searchhtml.go:252-269`、`app/searchhtml.go:461-468`）。
- `firstStatus` は `searchAllIDs` を呼ぶ（`app/firststatus.go:118`）ため、最終結果 cache を `searchAllIDs` 内へ置けば page 移動・UI prefetch・firstStatus が自動的に共有する。UI page 移動は call ごとに `/api/search` 経由（UI の `localStorage` ids 保存は reload restore 用で page 移動の server 再計算は回避できない・decision `2026-11-01-ui-render-generation-and-search-persistence.md`）。

### 実測（mock・外部 API 呼出しなし・既存 mock fetch 流儀）

- 環境: 現マシン・`go1.27.0 windows/amd64`。スクリプト `docs/ai/tmp/assess-20261106-1056/main.go`（正典関数を verbatim copy・term cache warm 状態・gate/network なし・CPU 時間のみ）。
- dataset 規模（30 万件級): base 300,000 ids。positive term datasets 実生成 150,000 / 30,000 / 10,000。or group datasets 10,000 / 4,054。negative 5,000。
- page 移動 10 回（10 call）を測り、page あたり:

| 経路 | page あたり CPU |
|---|---|
| 現行（call ごとに copy+map 再計算・multi-term） | 25.11 ms（10 call 251.1 ms。另測 27.93 ms） |
| 現行（single term 300k・final 150k） | 20.27 ms（10 call 202.7 ms） |
| R-10 cache hit（copy+slice・final 150k） | 0.100 ms |
| R-10 cache hit（copy+slice・final 5,000） | <0.001 ms（測定下限） |

- cold 1 回目: 28.6–51.2 ms（run により差）。
- 結論: 30 万件級で page あたり 20–28 ms の CPU が R-10 で page あたり ≤0.1 ms（hit 経路の copy+slice）。改善は warm path（初回 compute 後）限定。network 呼出しは増減なし（term cache が既に cover）。体感の支配要因は network/gate（100 ms 級・`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md`）で、R-10 は CPU 側の効果。

### 有効性

有効（実測で閉じた）。指摘の機構（term 単位 dataset cache・call ごと copy+map 再計算）は現コードどおり。page 移動 10 回分の実測で page あたり 20–28 ms（30 万件級）を確定。cache 案の hit path は ≤0.1 ms/page。cold・単発検索には効果なし（初回 compute は同じ）。

### cache key の order（D-09 照合）

- D-09 の dataset 選択は既存 term cache key に order を含むことで解消済み（`app/searchhtml.go:116-119`）。これは変更不要。
- R-10 の結果 cache key: order term は query 文字列の一部なので、key を「normalized query + resolved language」にすれば order は含まれる。`buildSearchState` の normalized term（`app/searchhtml.go:31-33`・`app/searchhtml.go:290-315`）と resolved language（pre-scan `app/searchhtml.go:364-371`）を key に使うこと。raw lang ではなく resolved lang を使う（`language:` term の dataset 選択と一致させる）。
- `orderby`≡`sortby` の綴り違い・`orderbykey` の既定確定（`app/searchhtml.go:93-107`・`app/searchhtml.go:324-330`）は同一結果の key 差異（cache miss）になり得るが正しさには影響なし。厳密化するなら resolved state の serialization を key にする案（推奨: normalized query + resolved lang・実装時に決定）。
- `background` は key に入れてはいけない。fetch lane であって結果内容ではない。page/prefetch/firstStatus 共有の条件（firstStatus は foreground・`app/firststatus.go:118`・UI prefetch は background）に反する。
- random order（`orderbydir:random`・`applySearchOrder` の shuffle `app/searchhtml.go:247-249`）は「page が呼出ごとに変わる」正典（`docs/ai/decisions/2026-11-05-jump-first-status-page-random-order-disabled.md`）と衝突するため、結果 cache から除外する（hit を freeze しない）。推奨: random のとき cache しない・key にしない。firstStatus は random を error で弾く（`app/firststatus.go:115-117`）ので影響なし。cache する案はユーザーに見える意味（UX）の変更・正典 decision 変更を要る。

### firstStatus との共有

共有可。`firstStatus` は `buildSearchState` + `searchAllIDs` を同一 query/lang で呼ぶ（`app/firststatus.go:106-121`）。`searchAllIDs` 内 cache なら page 移動（`SearchHTML`）・prefetch・firstStatus が単一路径で共有する。既存固定テスト `TestFirstStatusSharedSearchCacheNoRefetch`（`app/firststatus_test.go:260-300`）は network fetch 計上のみで、結果 cache hit は fetch を増えない（通過維持）。

### 既存テストの固定照合

- page 結果 cache の意味を固定している既存テストはない（照合済み）。`app/searchhtml_test.go` で同一 query の 2 度呼出（page 移動相当）を持つテストはなし（`big_breasts`/`big breasts` は別 query）。反復呼出テストは term cache 層のみ: `TestSearchNozomiPartsUsesTTLCacheAndCopies`（`app/searchhtml_test.go:28-65`）、`TestSearchCacheEvictsLRUEntriesWhenOverEntryLimit`（`app/searchhtml_test.go:67-143`）。
- 維持すべき固定（新 cache にも同じ流儀を Extension する): hit は copy 返却（cache が caller slice mutation で壊れない・`app/searchhtml_test.go:57-64`）。`applySearchOrder` は hit 時に再適用しない（cache に最終順序済みで保存。hit で asc reverse を再適用すると結果が壊れる・`app/searchhtml_test.go:594` 二重 reverse 禁止）。
- 実装時は hit/cold・key（normalized・resolved lang・random 除外・background なし）を固定する新規テストが必要（既存テストは変更しない）。

### freshness 正典照合（A-14・D-13）

- 実測訂正（2026-11-06 13:17・指示方法どおり read + glob）: `docs/ai/tasks/2026-11-03-a14-search-freshness-checks-cache.md`・`docs/ai/tasks/2026-11-03-d13-search-freshness-decision.md` は read で File not found、Glob `docs/ai/tasks/2026-11-03-a14*`・`docs/ai/tasks/2026-11-03-d13*` で 0 件（実体なし）。git 履歴にもなし。この実体なしを測定根拠として記録する。A-14 の実記録は `docs/ai/tasks/2026-11-03-c05-cache-limits.md`（A-14/C-05 統合先・`docs/ai/tasks/2026-11-03-claude-review-349778d-assessment.md` の ID 表 48・65 行）で、D-13 は同表に項目なし（D-01〜D-10）。正典同士の日付優先ルール適用: 壊れた pointer や古い文書の状態記述を根拠にせず、古い方の状態記述には寄らず、実在する正典（2026-11-06 findings R-10 欄、2026-11-03 c05-cache-limits・`docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md` §8、2026-11-05 decision）を採用した。
- 採用正典: (1) C-05/A-14 設計（cache = TTL + bounded LRU・hit で access order・insert 後 cap 超過で stale eviction・`docs/ai/tasks/2026-11-03-c05-cache-limits.md`、`app/hitomi.go:92-107`）。(2) 検索 cache は TTL cache（`searchCacheTTL`/`topCacheTTL`/`indexVersionTTL` = 1h・`app/hitomi.go:58-61`、TTL hit・eviction 後 refetch はテスト固定済み）。permanent cache は galleryinfo の Phase 3 層のみ例外（meta 不変前提・decision `2026-11-05-phase3-gallery-info-persistent-cache-file-store.md`）。(3) 正典 §8（`docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md` 107・113 行）は既に「full ids を保持し page slice は cache から取る」「ページ移動で同 query/lang の cache hit」を合格条件化。R-10 はこれの最終結果層への延長で、正典と同一方向。
- 衝突判定: 衝突なし。R-10 の「短 TTL・(query, lang)・既存 searchCache 維持」は TTL cache 意味を弱めない。結果 cache の staleness は最大で underlying term dataset の staleness（TTL 1h 正典）までで、R-10 の短 TTL はこれを超えない（超える設計は不要）。freshness check の cache への降格（call ごとに remote 確認せず TTL hit で返す）は現機構どおりで、新層はその TTL を短くするだけ。TTL の具体値は実装 choice（推奨: 正典 convention 値から選ぶ・invented number 禁止。既存 convention に 5 分（`negativeNoFullCacheTTL`・`app/hitomi.go:22`）と 1h（`searchCacheTTL`）があり、短 TTL は 5 分 convention が妥当）。
- 正典ルール変更が絡む唯一の論点: random order を cache するか否か。除外（推奨）なら正典 `2026-11-05-jump-first-status-page-random-order-disabled` の「呼出ごとに変わる」意味を保持でき、変更不要。cache 側に含める案は UX 意味変更（TTL 内で page が固定される）で承認が必要 → 判断保留該当はこの案のみ。

### 副作用・影響範囲・代替案

- memory: 結果 cache entry は最大 300,000 ids（2.4 MiB・entrySize = ids*8 convention・`app/hitomi.go:125`）。C-05/A-14 流儀で bounded（entry 数 + bytes cap）が必須。単 entry が cap 超なら cache しない（既存 convention・`docs/ai/tasks/2026-11-03-c05-cache-limits.md` 19 行）。cap 値は実装時に既存 convention から選定（invented number 禁止）。
- 依存（R-01）: `boundedCachePut` は上書き計上 bug（`docs/ai/tasks/2026-11-06-r01-bounded-cache-put-overwrite-double-count.md`）。新 cache が同一 helper を使うと継承する。R-01 先行修正を推奨（小規模・確定・helper 1 箇所で 5 cache 網羅）。
- gate・rate・politeness・TN・full image rules・API は非接触（cache hit は network を増減しない）。firstStatus の read-only・DB 書込み条件は不変。
- 代替案: A（推奨）= 上記の bounded・短 TTL・random 除外。B = UI 側のみ（localStorage ids 保存は既存・reload restore 用で page 移動の server 再計算と firstStatus/prefetch を cover できず不足）。C = 追加しない（page あたり 20–28 ms CPU・機能影響なし・CPU 側の体感優先度が低いなら降格可）。

### 推定規模・優先順位付けに効く要素

- 推定規模: 小〜中（`app/searchhtml.go` に cache 1 層 + key 構築 + bounded helper 流儀 + 新規固定テスト。UI・gate・schema 非接触）。
- 重要度: 中。page 移動・prefetch・firstStatus の warm path で効く。cold・単発検索には効かない。network latency（100 ms 級 gate）が支配で、CPU 改善は副次（体感改善の主因は R-11 側の逐次 fetch・worker 定数）。
- 緊急度: 低（機能停止・正誤への影響なし）。
- リスク: 低〜中。新 cache の bounded・copy 不変条件・hit で order 再適用禁止・random 除外・resolved lang key をテストで固定すれば決定的。R-01 accounting への依存。
- 依存: R-01（helper 修正先行が望ましい）。既存 term cache（searchCache）維持前提。R-11（term 逐次取得）とは独立・併存可。

### 実装＋固定テスト（2026-11-08・実作業）

- 採用判断（段階1・記録の推奨案）: 案 A（bounded・短 TTL・random 除外）。持帰りはなし。
  - TTL: `searchResultCacheTTL = negativeNoFullCacheTTL`（正典 convention 5 分・invented number なし）
  - key: normalized terms + resolved language（`searchResultCacheKey`）。separator は `\n`（term boundary 保存）。`\n` は whitespace で、`strings.Fields` 由来の parts（term・key lang）には入らないため content 衝突なし一意。space 区切りは `big_breasts`（1 term・space 形）と `big breasts`（2 term）を衝突させ、既存固定テスト `TestSearchHTMLPlainTermUnderscoreUsesSpaceFormKeyPerCanonicalHitomi` の FAIL で検出（試行1）→ 原因分析: term boundary 非保存 → `\n` 区切りへ修正 → 再検証合格（試行2）。key に background を入れない（fetch lane）。random order（`orderbydir:random`）は cache しない（key を作らない・decision 2026-11-05-jump-first-status-page-random-order-disabled の「呼出ごとに変わる」意味を保持・正典変更なし）。
  - bounded: C-05/A-14 流儀（entry 数 + bytes cap・hit で access order・insert 後 cap 超過で最古 access eviction・単 entry cap 超は即 eviction）。cap 既定は既存 searchCache convention 値（256/8 MiB・invented number なし・`putSearchResultCacheLocked`）。R-01 の `boundedCachePut`（`ef7631d`）をそのまま使用（R-01 変更に非接触。TTL 切れ key の再 store は上書き経路 → R-01 accounting の適用範囲内）。
  - hit は copy 返却（copy 不変条件）。hit で order 再適用なし（cache に最終順序済みで保存・二重 reverse 禁止）。単 entry の空結果は既存の early return（cache しない）を維持（挙動変更なし）。
- 変更ファイル: `app/hitomi.go`（TTL 定数・entry 型・Client フィールド・put helper）、`app/searchhtml.go`（`searchResultCacheKey`・`searchAllIDs` 内 hit/cold 層。単一路径なので page 移動・prefetch・firstStatus が共有）、`app/searchhtml_test.go`（新規 7 件のみ・既存テスト無変更）。UI・gate・schema・viewer・download 非接触。
- 新規テスト: `TestSearchResultCacheSharedAcrossPageCallsAndReturnsCopy`（page 移動共有・hit 経路実証・copy 不変条件）、`TestSearchResultCacheKeyNormalizedQueryAndResolvedLanguage`（正規化・resolved lang・term boundary）、`TestSearchResultCacheExcludesRandomOrder`、`TestSearchResultCacheKeyIgnoresBackgroundLane`、`TestSearchResultCacheDoesNotReApplyOrderOnHit`、`TestSearchResultCacheEvictsLRUEntriesWhenOverEntryLimit`（bounded・LRU・memory cap）、`TestSearchResultCacheExpiresAfterShortTTL`（TTL=5 分 convention・超過後再計算）。
- 試行1: 実装後 `go vet` clean だが `TestSearchHTMLPlainTermUnderscoreUsesSpaceFormKeyPerCanonicalHitomi` FAIL（key の space 区切り衝突・上記）。試行2: separator 修正 → 全通過。試行回数 2。

## Verification

- 実測コマンド（本項目・実装後): `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean、test ok `24.781s`（既存＋新規全通過）。新規 7 件個別（`-run "TestSearchResultCache" -v`）→ 7/7 PASS。安定化確認: 新規＋既存固定（`TestSearchNozomiPartsUsesTTLCacheAndCopies`・`TestFirstStatusSharedSearchCacheNoRefetch`・`TestSearchCacheEvictsLRUEntriesWhenOverEntryLimit`・`TestApplySearchOrderDoesNotDoubleReverseAsc`）を `-count=2` → 全 PASS（LRU の clock tie は既存流儀どおり lastAccess 明示オフセットで決定化）。経過 1.345s。
- `python -m pytest tests/ -v`（ルート）→ 54 passed・1 failed `9.96s`。failed は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md の Rules 条が commit `e018c1c` で `docs/ai/rules/confirmation-test-rule.md` へ集約され、テストが照合する literal `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` が AGENTS.md から消えている）。本項目のコード変更・テスト変更はなし（既存状態の失敗・R-10 と非接触）。known-issues 化处理は本項目スコープ外（本記録のみ編集可のため、親へ board INFO で報告）。
- 実装後 `python -m pytest tests/ -v`（ルート）→ 56 passed・1 failed `12.26s`（2026-11-08）。同一の既存 known failure `test_canonical_build_freshness_rule_exists`（`KI-20261106-04`・本項目の不合格ではない・Python 非変更）。passed が 54→56 は他項目（R-12 等）のテスト追加によるベースライン増加。本項目の不合格点なし。
- 実測スクリプト: `docs/ai/tmp/assess-20261106-1056/main.go`（mock dataset・外部 API 呼出しなし・正典関数 verbatim copy）。結果は「実測」欄の表（page あたり 20–28 ms → hit ≤0.1 ms）。
- 正典 pointer 実測（2026-11-06 13:17・訂正）: 指示の 2 path を read（File not found）＋ Glob `docs/ai/tasks/2026-11-03-a14*`・`docs/ai/tasks/2026-11-03-d13*`（0 件）。実体なしを確定。照合内容（freshness check が cache へ降格＝TTL hit・per-call 確認なし、top cache は TTL、page slice は cache から取る）は実在正典側（c05-cache-limits・decision §8 107 行・`app/searchindex.go:236-245` の top cache TTL）で確認済み（上記「正典照合」欄）。
- 画面確認: 該当なし（server 側 cache 項目・UI 変更なし）。Google Drive 保存なし。
- `git status --short`: 調査段階は本タスク記録のみ。実装段階の変更は `app/hitomi.go`・`app/searchhtml.go`・`app/searchhtml_test.go`（新規テスト）と本タスク記録（UI 無変更）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（親が `task_manager` で管理）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
