# Phase 1 実装 — 検索カードの可視優先順序付け（完了）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 本項目分のみ commit/push 後、正典 task_manager.py end

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

- 項目ID: `2026-11-05-hitomi-hub-search-phase1-visible-priority` / 実行固有ID: `assess-20261105-0006`。
- 変更ファイル: `app/search.html`, `tests/search_visible_priority_harness.mjs`, `tests/test_search_visible_priority.py`, 本タスク記録, `docs/ai/current-task.tsv`。

### 実装済み内容

- `app/search.html` の `loadSearchCards()` に `IntersectionObserver`, `visibleIds`, `pending`, `placeholderElement`, `distanceToVisibleArea`, `takeNextSearchCard`, `observer.disconnect()` を追加。worker pop を `queue.shift()` から `takeNextSearchCard()` の優先選択へ変更。
- 維持確認: `SEARCH_CARD_CONCURRENCY = 4`, data gate `1000ms`, image gate `250ms`, `searchGeneration` guard, `restoreSearchState()` 経由の同一 `loadSearchCards()` 経路, TN-only cover, `x-cache` 判定弱体化なし。

### 合格条件

1. node harness: `node tests/search_visible_priority_harness.mjs` → `search visible priority harness: 20 checks passed`。
   - 表示領域内カードが可視外カードより先に処理される: PASS。
   - 実行中昇格: card that enters display area during processing is promoted ahead of remaining queue / promotion adds no extra request: PASS。
   - `SEARCH_CARD_CONCURRENCY = 4`, generation guard, per-card perf timing, bounded workers, restore path, observer root results scroll container, no polling loop: PASS。
2. pytest wrapper: `python -m pytest tests/ -v` → `52 passed in 12.01s`, including `tests/test_search_visible_priority.py::test_search_visible_priority_order_and_promotion PASSED`。
3. `app` verification: `go vet . ; go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 18.355s`。`viewer_test.go` の `loadSearchCards` 文字列アサートは現行構造で pass、再アンカー不要と確認。
4. guard tests individually: `go test -count=1 -timeout 300s -run 'TestStorageURLUsesFormatNotNameExt|TestCanonicalGGRuleComposition|TestThumbnailURLMatchesVerifiedTNPath' .` → `ok hitomi-hub/app 0.168s`。TN / imageProxy / NoFull / Cache 系は full `go test` で全 pass。
5. canonical build freshness: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass; `python -m pytest tests/test_canonical_build_freshness.py -v` → `2 passed`.

### Canonical server / cold UI real confirmation

- Canonical server command: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`。
- Stale server PID `31304` を停止し、正典 server PID `24340` を正典 exe で再起動。Served page contains `takeNextSearchCard` and `IntersectionObserver` を確認。
- Google Drive 保存方針: `docs/ai/decisions/2026-11-02-google-drive-confirmation-storage-procedure.md` の日付フォルダ規定と `docs/ai/decisions/2026-11-02-google-drive-task-specific-screenshot-folder.md` のタスク別フォルダ規定が矛盾していたため、後者のタスク別フォルダ優先を採用。保存先: `I:\マイドライブ\kilo\2026-11-05-phase1-visible-priority-implementation\`。

Primary cold screen evidence:

- URL: `http://127.0.0.1:8731/search?newtab=1#/q/furry/page/9`
- fresh isolated tab, `localStorage` cleared before navigation.
- cold evidence: first `/api/search-card` response header for card `4285339` was `x-cache: miss`.
- screen capture: `phase1-visible-priority-page9-midrun-viewport-20261105.png` saved to Google Drive task folder.
- midrun status snapshot: `カード読み込み 12/25: 4284485`。
- server `ids` 配列順（DOM order, 25 cards）:
  `4285339,4285322,4285317,4285279,4285229,4285225,4285194,4285165,4285151,4285140,4285113,4285086,4284988,4284976,4284880,4284701,4284639,4284485,4284213,4284091,4283615,4283548,4283465,4283355,4283063`
- request order after scroll:
  `4285339,4285322,4285317,4285279,4285229,4285225,4285194,4285165,4284091,4283615,4283548,4283465,4283355,4283063,4284213,4284485,4284639,4284701,4284880,4284976,4284988,4285086,4285113,4285140,4285151`
- visual state: display-area bottom cards `4285086,4284988,4284976,4284880,4284701,4284639,4284485,4284213,4284091,4283615,4283548,4283465,4283355,4283063` were processed/displayed while ids-order-earlier pending cards `4285151,4285140,4285113` remained loading. This confirms scroll promotion and display-area priority.

Additional live confirmation:

- URL: `http://127.0.0.1:8731/search?newtab=1#/q/furry/page/8`
- request order after scroll:
  `4288223,4288184,4288153,4288143,4288125,4287912,4287896,4287895,4285765,4285709,4285694,4285618,4285579,4285398,4285819,4285894,4286266`
- display-area bottom cards `4285765,4285709,4285694,4285618,4285579,4285398` were requested before ids-order-earlier pending cards `4285819,4285894,4286266`.
- screen capture: `phase1-visible-priority-page8-midrun-20261105.png` saved to Google Drive task folder.
- confirmation txt: `phase1-visible-priority-confirmation.txt` saved to Google Drive task folder.

### 最終時点画面確認（2026-11-05 subagent・追加証拠のみ）

- Status / Next Action は変更しない。実装変更なし、記録の追加証拠のみ。
- 正典 server command: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`。
- 正典 server pid: `6452`。
- Direct baseline（UI 計測前）:
  - `http://127.0.0.1:8731/api/search?q=furry&lang=all&page=19`
  - `X-Search-Timing: search=0;record=53;json=1`、`Content-Length: 991`、`Cache-Control: no-store`
  - total `32917`、ids count `25`
  - ids order: `4250610,4250432,4250419,4250354,4250350,4250333,4250263,4250262,4250137,4250108,4250110,4250107,4250064,4249890,4249838,4249791,4249585,4249060,4249016,4249015,4249009,4249000,4248900,4248750,4248541`
- Fresh isolated browser、storage clear、actual UI `#query=furry`、`#page=19`、`#search-submit` click。
- 計測注入: `/api/search-card` fetch-order wrapper、IntersectionObserver stub / visible set、`window.startPrefetchNow = async () => {}`。プロジェクトコード変更なし。
- UI response `ids` / total direct baseline と一致。UI `X-Search-Timing: search=0;record=41;json=1`、`Content-Length: 991`、`Cache-Control: no-store`。
- first card cache: `miss`。
- Midrun status at screenshot: `カード読み込み 21/25: 4248541`。
- processed first-occurrence order: `4250107,4249890,4250064,4249838,4249791,4249585,4250432,4250354,4250419,4250610,4250350,4250333,4250263,4250262,4250137,4250110,4250108,4249060,4249016,4249015,4249009,4249000,4248900`。
- pending at midrun: `4248750,4248541`。first processed `4250107` は `ids` index `11` で、pending top `4248750`（index `23`）/ `4248541`（index `24`）より先に処理。
- Pass: `python -m pytest tests/test_canonical_build_freshness.py -v` → `2 passed`。
- Google Drive evidence: `final-phase1-page19-20261105.png`、`final-phase1-confirmation-20261105.txt`。スクショは Playwright 画面スクリーンショット代替（許可済み、記録済み）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
