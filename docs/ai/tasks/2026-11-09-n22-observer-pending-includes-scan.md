# loadSearchCards の pending.includes が O(n)

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

項目 #15（実行固有ID: assess-20261109-1320）の調査。`loadSearchCards` の IntersectionObserver コールバックで `pending.includes(id)` がイベントごとに O(n) 配列探索になるという指摘の確認:
- 現行コードに探索が実際に存在し、イベントごとに走るか
- pending の実規模（25 固定か、prefetch 分で増えるか）と探索回数の実測
- Set 化のSemantics一致（重複イベント・generation guard・削除タイミング）
- 実害の大きさの実測（µs 級か）

## 調査結果 / 変更内容

### 現在の実装（事実確認）

- `app/search.html:634` `const pending = ids.slice();`（配列）
- `app/search.html:640-648` IntersectionObserver コールバック。`app/search.html:644` に `if (!pending.includes(id)) continue;` — 指摘どおりイベント（entry）ごとに O(n) 走査が走る。意味は「worker が消費済み（pending から splice 済み）の id を skip」
- pending の規模: **上限 25 固定**。`runSearch` の API 呼び出しは `limit=25`（`app/search.html:794`）で `searchPageSlice` が limit で切る（テスト固定 `app/viewer_test.go:1544`）。**prefetch は pending を増やさない** — `prefetchQueue`（`app/search.html:906-926`）は jobs 由来の別 queue で observer なし
- pending は worker の pop（`app/search.html:687` `pending.splice(pending.indexOf(id), 1)`）で縮小し 0 になる。実測平均長 17.1（25 → 0）

### 実測（正典 harness 流儀・tests/harness_lib.mjs 経由）

harness: `docs/ai/tmp/assess-20261109-1320/harness_observer_pending.mjs`（現行 loadBlock をそのまま実行し、`Array.prototype.includes` の instrument で走査回数を計測。scenario = 25 id・全カード(scroll window 8)を一周するモデル: 初期 25 entry + 遷移 34 entry + 処理済みカードの除去 entry 25 = 84 entry）

- membership check 84 回 / 走査 999 回（hit 42・miss 42）
- microbench（node v24・V8）: `Array(25).includes` miss **25.1 ns/回**（≈1 ns/comparison）、`Set.has` **10.5 ns/回**
- 1 セッション合計: array 約 **1.0 µs**・Set 換算 約 0.89 µs・差分 **約 0.12 µs**（scroll が激しくなり entry が数百回 Even ても tens of µs 級止まり）

**「実害小」判定維持**: µs 級。カード 1 枚あたりの処理は API ラウンドトリップ（ms 級）で、observer 機構全体の overhead の 1/1000 以下。

### 有効性

**有効（指摘は事実）だが価値低 — 掃除レベルの低優先項目**。O(n) 走査は実在しイベントごとに走るが、実害は実測で µs 級（上記）。「判断保留」不要、実測で閉じた。

### Set 化の意味一致（現行機構で確認）

- 同一 id の重複イベント: `includes` は純粋な membership 判定のみで順序・回数影響なし → `has` と等価
- 削除タイミング: worker pop（`:687`）で pending から除去される瞬間が gate の意味。Set 化するなら同一箇所で `pendingSet.delete(id)` 相当の同期維持が必要（等価性成立）
- generation guard: コールバック冒頭（`:641`）で membership 判定より前 → 影響なし
- 重複 id: search ids は set 基線 dataset（beIDs）由来で page 内重複は発生せず（正典テスト `TestSearchHTMLPreservesOrOnlyUnionWithoutDuplicateGroup` が union の重複グループなしを固定）。Set のdedup は理論上の差異だが現行機構で発生しない
- 順序意味: `takeNextSearchCard`（`:615-629`）は pending の配列順序（=ids 順）に依存。JS Set は挿入順反復なので full Set 置換でも等価だが、差分最小は補助 Set

### 対応の実現可能性・推奨修正

- **推奨**: 補助 `pendingSet` を membership 判定用に追加（生成時 `new Set(ids)`、worker pop 箇所で `delete`）、observer コールバックは `pendingSet.has(id)` へ。`pending` 配列と `takeNextSearchCard` の順序意味はそのまま（差分最小・意味不変）
- 代替案: `pending` を Set に full 置換（`pending.length`→`size`・`pending[0]`→イテレータ先頭変更が必要）。adjacent な O(n)（`takeNextSearchCard` の 2-pass 走査・`pending.indexOf`（`:687`））も O(1) 化できるが、項目 #15 のスコープ外（それらも同オーダーで µs 級）
- 副作用: `pending.includes` 文字列を pin しているテストはなし（`app/viewer_test.go`・`tests/search_visible_priority_harness.mjs` 確認済み）→ 修正は既存テスト非影響。意味一致を固定する new pin は任意

### 影響範囲・優先順位付け要素

- 重要度: 低（実害 µs 級）。緊急度: 低。リスク: 低（意味一致確認済み・pin なし）。依存: なし
- 単独で着手する価値は薄く、observer 周辺に触れる変更のついでに載せる掃除項目

## Verification

- 本項目はソースコード変更なし（調査子・変更は本タスク記録のみ）。Go 変更なし → 検証コマンド（`go vet . && go test -count=1 -timeout 300s .`）は docs-only 例外（global AGENTS.md コード検証条）で適用なし
- 実測: `node docs/ai/tmp/assess-20261109-1320/harness_observer_pending.mjs`（正典流儀 tests/harness_lib.mjs）— 実行 2026-11-09、経過約 0.09s、結果は上記（84 checks・999 comparisons・includes miss 25.1 ns/回・Set.has 10.5 ns/回・セッション合計 array 1.0 µs vs Set 0.89 µs）
- 画面確認: UI 項目でないため省略（スクリーンショット該当なし）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
