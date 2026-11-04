# Remote Task Phase 1 — 検索カードの可視優先順序付け（調査・評価）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 次段階（実装）へ引き継ぎ: 優先1・有効性「有効」（実測確定）。推奨=IO標準+takeNext優先選択（実行中昇格）。実装はリモートタスク発行済み（承認）

## 意図 / 背景

- 実行固有ID: `assess-20261105-0006`（開始日時 2026-11-05 00:06 JST）
- 対象項目一覧（1件）:
  - 項目ID: `2026-11-05-hitomi-hub-search-phase1-visible-priority`（リモートタスク「検索先読み Phase 1（残）— 検索カードの可視優先順序付け」）
  - 個別タスク記録のパス: `docs/ai/tasks/2026-11-05-assess-20261105-0006-phase1-visible-priority.md`
- 手順: コマンドファイル `assess-items.md`（`C:\Users\fujim\.config\kilo\command\assess-items.md`）の本文が正本。親は登録・統括・優先順位付けのみ、調査実務は子へ委譲。ソースコードは変更しない。
- 開始時点の既存未コミット変更: なし（`git status --porcelain` 空、HEAD `7160dae`）。リモートタスクの基準 HEAD `7160dae` と一致。
- 重複確認: `current-task.tsv` は空（進行中タスク 0 件）。既存タスク記録に可視優先順序付けの進行中項目なし。
- 受領: 対象タスクファイルは Drive から正典受領フォルダ `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\` へ移動済み（Phase 2〜4 は未受領のため Drive 側に残す）。

## 調査結果 / 変更内容

### 棚卸し（作業開始時点）

- HEAD: `7160dae`（リモートタスク基準 HEAD と一致）。
- `git status`: 既存未コミット変更なし。親の登録分のみ（`current-task.tsv` staged、本タスク記録）。開始時点の既存変更は成果物に含めない。
- 重複確認: `current-task.tsv` は本項目 1 件（進行中）のみ。可視優先順序付けの他の進行中タスクなし。タスク登録は正典 `task_manager.py add` 済み（親実行。記録 ID = 本ファイル、実行固有ID `assess-20261105-0006`、項目ID `2026-11-05-hitomi-hub-search-phase1-visible-priority`）。

### 項目の主張の照合（実コード優先）

- 主張「現行の `loadSearchCards()` は ID 順の FIFO キュー」→ **確認**。
  - `app/search.html:295` `const queue = ids.slice();`、`app/search.html:326` `queue.shift()`。可視優先なし。リポジトリ全体 grep で `IntersectionObserver` 0 件（可視判定機構は存在しない）。
  - 精确化: 厳密には「ID 順」ではなく server 応答 `ids` 配列順（nozomi 検索結果順）。page 1 実測で末尾 `4300110 → 4300111` の降順 swap を確認 → 順序テストの照合基準は「ids 配列順（queue 順）」と書くべき。実務上は ID 順とほぼ等価（主張は有効）。

### 実測（正典 server・fresh tab・resize 使用前・2026-11-05 00:13〜00:17 JST）

- server direct baseline 先に確認: canonical server（port 8731）`GET /search` → 200。canonical build freshness test pass（正典 exe が現行 2 ページを embed、正典 build 実機確認可能状態）。
- page 1（furry）: card 発行順が server 応答 ids 配列順と完全一致（FIFO 実証。`docs/ai/tmp` 不要、network requests 照合）。
- page 2（fresh tab）: 発行順 = ids 順完全一致。`#results` card 数 25 == `ids.length`、duplicate `id="query"` 1 個、perf-status 記録あり（cards 25・cache miss=25）。
- page 3（fresh tab・実行中昇格欠如の実測）: 9/25 処理時点でリスト最下部へスクロールし後方 6 カード（4296035, 4295944, 4295923, 4295911, 4295779, 4295770）を可視化 → その発行（req #41〜#51）は、可視化時点で未処理だった可視外カード（4296145…4296045、req #30〜#40）より後。実行中昇格なしを实測で確定。
- 画面キャプチャ保存先（正典手順: 2026-11-02 decision 2 件。タスク別フォルダ条（新しい側）を優先採用）: `I:\マイドライブ\kilo\2026-11-05-assess-20261105-0006-phase1-visible-priority\phase1-visible-priority-current-fifo-baseline-20261105.png`（page 3 完走後・perf-status 込み）＋手順テキスト `phase1-visible-priority-current-fifo-baseline-confirmation-20261105.txt`。

### 有効性

**有効**。主張・前提（Phase 1 残件＝順序のみ、worker pool 化済み `c77f92a`、`currentPriorityDone` 待ち解消済み `727d9fb`、TN-only cover）はすべて現行コードと一致。実測で閉じた（判断保留なし）。

### 対応の実現可能性と推奨修正方法

- 推奨: IntersectionObserver 標準方式（placeholder `<li>` を観測）。worker の pop を `queue.shift()` から優先選択（`takeNext()`）へ変更: ①現在可視 ②可視域近い（IO entry 順＝距離）③残りを ids 順。可視集合（Set）+ pending 配列で実装、n=25 以下なのでヒープ不要（O(n) スキャン十分）。
- 実行中昇格: IO entry でのマーク＋worker の次回 pop 選択で達成。初始ソートは IO の初回 callback（observe 直後に対象全部へ発火）または同期 `getBoundingClientRect` 走査。node ハーネスで実 IO がないため、テストは stub IO（可視集合注入）で同じ `takeNext()` 選択ロジックを検証 — 既存 node ハーネス方式（`tests/harness_lib.mjs` + mock document + `new Function` 抽出実行、`read_lists_debounce_harness.mjs` の先例）と等価にテスト可能。
- 維持条件（すべて現状維持で両立可）: `searchGeneration` guard、perf 指標、TN-only cover、`restoreSearchState()` 経由の復元（同じ `loadSearchCards` を通る）。worker 数 `SEARCH_CARD_CONCURRENCY = 4` 変更なし。ゲート（data 1000ms / image 250ms）変更なし・総リクエスト数不変（順序変更は request 数を増やさない）・ポーリング禁止は IO callback 方式で遵守。

### 副作用・影響範囲・代替案

- `app/viewer_test.go` の関数本文文字列アサート（1678、2129-2135、2362-2401、2857-2878、3013-3019）は `loadSearchCards` 本文の断片（bounded workers、`Math.min(SEARCH_CARD_CONCURRENCY, ids.length)`、per-card timing、cover helper 呼出）を要求。worker 構造・guard・timing を維持すれば壊れないが、実装形態（`shift` 削除）により 1〜3 件 re-anchor の可能性あり。実装時に該当 assertion を確認すること。
- 代替案（非推奨）: `getBoundingClientRect` 手動走査＋scroll event。負荷は低い（render ごとに 1 回）が可視判定の標準から逸脱。IO 標準で足りるため代替は不要。
- 実行固有ID `assess-20261105-0006` / 項目ID `2026-11-05-hitomi-hub-search-phase1-visible-priority`。

### 推定規模

- `app/search.html`: `loadSearchCards()` + IO 登録 + 優先選択 helper で約 40〜70 行（UI のみ、Go 変更なし）。
- 新規 node harness（順序・実行中昇格の 2 検証）約 60〜80 行 + pytest wrapper（既存 pattern 踏襲）。
- viewer_test.go assertion re-anchor 最大 3 件。

### 優先順位付けに効く要素

- 重要度: 中（Phase 1 残件、UX 体感のキー。Phase 2〜4 は別タスクで順序設計が本タスクの推奨に依存）。
- 緊急度: 中（実装開始指示はリモートタスク発行＝承認済み、ゲート不要）。
- リスク: 低（範囲狭い。フル画像ガード不変条件に接触なし）。
- 依存: canonical build（freshness test 現行一致）、正典 server 実機確認（port 8731・起動済み）、既存 node harness 方式。

### 範囲外で気づいた点（申し送り・実装しない）

- startup duplicate card fetch: fresh tab 読み込みで `/api/search` 応答前に `restoreSearchState()` 由来の card リクエスト occurs（page 1 で 5 件実測）。2026-11-02 code review 記録（`docs/ai/tasks/2026-11-02-external-code-review-f501e2b.md`）の既知項目・未修复（実コードで未解決を確認）。本タスク範囲外。順序実装では restore 経路の二重 queue に注意（昇格対象が重複 queue で分かれる可能性）。
- フル画像ガード触らない範囲のテストは個別 pass 済み（下記 Verification）。実装で本件に接触する理由はない。

### 親控（委譲結果確認・優先順位）

- 要約と記録有無確認済み: 有効性「有効」（実測で閉じ・判断保留なし）、主張は実コードどおり。調査ログ全文は記録済み（本ファイル）。
- 優先順位: 1（単一項目。対応対象・次段階＝実装へ引き継ぎ。実行固有ID `assess-20261105-0006`。根拠: 重要度中／緊急度中（発行＝実装承認済み）／リスク低／依存は canonical build・正典 server・node harness 方式。判断材料不足の項目なし）。

## Verification

- app: `go vet ./...` → pass。`go test -count=1 -timeout 300s ./...` → **ok hitomi-hub/app 23.967s**（本タスクは UI 変更なしの調査のみだが、守るテストの pass を確認）。
- フル画像ガードの守るテスト（個別明記）:
  - `TestStorageURLUsesFormatNotNameExt` PASS
  - `TestCanonicalGGRuleComposition` PASS
  - `TestThumbnailURLMatchesVerifiedTNPath` PASS
  - viewer imageProxy・TN 非混入系（`TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing`、`TestViewerImageProxyReturnsNoFullWhenStorage404AndDoesNotPersistTN`、`TestImageProxyCacheStateIsNotTN`、`TestViewerThumbProxyTNOnlyDoesNotServeFullCache`、`TestViewerThumbProxyTNFailureHeadersDoNotTreatNoTNAsFull` ほか）→ run "TN|imageProxy|NoFull|Cache" **45 件全 PASS（2.177s）**
- `python -m pytest tests/ -v` → **51 passed (10.32s)**（canonical build freshness test 含む = 正典 exe 現行ページ embed 確認）
- 実機確認（評価・ベースライン）: 正典 server 8731、fresh tab、resize 使用前、server direct baseline 先（/search 200）。FIFO・実行中昇格なしを实測で確認（上記調査結果）。保存先: `I:\マイドライブ\kilo\2026-11-05-assess-20261105-0006-phase1-visible-priority\`（png + confirmation txt）。
- 実装後の合格条件（順序固定テスト追加・canonical build 再作成・実機確認）は本評価の範囲外（コード変更不可の調査委譲）。テスト追加方式の推奨のみ記録済み。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
