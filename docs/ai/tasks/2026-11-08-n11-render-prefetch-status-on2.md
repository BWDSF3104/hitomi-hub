# N-11: renderPrefetchStatus が呼出ごとに current.files 全件走査（O(N^2)）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `renderPrefetchStatus` が呼ばれるたびに `current.files` 全件を走査している。1000 ページの gallery で画像の完了ごとに key を 1000 個作り、全体では O(N^2)。debounce かカウンタの差分更新。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-11 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（機構確定・判断根拠）

- `renderPrefetchStatus`（`app/viewer.html:432-458`）は呼出ごとに `current.files` 全件を走査し、`actualPixels` / `localDirect` / `objectUrlCache` を map（`displayPassByEntry`・`sourceTypeByEntry`・`sourceStatusByEntry`・`activeObjectUrlKeys`）から毎回再計算。呼出点: display pass（339）、revoke（387）、evict（417）、updatePrefetchCounts 各完了（794/798/806）、gallery open（1038）、manualRetry 完了（1097）、clear（886）。
- `counts`（fullDone 等）は差分更新済みだが、上記 3 値は per-call O(N) 再計算。呼出回数 O(N) × 走査 O(N) = O(N²)。指摘の計算は機構的に確定。
- 正典確認（実在確認済み・読了）: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`。status 意味の正典は `docs/ai/decisions/2026-11-01-viewer-display-status-and-page-switch-recommendation.md`（displayPass=DOM truth・status=source truth・件数表示の意味は map 由来。更新タイミングの意味の固定なし）。

### 有効性: 有効（実測で閉じた）

node harness（`docs/ai/tmp/assess-20261108-1332/r11/r11_measure.mjs`・正典 `harness_lib.mjs` の `bodyIn` で `app/viewer.html` 実ブロックを `new Function` 実行・外部 API なし・9 checks passed）:

- (a) N=1000（呼出 1037 = open 1 + 完了 1000 + display pass 37): 走査 1,037,000 回（1 呼出あたり 1000）・key 文字列生成 1,037,000・累计 621.876 ms（1 呼出 約 600 µs）。
- 二次成長の実測（cumulative ms）: N=100→11.7、N=500→181.1、N=1000→628.2、N=2000→2613.9、N=4000→10736.8。
- (b) 差分カウンタ案（per-gallery カウンタ・同一 predicate・mutation 点で transition-aware 差分）: 呼出数同一（1037）で走査 0・累计 7.898 ms（約 1/79）。per-call 表示文字列が現行と全件一致（即時性保持・最終 count 一致）。境界ケース一致: E1 既知 key の status 遷移（798 path）、E2 sourceType objectURL→local-direct 遷移、E3 revoke（削除前 snapshot）、E4 他 gallery key の帰属と切替（key prefix 帰属・`decrementPrefetchedCountForKey` の流儀）。
- debounce 案（trailing 250 ms・read/lists convention 引用）: 表示値は最終一致だが呼出あたり即時更新が失われる。dense stream（70 ms 間隔・1037 呼出）→ 1 fire（中間 1036 件スキップ）、sync burst → 1 fire、rAF（16 ms）は疎 event では削減なし（1037）。served 実測（同一 tab 同一条件 patch sim）: 100 呼出→34 renders、lag 最大 250 ms、最終表示=per-key 再計算と一致。

### 対応の実現可能性・推奨修正方法

- 推奨: **差分カウンタ案**（per-gallery・per-key contribution を現行 map の membership で算出、mutation 点で before/after 差分、render O(1)）。表示の意味（件数・即時更新・per-entry 判定）を変えない。R-08 の per-entry `local` 意味（per-entry flag・search card 分子）は map 判定由来で不変。
- 制約: `app/viewer_test.go:2850-2860` が `renderPrefetchStatus` 本体の文字列（日本語ラベル・`displayPassByEntry.get(key)`・`sourceTypeByEntry.get(key)`・`activeObjectUrlKeys.has(key)`）と `renderRetryButtonState()` 呼出、関数順（renderRetryButtonState→renderPrefetchStatus→stripItems）を pin。pure delta は本文から get/has が消え test 更新（re-anchor）が必要。既存テストは件数・更新タイミングの意味を固定していない（tests/ 照合: status 意味固定は文字列 pin のみ、timing pin なし。`tests/viewer_objecturl_cap_harness.mjs` は renderPrefetchStatus を stub するため内部変更非影響）。既存テストを変更しない実装形を選ぶなら、pin 文字列を保つ guarded reconciliation（初回/gallery 切替時のみ全件照合、steady state はカウンタ）が必要。
- 代替案（定数削減のみ・test 形保持）: 呼出時に `current.files` 走査へ回す key list を open 時に 1 回生成（key 生成 1,037,000 回→1000 回）。O(N²) 走査は残る（実測上は debounce/差分が本命）。

### 副作用・影響範囲

- debounce 側は「画像完了ごとに status が更新される」即時性（ユーザーに見える更新順序・lag）を変える → 承認案件として分離（served 現行は完了ごと即時更新を観測済み）。単一の status 行を loadFullImage の per-page transient 文（エラー/no_full/TN禁止: page）が共有するため、debounce は transient 文の可視時間の意味も変える。
- 差分カウンタは set 判定（key 集合 membership）を保持する限り per-entry 判定（どの file が保存済みか）の意味を変えない（E1–E4 一致）。generation guard・`x-cache` 判定・objectURL 表示・TN 表示源禁止・TN バイト非保存は非接触。
- 影響: viewer.html 関数内部のみ。正典 exe 再生成が必要（freshness test 経路）。

## Verification

- `python -m pytest tests/ -v` → 59 passed・1 failed（10.26 s）。failure は既知の既存 `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来・非接触・本項目と無関係）。
- `node docs/ai/tmp/assess-20261108-1332/r11/r11_measure.mjs` → 9 checks passed（上記 (a)(b) 実測。経過 約 15 s 内）。スクラッチ: `docs/ai/tmp/assess-20261108-1332/r11/`（r11_measure.mjs・r11_dbcheck.py）。
- `node tests/viewer_objecturl_cap_harness.mjs` → 既存 harness 通過（非影響確認・テスト変更なし）。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean・ok 26.756 s（源コード非変更・pin 保持）。
- 画面確認（served・正典 exe fresh build・fresh instance pid 2624・port 8740・isolated origin n11-assess-20261108・window outer 1678x1025・inner 1665x932・gallery 4309093（1921 files）): before 50/147 → 操作（#nav 次・page 5-6・reader img naturalWidth 2880 objectURL）→ after objectURLキャッシュ 176/1921（100 ms sampler で完了ごとの増分更新＝即時観測）。debounce sim（同一条件 patch）: 100 呼出→34 renders・lag ≤250 ms・最終表示=per-key 再計算一致。
  - 保存先: `I:\マイドライブ\kilo\2026-11-08-n11-render-prefetch-status-on2\`（`n11-status-before-20261108.png`、`n11-status-after-20261108.png`、`n11-canonical-before/after-20261108.png`、`n11-debounce-after-20261108.png`、`n11-prefetch-status-confirmation-2026-11-08.txt`）。local copy（project scratch）は正典 `kilo-safe-remove` でゴミ箱へ送った。Playwright 一時出力（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`）は project root 外のため残件として記録（削除なし）。正典 server instance（8740）は本確認専用で停止済み。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（本項目は記録のみ・親の task_manager で管理）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）— 本項目は指摘検証のみで known-issues 新規なし（対応は実装指示待ち）
- [ ] `backlog.md` の完了項目を削除（該当する場合）— 該当なし（本項目は findings 由来・backlog 項目ではない）
