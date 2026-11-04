# Phase 1 実装 — 検索カードの可視優先順序付け（未開始）

**Status**: 未開始（実装開始指示待ち）
**Date**: 2026-11-05
**Next Action**: 実装未開始・明示的な実装開始指示待ち。開始後は検討結果（推奨=IO標準+takeNext優先選択・実行中昇格）と正典合格条件に従い、テスト→canonical build→実機確認→報告

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase1-visible-priority`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase1-visible-priority.md`）の実装タスク。実装開始は未指示（このタスクは検討結果の反映のみ・コード無変更）。
- 実行固有ID: `assess-20261105-0006`（項目ID `2026-11-05-hitomi-hub-search-phase1-visible-priority`）。評価根拠: 個別タスク記録 `docs/ai/tasks/2026-11-05-assess-20261105-0006-phase1-visible-priority.md`（commit `52b9f27`）。
- 優先順位: 1（assess 完了報告の優先順位付けに基づく）。

## 調査結果 / 変更内容

### 検討結果の反映（assess-20261105-0006・有効性「有効」・実測確定）

- 現行ベースライン（実測済み）: `loadSearchCards()` は `ids.slice()` → `queue.shift()` の FIFO（`app/search.html:295`／`app/search.html:326`）。可視判定機構なし（IntersectionObserver 全リポジトリ 0 件）。page 3 実測で処理中(9/25)のスクロール可視化カードが可視外カードより後処理＝実行中昇格なし。
- 推奨実装方式（採用）:
  1. 可視判定は IntersectionObserver 標準（placeholder `<li>` 観測）。
  2. worker の pop を `queue.shift()` → 優先選択 `takeNext()` へ変更: ①現在可視 ②可視域近い（IO entry 順＝距離）③残りを ids 順。可視集合（Set）＋ pending 配列、n≤25 で O(n) スキャン（ヒープ不要）。
  3. 実行中昇格: IO entry でのマーク＋worker の次回 pop 選択で達成。初期ソートは IO 初回 callback（observe 直後の全対象発火）または同期 `getBoundingClientRect` 走査。
  4. テスト方式: node harness（`tests/harness_lib.mjs` 方式・stub IO で可視集合注入）で `takeNext()` 選択を検証。示すこと: (a) 表示領域内のカードが ID 順で前の可視外カードより先に処理される、(b) 処理中に表示領域へ入った未処理カードが残りのキューより先に処理される（実行中昇格）。
- 評価精确化（テスト照合基準）: 「ID 順」ではなく **server `ids` 配列順**（nozomi 結果順。page 1 末尾 `4300110/4300111` swap 実測）。
- 維持必須（現行どおり）: `searchGeneration` guard、perf 指標・`perf-status`、TN-only cover（full objectURL 参照なし）、`restoreSearchState()` 経由の復元で同じ順序付け、`SEARCH_CARD_CONCURRENCY = 4`、ゲート（data 1000ms/image 250ms）・総リクエスト数不変、ポーリング連打禁止。
- 実装時の注意: `app/viewer_test.go` の関数本文文字列アサート（1678、2129-2135、2362-2401、2857-2878、3013-3019 行）は `loadSearchCards` 本文断片を要求。worker 構造・guard・timing を維持すれば壊れないが、`shift` 削除により最大 3 件 re-anchor の可能性あり → 実装時に確認。
- 範囲外申し送り（このタスクで修復しない）: startup duplicate card fetch（`restoreSearchState()` 由来、page 1 で 5 件実測、2026-11-02 code review 既知未修复）。順序実装では restore 経路の二重 queue に注意（昇格対象が重複 queue で分かれる可能性）。

## Verification

未実施（実装未開始）。評価時の証跡（assess 記録): `go vet ./...` pass、`go test -count=1 -timeout 300s .` ok 23.967s（ガードテスト StorageURL/GG/TN 個別 PASS、TN・imageProxy・NoFull 系 45 件全 PASS）、pytest 51 passed（canonical build freshness 含む）。実装後の合格条件はリモートタスク原文（テスト追加・canonical build 再作成・正典 server 実機確認・冷えた状態・報告）に従い、この節に結果を記録する。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
