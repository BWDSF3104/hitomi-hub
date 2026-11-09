# N-07: prefetch worker の race で item が undefined（KI-20261107-01）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: prefetch worker の race が未修正（KI-20261107-01）。await の間に他 worker が queue を空にすると `item` が undefined になり、catch 側の `item.entry` で再び TypeError が出る。`shift()` の直後に `if (!item) return;` を入れれば直る。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-07 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `app/viewer.html` `startBackgroundWorkers(signal)` の `worker()`（`PREFETCH_CONCURRENCY = 8`）は `while (prefetchQueue.length && !signal.aborted)` → `await currentPriorityDone` → `const item = prefetchQueue.shift()` → try（`prefetchPageImage(item.entry, ...)`）→ catch（`updatePrefetchCounts("error", item.entry, item.galleryId)`）。catch 内・try 内の `item.entry` がいずれも `item` undefined で TypeError。catch 側の TypeError は catch されず unhandled rejection として worker 外へ出る（現行コードに guard なし: `tests/harness_lib.mjs` の `bodyIn` 抽出で確認。handler を付けない初回実行では Node プロセス自体が unhandled rejection で落札し、served の console error（KI-20261107-01 R-15 1 回観測）と同形）。
- race 条件: worker が length check（length ≥ 1）通過後に await している間に、後続登録の他 worker が最後の 1 件を shift して queue を空にする。単一スレッドでは「length check と shift の間に await を挟む worker」のみ該当し、現行コードの `await currentPriorityDone` がその窓。

### 有効性

- **有効（実測で確定）**。スクラッチ `docs/ai/tmp/assess-20261108-1332/r07/n07_worker_race.mjs`（既存 harness 流儀: `bodyIn` 抽出 + `new Function`、microtask 登録順で決定論的再現）:
  - 現行: catch 側 `item.entry` の TypeError が 1 件外部へ（`Cannot read properties of undefined (reading 'entry')`）。エラー count 0（count 実行前に throw）、処理件数 1（正常 worker）、worker 正常終了 1・例外終了 1、queue 残件 0。
  - guard 適用（`shift()` 直後 `if (!item) return;` をスクラッチ側のみへ適用）: 例外外部への発生 0、worker 正常終了 2、処理件数 1・queue 残件 0（現行と同一）。guard は length check 通過後の race 時のみ発火し、queue が空なら worker はもともと `while` 条件で正常終了する。guard は「自然終了相当」の追加であり意味を変えない。
  - error path（定義済み item の実エラー）: 現行・guard 両版で完全一致（error count 1、処理 2、worker は continue して正常終了）→ guard は catch の error handling（worker 継続）意味を変更しない。

### 正典照合（矛盾確認）

- `docs/ai/known-issues/KI-20261107-01-viewer-prefetch-worker-undefined-item-race.md`（Status 未解決・次の対応 未実施）と項目原文「未修正」は一致。矛盾なし（採用文書: KI 本体。archive へ移動済みでないことも確認）。機構記述（served line 1114・HEAD 0b34704 由来）は現コードと一致。

### 対応の実現可能性と推奨

- 推奨: 項目原文どおり `const item = prefetchQueue.shift();` の直後へ `if (!item) return;`（1 行）。等価案: length check を await 後へ移す案（KI 記載の代替案）— 実効は同じで、shift 直後 guard のほうが `while` 条件の自然終了と一致し追加 의미가 0。どちらでも実装開始指示後に実施可（記録のみ・未実装）。
- 既存テスト照合: `tests/*.mjs` に worker 内部構造（`prefetchQueue.shift`・`while (prefetchQueue.length`・catch 内 `item.entry`）を固定するアサーションなし。`viewer_test.go` は `startBackgroundWorkers(signal)`・`prefetchQueueKeys` などの文字列存在と manualRetry の catch（「unhandled rejection を漏らさない」）を固定するのみで、guard 追加は影響しない。search prefetch 系テストは `app/search.html` の別 worker（concurrency 4）を固定しており非接触。既存テストは変更していない。

### 副作用・影響範囲

- `await currentPriorityDone` の優先待ち（`docs/ai/decisions/2026-11-01-operation-priority-placeholder.md`・`2026-11-01-required-function-verification.md` の「background worker は current priority を待つ」）は guard の位置が await より後なので不変。queue の优先级（距離順・`prefetchQueueKeys`）も不変。
- TN rules（TN を reader 表示源にしない・TN display fallback 禁止）、`x-cache` 判定、objectURL 表示、generation guard は worker 呼び出しの guard 追加で接触なし（実測 error path 一致 + 変更箇所が view/display 経路外）。
- catch の正典意味（aborted fetch error を catch し unhandled rejection を漏らさない, `viewer_test.go:2333-2335`）と衝突しない: guard は undefined item の catch 内 throw（= unhandled rejection 漏出）を消す方向で正典意味に一致。衝突する案はなし（承認分離不要）。

### 画面確認

- **未確認**。server（127.0.0.1:8731）起動済みでなく、race は microtask 順序依存のため served 画面で決定論的に再現できない（ソース変更禁止）。Google Drive 保存なし（cold hygiene・before/after 適用できず）。スクラッチ harness 実測で機構確定。

## Verification

- スクラッチ node harness（`docs/ai/tmp/assess-20261108-1332/r07/n07_worker_race.mjs`）: 9 check 全 pass（0.4 s）。数値: 現行 race = 例外外部 1・error count 0・処理 1・worker 正常 1/例外 1・queue 残 0。guard 版 race = 例外 0・worker 正常 2・処理 1・queue 残 0。error path 両版一致（error count 1・処理 2・worker 継続）。
- `python -m pytest tests/ -v`: 59 passed・15.03 s（経過 16.1 s）。既知 failure `test_canonical_build_freshness_rule_exists`（KI-20261106-04・AGENTS.md pointer 案件）は非接触として明記。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet clean・`ok hitomi-hub/app 35.054s`（経過 38.1 s）。baseline 通過（worker 内部を固定するアサーションなしを確認済み）。
- 画面確認: 未確認（上記）。実装未実施（guard はスクラッチ側のみ適用、ソース未変更）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

注: 本調査は記録のみで guard を実装していないため、KI-20261107-01 は未解決のまま（archive 移動は実装完了時）。項目原文の修正（1 行 guard）実施後に上記チェックを反映する。

## 2026-11-09 再調査（assess-20261109-1320）

### 現在の実装と判断根拠

- 現行 `app/viewer.html` `startBackgroundWorkers(signal)` の `worker()`（1102-1119 行、`PREFETCH_CONCURRENCY = 8`）に guard **未実施（未修正）**。`while (prefetchQueue.length && !signal.aborted)`（1104）→ `await currentPriorityDone`（1105）→ `const item = prefetchQueue.shift()`（1107）の直後に `if (!item) return;` なし。catch（1112-1115）で `item` undefined 時 `updatePrefetchCounts("error", item.entry, ...)`（1114）が TypeError。呼び出し側 `for (...) worker();`（1118）に handler なし → 例外は worker 外へ unhandled rejection として漏出。guard 有無は現行ソースの文字検索（scratch harness）で `guardInCurrent = false` として実測。
- race 窓は前批と同一: length check 通過後の `await currentPriorityDone`（1105）が窓。1106 の `if (signal.aborted) return;` は窓を塞がない（undefined shift は aborted と無関係）。
- 機構は KI-20261107-01 の記述（served line 1114・HEAD 0b34704 由来）と現行コードで位置も含め一致（現行 1114 行が該当文）。2026-11-07 以降 `app/viewer.html` の commit（0cb18b7・7cb776a・e1ae526・823ff16）は worker 構造を追加修正していない（guard なしの現行で確認）。

### 有効性

- **有効（実測で確定・未修正）**。前批 harness は `docs/ai/tmp/` の 7 日アーカイブ規則で消失していたため、正典流儀（`tests/harness_lib.mjs` の `bodyIn` 抽出 + `new Function`）で再作成し現行コードから決定論的に再現（`docs/ai/tmp/assess-20261109-1320/n07_worker_race.mjs`、6 check 全 pass）:
  - 現行 race（concurrency 2・queue 1 件）: catch 側 `item.entry` の TypeError 1 件が worker 外へ（`Cannot read properties of undefined (reading 'entry')`）。error count 0（throw が count 実行前）、処理 1・queue 残 0・worker 正常 1/例外 1。
  - guard 版（`shift()` 直後 `if (!item) return;`）: 例外 0・worker 正常 2・処理 1・error 0・queue 残 0（現行と正味同一。guard は自然終了相当の追加）。
  - error path（定義済み item の実エラー）: 現行・guard 両版完全一致（error 1・cache 1・worker 継続）→ guard は catch の正典意味（unhandled rejection を漏らさない）を変更しない。
- 前批記録（assess-20261108-1332）の数値・推奨・KI 状態（未解決・次の対応 未実施）と矛盾なし。正典文書間の状態記述矛盾なし（KI 本体・個別タスク記録・項目原文すべて「未修正」で一致）。

### 対応の実現可能性と推奨

- 推奨は前批と同一: `const item = prefetchQueue.shift();` 直後へ `if (!item) return;`（1 行）。KI 記載の代替案（length check を await 後へ移す）も実効同一。実装開始指示待ち（本調査は記録のみ、ソース未変更）。

### 副作用・影響範囲

- 前批と同一結論、現行コードで再確認: `await currentPriorityDone` 優先待ち正典（`docs/ai/decisions/2026-11-01-operation-priority-placeholder.md`）は guard が await より後なので不変。TN rules・`x-cache`・objectURL 表示・generation guard・queue 优先级（`prefetchQueueKeys`）非接触。
- `viewer_test.go` の worker 関連アサーションは関数名（`startBackgroundWorkers(signal)`・`prefetchQueueKeys`）と manualRetry の catch 意味固定のみで、worker 内部構造（`while (prefetchQueue.length`・`prefetchQueue.shift()`・catch 内 `item.entry`）を固定するアサーションなし（grep 読み取りで確認。guard 追加は影響しない）。
- `app/search.html` 側（項目原文の「同期ブロック」主張）: `worker()` の length check（909）と `pending.shift()`（910）は同一同期ブロックで await 窓なし → 主張どおり該当なし。search  prefetch worker（concurrency 4）は別実装であり本項目の修正対象外。

### 画面確認

- **未確認**（前批と同一理由）。server（127.0.0.1:8731）は 2026-11-09 時点で起動済みなし（TCP 確認 False）。race は microtask 順序依存で served 画面では決定論的に再現できず、ソース変更禁止のため cold 確認条件（fresh canonical build）でも証跡にならない。Google Drive 保存なし（画面確認実施せず）。有効性是実測（scratch harness）で確定済み。

### Verification（本再調査）

- scratch harness `docs/ai/tmp/assess-20261109-1320/n07_worker_race.mjs`: 6 check 全 pass（経過 <1 s）。数値は前批と一致（上記）。
- コード検証コマンド（Go app `go vet . && go test -count=1 -timeout 300s .`）は実行不要として記録: 本調査はソース変更なし、テスト呼び出しの要否は読み取り（worker 内部固定アサーションなし）で足りると確認。実装実施時に変更後に実行する。
- ソース未変更（guard はスクラッチ側のみ適用）。Status・Next Action は親管理（正典 task_manager）として変更せず、本節のみ追記。
