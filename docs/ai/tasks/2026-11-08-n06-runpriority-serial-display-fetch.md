# N-06: runPriority が表示中の 2 枚を直列取得している

**Status**: 進行中
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
  - Google Drive 保存: `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06e-before-3863136-cold.png`, `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06e-after-3863136-cold.png`, `I:\マイドライブ\kilo\2026-11-08-n06-runpriority-serial-display-fetch\n06-cold-display-pair-confirmation-2026-11-08.txt`。
  - local copy cleanup: project scratch の confirmation text と invalid before screenshot は `kilo-safe-remove` でゴミ箱へ。Playwright output local copies（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-before-3863136-cold.png`, `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n06e-after-3863136-cold.png`）は正典 `kilo-safe-remove` が project root 外を削除不可のため削除できなかった。
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

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
