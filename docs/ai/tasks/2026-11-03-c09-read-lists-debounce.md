# C-09: /api/read と /api/lists の debounce 未実装

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（commit/push 済みで close）。実装: 250ms debounce・重複 post skip・window 内 fetch 共有・generation guard 維持。検証: mocked fetch harness 16 checks・固定テスト追加・実機 page 移動 5 回→post/list 各 1 回（screenshot Drive 保存）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/ui.html:1065-1076`（`runBackgroundReadAndLists`）は showPage ごとに `/api/read` + `/api/lists` を呼び、`openGallery`（`app/ui.html:1154,1165,1180`）と search（`app/ui.html:1416`）も `showLists()` を呼ぶ。debounce なし。対応: 同一 gallery/page の重複 post skip、list 表示の debounce（例 250ms）。合格条件: 連続 page 移動で post/list 回数が 1（mocked fetch で確認）。推定規模: 小。

### 実装設計（実施前確定）

- 行番号は実施時点の現物で確認済み（C-06/C-07 等の行移動で `runBackgroundReadAndLists` は `app/ui.html:1152-1163`、`showLists` は `1281-1291`、search の `showLists()` は `1534`）。内容（呼び出し構成・debounce なし）は記録と一致。
- `runBackgroundReadAndLists`: 1 回の呼び出し即 post ではなく、`clearTimeout` + `setTimeout(250ms)` の debounce に変更。同一 gallery/page の再表示（同一 key `current.id + "/" + current.page`）は重複 post を skip（key は post 成功後に記録）。timer 実行時・then 実行時ともに `showGen !== showGeneration || openGen !== openGeneration` の既存 guard を維持（弱めない）。
- `showLists`: list 表示の debounce（250ms）を内部化。window 内conc Calls は shared promise で resolve 後、window あたり fetch 1 回（in-flight 共有 guard）。guard（`showGen !== undefined && ...` の前・後 guard）と render（`targetAttr()`）は維持。これにより openGallery（`await showLists(showGeneration, gen)`）・search（`await showLists();`）・fav/complete handler の呼び出しは全て debounce path を経由。呼び出し側の文字列（固定テスト `TestEmbeddedUIProtectsAsyncReaderWrites` の `await showLists(showGeneration, gen)` 要件）は変更しない。
- showPage の display path（`await post`/`await showLists` なし）不変。generation guard（showGeneration/openGeneration/renderGeneration）・objectURL display path（no-store fetch・`x-cache` 判定・`URL.createObjectURL`・`img.src = objectUrl`）・TN display fallback disabled・`tn` を reader 表示源にしない規則は一切変更しない。debounce は post/list の回数抑制のみ。
- 読了 decision（作業前）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。
- 合格条件と検証: (1) mocked fetch harness（`tests/read_lists_debounce_harness.mjs`・既存 `tests/strip_rebuild_dom_harness.mjs` の流儀、node+mocked fetch）で 連続 page 移動（window 内 5 回）→ `/api/read` post 1 回・`/api/lists` fetch 1 回、同一 gallery/page 再表示で重複 post skip、stale generation guard 維持、window 内 showLists 並行呼び出し fetch 1 回、full image display path（objectURL・x-cache・TN display fallback disabled）不回帰を固定。pytest  wrapper `tests/test_read_lists_debounce.py` 追加。(2) Go 固定テスト `TestEmbeddedUIDebouncesReadAndListsToSingleCall`（`uiPage.ReadFile("ui.html")` 文字列固定、C-07 同型）。(3) 実機: canonical exe rebuild 後 canonical server（既定 `http://127.0.0.1:8731`・正典 dataDir、テストは scratch dataDir `docs/ai/tmp/assess-20261103-1220/`）で fresh tab、fetch 計装（/api/read・/api/lists 計数）、連続 page 移動 → post/list 各 1 回を実測、reader actual pixels（objectURL blob・naturalWidth>0）・`/api/image` 200・`x-cache` を確認、screenshot を `I:\マイドライブ\kilo` へ保存。

## Verification

- 作業前読了 decision: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。
- mocked fetch harness（node・既存 `tests/strip_rebuild_dom_harness.mjs` 流儀の `tests/read_lists_debounce_harness.mjs`）: 16 checks passed — 連続 page 移動（window 内 5 回）→ `/api/read` post 1 回・`/api/lists` fetch 1 回、post は latest gallery/page（page 6）を指す、同一 gallery/page 再表示の重複 post skip、page 変更で post 再開、stale showGeneration の debounce 前 return、window 内 showLists 並行呼び出し fetch 1 回、full image display path（no-store fetch・objectURL・`x-cache`・TN display fallback disabled）不回帰。
- Go 固定テスト `TestEmbeddedUIDebouncesReadAndListsToSingleCall`（`uiPage.ReadFile("ui.html")`）追加。初回失敗の原因分析: clearTimeout/setTimeout の順序 assertion が block 全体参照で `debounceLists` 内の `setTimeout` を先に見ていた。テスト側を `runBackgroundReadAndLists` body slice と `listsDebounce` 側順序 assertion へ分離して再検証し合格（無分析反復なし）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet clean・ok 67.079s（C-06/C-07 回帰テスト合格維持）。
- canonical exe rebuild（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）後、root で `python -m pytest tests/ -v`: 40 passed（`test_canonical_build_freshness.py` 合格、新テスト `tests/test_read_lists_debounce.py` 含む）。
- UI 実機確認（canonical exe・scratch dataDir/root `docs/ai/tmp/assess-20261103-1220/`・既定 `http://127.0.0.1:8731`・fresh tab、fetch 計装 = window.fetch wrapper で /api/read・/api/lists 計数、x-cache path 通過維持）:
  - gallery 4302149 open: reader actual pixels（objectURL blob、naturalWidth 623）
  - 計数リセット後、連続 page 移動 5 回（ArrowLeft 5 回 dispatch＝同一 debounce window 内バースト）→ `/api/read` post **1 回**・`/api/lists` fetch **1 回**、route/page = page 6 反映
  - `x-cache` 判定維持: `/api/image/ea5c4432cad78cf53f75f5b6229227e88db5edfca7ca5771eee619895335da07?format=avif&gallery=4302149` → 200・`x-cache: miss`
  - burst 後 reader display objectURL（blob:）・naturalWidth 5120。TN を reader 表示源に不使用、TN display fallback path 不変
  - screenshot 保存: `I:\マイドライブ\kilo\2026-11-03-c09-page-move-debounce.png`（実測値 overlay 込み）
- server 状態: 開始時 稼働なし（8731 接続確認 False）→ scratch dataDir で起動して確認 → 確認後停止（開始時状態へ復元）。canonical exe は再 build 済み（新 ui.html を含む）。
- 他項目への影響: 既合格項目（C-06/C-07）の固定テスト合格維持。`showLists` の debounce 化により open/search での list 表示は最大 250ms 遅延（設計通り・display path 不変）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
