# C-07: resize と fullscreen の generation

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（実装・固定テスト・実操作確認済み。commit 予定）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/ui.html:1508-1514` の resize handler は event ごとに `renderReader()` + `startFullPrefetch()` を呼ぶ（debounce なし）。`renderGeneration`/`showGeneration`/`openGeneration` guard は存在し、weak にしないこと（正典）。対応: resize に小 debounce（例 150ms）を追加し、guard は維持。合格条件: 連続 resize で render 回数が 1、objectURL display path が保われる。推定規模: 小。

### 実装設計（実施前確定）

- 現在の resize handler は `app/ui.html:1631-1637`（C-06 等で行移動済み、内容一致確認済み）。これを debounce 化する。
- 方式: event ごとに既存 timer を `clearTimeout` し 150ms 後の `setTimeout` で 1 回だけ `renderReader()` / `renderPosition()` / `renderViewState()` / `startFullPrefetch()` を実行。連続 event は 150ms 静穏後 1 回の実行に収束。
- generation guard 維持: event 時点で `openGeneration` を capture し、timer 実行時に `resizeOpenGen !== openGeneration` なら return（stale burst が別 gallery の render を追加実行しない）。`renderGeneration`/`showGeneration` guard と objectURL display path（`loadFullImage` の no-store fetch、`x-cache`、`URL.createObjectURL`、`img.src = objectUrl`、TN display fallback disabled）は一切変更しない。
- fullscreen 関連: `fullscreenchange` は状態遷移あたり 1 回の discrete event で burst しないため synchronous のまま（f/Escape gesture 応答と既存固定テスト `TestEmbeddedUISeparatesFullscreenStateAndRestoreLastViewMode` を維持）。fullscreen 中の window resize burst は同一の debounce path で 1 回に収束。
- 合格条件: 連続 resize で render 回数が 1、objectURL display path 保われる。
- 検証: (1) mocked DOM/test harness（`uiPage.ReadFile("ui.html")` の固定テスト）で、handler が renderReader を synchronous に呼ばず debounce timer 内 1 回のみ、`clearTimeout` 先行、150ms、timer 内 openGeneration guard 前 return、objectURL display path 文言不回帰を固定。回帰テスト `TestEmbeddedUIDebouncesResizeRenderToSingleCall` 追加。(2) 実操作: canonical exe rebuild 後 canonical server `http://127.0.0.1:8731` の fresh tab で gallery open、renderReader 計装し連続 resize 5 回→debounce 後 1 回、objectURL（`blob:` src）display、`/api/image` の `x-cache`、status 表示を Playwright で確認、screenshot を Google Drive `I:\マイドライブ\kilo` へ保存。

## Verification

- 固定テスト（mocked DOM/test harness、既存流儀 `uiPage.ReadFile("ui.html")`）: `app/viewer_test.go` に `TestEmbeddedUIDebouncesResizeRenderToSingleCall` 追加。handler が renderReader を synchronous に呼ばず debounce timer 内で 1 回のみ、`clearTimeout` 先行、150ms、timer 内 `resizeOpenGen !== openGeneration` で前 return、objectURL display path（no-store fetch・`x-cache`・`URL.createObjectURL`・`img.src = objectUrl`）と TN display fallback disabled・renderGeneration guard 不回帰を固定。
- 初回失敗の原因分析: 150ms 定数 assertion が handler body slice の外（`const` 宣言が `addEventListener` 前）を参照していた。テスト側の slice 範囲修正で再検証し合格（無分析反復なし）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: clean・ok 67.244s。
- canonical exe rebuild 後、project root で `python -m pytest tests/ -v`: 39 passed（`test_canonical_build_freshness.py` 合格）。
- 正典 Common_Operations tests（task_manager validation 破れなし確認）: 結果は commit 直前の実行結果参照（正典 tool 変更なし）。
- UI 実操作確認（canonical server `http://127.0.0.1:8731`・rebuild 済み exe・fresh tab `?newtab=1`）:
  - gallery open: fresh tab → search(furry) → card click → gallery 4302149（tab #2）
  - 連続 resize ×5（非 fullscreen）→ debounce 後 render 呼び出し 1 回（renderCount=1）
  - `f` で fullscreen on + 連続 resize ×5 → burst あたり render 1 回（burstRenders=1）。fullscreenchange は discrete event のため synchronous 維持（f/Escape gesture 応答・既存固定テスト維持）
  - reader actual pixels: img naturalWidth 623×500 を debounce render 後も保持。objectURL display path 維持（`src` blob:・`sourceType` objectURL）。TN は display source 使用なし（TN display fallback disabled path 不変）
  - `/api/image/698693779485bb99fa93cf61647b03b6fd36514906872b241ba1e88f29a99803?format=avif&gallery=4302149` → 200・`x-cache: miss`
  - status 表示: `view: double・display: fullscreen double・last layout: double・spread rtl・page 1 / 65・fullscreen on`
  - Drive 保存: `I:\マイドライブ\kilo\2026-11-03-c07-resize-debounce-reader.png`（743144 B、初回 burst 後の full page）、`I:\マイドライブ\kilo\2026-11-03-c07-resize-debounce-reader-fullscreen.png`（775552 B、fullscreen 中 #reader element 撮影。fullPage/viewport 撮影は fullscreen 描画中 5s タイムアウト、原因分析後 element 撮影で回避）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
