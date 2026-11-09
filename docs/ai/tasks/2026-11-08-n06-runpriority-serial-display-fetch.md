# N-06: runPriority が表示中の 2 枚を直列取得している

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `runPriority` が表示中の画像を直列に取得している。見開きの 2 枚を 1 枚ずつ `await` する。image lane の間隔は 60ms なので並列にしても支障はない。cold 時の待ちが約 2 倍。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-06 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

- 指摘は有効。`runPriority` は display items を `for` ループで 1 枚ずつ `await loadFullImage(...)` している。表示 2 枚 cold では 1 枚目の fetch 完了後に 2 枚目が開始する。
- 現行表示経路:
  - `app/viewer.html:998` `runPriority(displayed, signal)`。
  - `app/viewer.html:1008` の `if (img) await loadFullImage(img, item.entry, signal, priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen);` が直列 await。
  - `loadFullImage`（`app/viewer.html:889`）は `entrySource` の URL を fetch し、x-cache / objectURL / generation guard を確認する。display path では `priority=background` を付けないため server 側 foreground 扱い。
  - `imageProxy`（`app/viewer.go:1066`）は `r.URL.Query().Get("priority") == "background"` で background を判定する。display path の foreground lane は `app/hitomi.go` の image lane interval を使う。
- Gate 機構との整合:
  - `gateLane` foreground は future reservation で interval を dispatch 間隔として維持し、network overlap を可能にする。既存意味（FIFO、30 s grace、interactive search priority、background worker priority）を変え並列化する案は実測が予算例 `1.5 s` を超える場合だけ検討案。本項目の実測は予算例を超えないため採用しない。
  - canonical image lane interval は `60 ms`、data lane interval は `90 ms`、lower guard は `50 ms`、politeness 退避先は `1000 ms`。`app/hitomi.go` の `defaultHitomiConfig()` は `MinIntervalMS: 90`, `ImageMinIntervalMS: 60`、`politenessDefaultIntervalMS = 1000`。
  - `fetchFullImageOnce` の key 単位 singleflight は別 hash / key の display 2 枚では leader 意味を変えない。
- Scratch mock 実測: `docs/ai/tmp/assess-20261108-1332/r06/app/n06_measure_test.go`。`fetchFunc` で network を `679 ms` 固定遅延として模擬し、外部 API は呼出した。
  - SERIAL median wall: `1359 ms`。call1 gate `0 ms` / network `679 ms`、call2 gate `0 ms` / network `679 ms`。
  - PARALLEL median wall: `739 ms`。second call gate wait `60 ms`、dispatch gap absolute `60 ms`。fully simultaneous arrival では slot order race 可能だが interval 自体は維持。
  - PARALLEL-ORDERED median wall: `739 ms`。JS `Promise.all` 相当で call1 → call2 の配列順 dispatch を 1 ms 差で再現。call1 gate `0 ms`, call2 gate `58-59 ms`, dispatch gap `60 ms`。
  - metrics lookup は storage URL full image class を `classifyFetchURL(url1)` で `"data"` として取扱う。初回 `metricsSnapshot()["image"]` を想定した lookup は `classifyFetchURL` の意味に合わず fail した。
  - 推定規模: parallel dispatch で cold display pair の wall は mock 上約 `739 ms`、serial は `1359 ms`。cold cold の約 2 倍指摘は mock network 一定条件で有効。実 network では per-file network が異なるため wall は条件依存。
- Served cold UI 実測: gallery `3863136` の cold display pair を fresh server instance・new isolated origin で確認。resource timing で display 2 枚が直列であることを確認。
  - actual window size: `1678x1025`、content viewport: `1665x932`、devicePixelRatio `2`。
  - first `imageProxy`: start `158 ms`, duration `957 ms`, transferSize `206099 bytes`, `x-cache: miss`, `200 OK`, `content-type: image/avif`。
  - second `imageProxy`: start `1116 ms`, duration `235 ms`, transferSize `138356 bytes`, `x-cache: miss`, `200 OK`, `content-type: image/avif`。
  - second start は first end（`158 + 957 = 1115 ms`）の直後。display pair overall wall は約 `1351 ms`。
  - before screenshot は placeholder/loading。after screenshot は actual pixels 表示。UI info: `実表示ピクセル 2/2`, `objectURLキャッシュ 2/2`, `フル画像なし 0/2`, `TN表示フォールバック禁止`。TN display fallback は確認されない。
  - Google Drive 保存（親の訂正・2026-11-08 11:52Z）: 子の実測時点では Drive へ画像が保存されておらず、confirmation txt が Drive 直下で「フォルダ名そのままのファイル」として存在していた（Glob・read で file と確認）。親が Drive にフォルダ `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\` を作成し、txt を同名へ移動、Playwright temp の `n06e-before-3863136-cold.png`・`n06e-after-3863136-cold.png` と、project root に残っていた before 試行 3 枚（`n06-`/`n06b-`/`n06c-before-cold-placeholder-20261108.png`）を同じフォルダへコピーした。
  - local copy cleanup: 上記 3 枚（project root 内）は Drive 保存後に正典 `kilo-safe-remove` でゴミ箱へ送った。Playwright output local copies（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-before-3863136-cold.png`, `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-after-3863136-cold.png`）は project root 外のため削除せず残存（Drive へコピー済み。削除には追加承認が必要）。
- 実装案（記録のみ、実装開始指示待ち）:
  - `runPriority` を Promise.all / errgroup 相当で display 2 枚を並列 dispatch するのが妥当。
  - dispatch order は array order を残す。既存 gate interval `60 ms` は dispatch interval として維持でき、FIFO future reservation 意味を変えない。
  - generation guard、abort signal、x-cache、no-store fetch、objectURL display、TN display fallback 禁止、full cache に TN byte を保存しない条件は維持する。
  - completion order が first → second と異なる可能性は UX 変更として承認案件。UI の strict first→second display order を残す要求があれば、Promise.all より dispatch order は保つが arrival order race を許容する設計にする。
  - existing tests は `loadFullImage(..., priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen)` substring と `await currentPriorityDone` を固定するが、`runPriority` 内部の直列 await order を固定していない。Promise.all 実装でも existing tests は維持可能。
- 優先順位付け要素:
  - cold wall reduction: mock 上 serial `1359 ms` → parallel `739 ms`。
  - gate politeness compliance: image lane `60 ms` interval を超えない並列 dispatch。
  - UI responsiveness: fast/non-blocking viewer policy 下で display actual pixels を objectURL で表示する条件を弱めない。
  - UX order risk: arrival order race。既存 viewer display order が strict first→second を要求するなら承認必須。
  - dependency: TN display fallback禁止、full actual pixels、persistent full cache、`no_full` guard を維持する。

## Verification

- app verification（source change なしだが本項目関連 code read と scratch measurement で再実行）:
  - command: `go vet . && go test -count=1 -timeout 300s .`
  - result: `ok hitomi-hub/app 25.859s`
- Python verification:
  - command: `python -m pytest tests/ -v`
  - result: `59 passed`, known failure `test_canonical_build_freshness_rule_exists` FAILED。この known failure はこの項目の非接触として記録。
  - `test_canonical_executable_embeds_current_pages` PASSED。canonical `hub.exe` が current pages を embed していることの evidence。
- Scratch mock measurement:
  - worktree: `docs/ai/tmp/assess-20261108-1332/r06/app/`
  - command: `go test -count=1 -run "TestN06MeasureColdSpread" -v .`
  - result: `go vet` clean。SERIAL median wall `1359 ms`, PARALLEL median wall `739 ms`, PARALLEL-ORDERED median wall `739 ms`, image lane dispatch gap `60 ms`。
- Canonical UI confirmation:
  - canonical server: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`
  - fresh server instance pid: `4056`。previous pid `17612` を stop 後に起動。
  - isolated context: `n06-cold-3863136`。cold gallery `3863136`。
  - actual window size: `1678x1025`、content viewport: `1665x932`、devicePixelRatio `2`。
  - before screenshot: placeholder/loading。after screenshot: actual pixels `2/2`, objectURL cache `2/2`, full image none `0/2`, TN display fallback禁止。
  - served cold resource timing: first `imageProxy` start `158 ms`, duration `957 ms`; second `imageProxy` start `1116 ms`, duration `235 ms`; second start after first end。
  - served cold network responses: `200`, `x-cache: miss`, `content-type: image/avif`。
  - Drive files: `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06e-before-3863136-cold.png`, `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06e-after-3863136-cold.png`, `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06-cold-display-pair-confirmation-2026-11-08.txt`。
  - local cleanup: `kilo-safe-remove 'C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-before-3863136-cold.png'` と `kilo-safe-remove 'C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-after-3863136-cold.png'` は project root 外のため fail。project scratch 内の invalid before screenshot と confirmation text local copy は `kilo-safe-remove` でゴミ箱へ送った。

### 実装後の検証（2026-11-09・assess-20261109-1320 委譲分）

- app verification:
  - command: `go vet . && go test -count=1 -timeout 300s .`（`app`）
  - result: `go vet` clean / `ok hitomi-hub/app 26.262s`（既存テスト変更不要・pin 文字列維持で合格）
- Python verification:
  - command: `python -m pytest tests/ -v`
  - result: `64 passed in 21.87s`（`test_canonical_build_freshness.py` 含む。前批の known failure `test_canonical_build_freshness_rule_exists` は現行合格）
- Canonical build（fresh）: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（`app` で実行）→ freshness test 合格＝canonical exe が現行 `app/viewer.html` を embed
- harness（正典流儀・mock・外部 API なし）: `node tests/viewer_runpriority_parallel_harness.mjs` → `16 checks passed`。SERIAL reference median `1370 ms` / PARALLEL median `747 ms`（0.54）・dispatch gap median `62 ms`（image lane 60 ms 遵守）
- served cold 画面確認: before（プレースホルダ）→ after（実像素 9/9・TN fallback なし）合格。詳細は上記「実装」節。Drive 保存先 `I:\マイドライブ\kilo\2026-11-09-n06-runpriority-serial-display-fetch\`

## 2026-11-09 再調査（assess-20261109-1320）

### 現在の実装と判断根拠

- 現行 `runPriority`（`app/viewer.html:998-1010`）は `for (const item of displayed)` ループ内で `await loadFullImage(...)` の直列 await のまま。並列化（`Promise.all` / `allSettled`）は未導入（harness 文字列チェックで確認）。
- dispatch order は array order（ソートなし・配列順 iterate）。display order（描画順）は render array order + RTL reverse のみ（`app/viewer.html:993`）。直列では completion order = dispatch order。
- 作業前チェック（フル画像・viewer 触る項目）として正典 3 件をファイル名で確認 reading: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。
- gate 正典（`app/hitomi.go` `gateLane`・495-564）は foreground FIFO 将来予約のまま変更なし。display path（`priority=background` なし）は image lane interval `60 ms`。正典値は現行でも data `90 ms` / image `60 ms` / politeness 退避 `1000 ms`（`defaultHitomiConfig()` 319-320）。
- 前批記録との矛盾: 前批は grace を「30 s」と記録したが現行コードは `defaultBgGraceMS = 300`（`app/hitomi.go:467`・decision 2026-11-05）。日付の新しい正典（2026-11-05 decision と現行コード）を優先し、300 ms を採用。矛盾は記録どおり。

### 有効性

有効（cold・remote display pair 限定）。現行でも 2 枚 cold では 1 枚目の fetch 完了後に 2 枚目が開始する（実機 cold で直列確認済み）。gate が空いている局面での無駄待ちは現行でも成立: 実測で 2 枚目は gate 待ち 0〜1 ms で dispatch 可能だったのに直列 await により 1 枚目の network 完了（1237 ms）を待った。効果は local/persistent full cache ヒット時（network なし）では無効（renderReader が objectURL/local で表示し、loadFullImage に network 待ちがない）。interval 支配の低速 network 局面ではゲート 60 ms が下限。

### 実測（mock・正典流儀 tests/harness_lib.mjs）

- scratch harness: `docs/ai/tmp/assess-20261109-1320/n06_remeasure_harness.mjs`（`node` で実行。external API なし。gate は現行 `gateLane` foreground 意味（`last+interval` 以降は即時予約）を模倣、network `679 ms` 固定・5 trials median）
- SERIAL median wall `1378 ms`。PARALLEL median wall `752 ms`（前批 1359→739 と整合）。
- SERIAL 2枚目 gate wait median `0 ms`（gate idle・無駄待ちは network 直列化）。PARALLEL 2枚目 gate wait median `60 ms`、dispatch gap median `63 ms`（image lane interval `60 ms` 遵守）。
- arrival order race 再現: per-file network が異なる（1枚目 957 ms・2枚目 235 ms）とき parallel では 2枚目が先に completion → strict first→second display order と衝突（承認案件は前批どおり現行でも有効）。dispatch order は array order のまま維持可能。

### 実測（served cold UI・確認テストルール適用）

- cold hygiene 3条件: fresh canonical build（pytest freshness 含む 60 passed）/ fresh server instance（pid 26072・new root）/ isolated origin（`n06-rem-20261109-cold5`）。window size `1666x932`・devicePixelRatio `2`。
- cold display pair gallery `4306867` page 7-8:
  - 1枚目（page 7・hash `fd489968…`）: start `356 ms`、duration `1237 ms`、transferSize `805683 bytes`、`x-cache: miss`、200、`image/avif`
  - 2枚目（page 8・hash `f88c78fe…`）: start `1594 ms`（= 1枚目 end `1593 ms` + 1 ms）、duration `328 ms`、transferSize `915973 bytes`、`x-cache: miss`、200。直列確認。
  - background prefetch は priority 完了後（`1922 ms`）から開始。UI info（after）: 位置 7-8/9、実表示ピクセル 2/9、ローカル直接 0/9、objectURLキャッシュ 2/9、フル画像なし 0/9、TN表示フォールバック禁止。
- before/after 組（同一 window size・同一 full page 範囲・同一取得方法）: before = プレースホルダ/読み込み中、after = 実像素 2/9。合格。
- Google Drive 保存先: `I:\マイドライブ\kilo\2026-11-09-n06-runpriority-serial-display-fetch\`（`n06-rem-20261109d-4306867-pair78-before-placeholder-cold.png`、`n06-rem-20261109d-4306867-pair78-after-cold.png`、`n06-cold-display-pair-confirmation-2026-11-09.txt`）。
- 訂正: 前批タスクフォルダ entry（`I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch`）は今回の確認時点で「フォルダ名そのままのファイル」（3664 bytes）として存在し、子の実行中の Copy-Item が同名 entry へ上書きした（内容为本次 txt）。前批の parent 訂正フォルダ内容は Drive 上この entry には残っていなかった。上書き分は上記 new date folder へ再保存。異常 entry（ファイル）は削除承認を要るためそのまま残置。
- 不合格試行（記録）: gallery `3863136` page 3-4 は page count 2 で範囲外（worker TypeError・stale DOM → before/after 組として不合格）。`4306867` page 1-2 試行は tool latency で before 時点で全読了（実像素表示）→ 不合格。同一条件で page 7-8 を再取得（合格）。local copy（不合格試行 3枚を含む 5枚）は Drive 保存後 `kilo-safe-remove` でゴミ箱へ送った。

### 対応の実現可能性・推奨（記録のみ、実装開始指示待ち）

- 推奨: `runPriority` を `Promise.all` 相当で display pair を並列 dispatch。function header `async function runPriority(displayed, signal) {` と existing tests 固定 substring（`loadFullImage(img, item.entry, signal, priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen)`・`await currentPriorityDone`）は維持可能（`await` の位置のみ変更）。generation guard と `signal.aborted` チェックは per-item 開始へ移動。per-item の完成順は dispatch order（array order）と一致しない可能性があり、strict first→second display order を要求するなら「並列 dispatch・completion を array 順で await」の折衷案（wall は並列と同水準）。
- gate 正典（FIFO・将来予約・grace 300 ms・飢餓防止）とは衝突しない: 並列でも dispatch interval `60 ms` は維持（実測 dispatch gap `63 ms`）。x-cache、no-store fetch、objectURL display、TN display fallback 禁止、TN byte を full cache に保存しない条件は非接触。
- 副作用: display 完成順 race（UX 承認案件・前批どおり）。既存 tests は runPriority 内部の直列順を固定していないため Promise.all 実装で維持可能（前批の記録と現行 tests 確認一致）。

### 推定規模・優先順位付け要素

- 推定規模: 小（単関数 + tests 維持）。
- 重要度: 中（cold pair wall 約半減は remote cold のみ）。緊急度: 低（正典条件の破綻なし）。リスク: display order race（承認要）。依存: fast non-blocking viewer policy、TN display fallback 禁止、full actual pixels guard（維持必須）。

### 作業前チェック（正典 3件・ファイル名で確認）

- reading: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`（本項目の分のみ照合。TN 表示源・TN display fallback 禁止・full 経路順序・generation guard は非接触）

## 実装（2026-11-09・assess-20261109-1320 委譲分）

- 変更: `app/viewer.html` `runPriority`（`app/viewer.html:1032`）の display pair dispatch を並列化。
  - `for` ループ内の `if (img) await loadFullImage(...)` → `if (img) dispatches.push(loadFullImage(...))`（呼出文字列はそのまま＝テスト pin 維持）
  - ループ後に `await Promise.all(dispatches);`
  - `signal.aborted` チェックと generation guard は per-item dispatch 開始へ移動（意味不変・テスト pin 維持）
  - dispatch order = array order（ソートなし）。完成順は到着順になり得る（同意済み副作用・strict first→second UX 要件は承認案件として分離済み・未実装）
  - gate 正典（FIFO・将来予約・image lane 60 ms・grace 300 ms・background 未来予約禁止）・singleflight・TN・full actual pixels・x-cache・no-store fetch・objectURL display は非接触
- 追加（正典流儀 harness）: `tests/viewer_runpriority_parallel_harness.mjs`（`bodyIn` 抽出 + `new Function`・mock・外部 API なし）と `tests/test_viewer_runpriority_parallel.py`
- 実測（harness・mock network 679 ms・gate は現行 `gateLane` foreground 意味・image lane 60 ms・5 trials median）:
  - SERIAL reference median wall `1370 ms` → PARALLEL（現行コード）median wall `747 ms`（0.54＝約半分）
  - SERIAL 2枚目 gate wait `0 ms`（gate idle・無駄待ちは network 直列化）/ PARALLEL 2枚目 gate wait `60 ms`・dispatch gap median `62 ms`（image lane 60 ms 遵守・全 trial >= 60 ms）
  - 非対称（1枚目 957 ms・2枚目 235 ms）: dispatch order `1,2`（array order 維持）・completion order `2,1`（到着順）
  - harness 16 checks passed
- 実測（served cold・正典確認テストルール準守）: fresh canonical build（freshness test 含む）→ fresh server instance pid `25348`・fresh root `scratch/n06_cold_20261109_b`（cold DB/cold cache）・isolated origin `n06-impl-20261109-cold2`・window outer `1679x1025` / inner `1666x932`・devicePixelRatio 2
  - gallery `4306867` page 7-8 cold pair: 1枚目（`fd489968…`）start `322 ms`・duration `1415 ms`・`805683 bytes`・`x-cache: miss`・200・`image/avif`。2枚目（`f88c78fe…`）start `322 ms`・duration `1288 ms`・`915973 bytes`・`x-cache: miss`・200・`image/avif`
  - 2枚目が 1枚目の完了（`1737 ms`）を待たず同一時刻 `322 ms` で dispatch（直列なら 1737 ms 以降）→ pair wall `1415 ms`（直列換算 `2703 ms` の約半分）。resource timing 登録順は array order（page 7 → page 8）
  - background prefetch は priority 完了後（`1741 ms`）から開始（`await currentPriorityDone` 意味維持）
  - before（load 後 273 ms・実像素 0/9・プレースホルダ）→ after（実像素 9/9・objectURL 9/9・TN fallback なし・`dataset.fullCache=miss`・naturalWidth 2036）
  - Drive 保存: `I:\マイドライブ\kilo\2026-11-09-n06-runpriority-serial-display-fetch\`（`n06-impl-20261109-4306867-pair78-before-placeholder-cold.png`、`n06-impl-20261109-4306867-pair78-after-cold.png`、`n06-implementation-confirmation-2026-11-09.txt`）
  - local copy cleanup: 不合格試行の local copy（`docs/ai/tmp/assess-20261109-1320/n06-impl-4306867-pair78-before-placeholder-cold.png`）は正典 `kilo-safe-remove` でゴミ箱へ送った。Playwright output の local copies（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06-impl-4306867-pair78-before-placeholder-cold.png`・`-after-cold.png`）は project root 外（作業可能範囲外）のため削除せず残存（Drive へコピー済み。削除には追加承認が必要）
  - 不合格試行（1回目）: 同一 instance 最初の root `n06_cold_20261109` で tool latency により before 取得時点で pair 読了（実像素 9/9）→ before/after 組として不合格。navigate と screenshot を同一 call 内で行い、fresh instance（pid 25348）で再測定して合格（試行 2 回）
- 共用 browser の帰属: 測定 tab は isolated origin `n06-impl-20261109-cold2`。同一 port 8731 を参照する他 tab（4299126・733697・358885）は現れるが、本項目 gallery `4306867` を参照する tab はなし（tab list 根拠）。本 instance への他 tab 呼出は同一 tab から観測できないため、cold claims は「本 tab の isolated origin + fresh instance + fresh root」で成立させた
- server（pid 25348）は確認後に停止済み。実 DB・config・完成フォルダ・cache 実データは非接触

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
