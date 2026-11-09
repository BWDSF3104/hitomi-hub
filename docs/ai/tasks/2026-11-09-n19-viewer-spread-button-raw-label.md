# viewer.html 起動時の spreadButton が生値表示

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

新規発見のUI表示疑い: 起動時に `#spread-direction` ボタンラベルが生値 `"rtl"` で描画され、`syncViewModeButtons` 呼び出しまで日本語ラベルにならない、という指摘（原文項目 #3）。実挙動を実測で確定する。実行固有ID: `assess-20261109-1320`。

## 調査結果 / 変更内容

- 現在の実装: `app/viewer.html:1505-1506`（body末尾の同期インラインスクリプト）で `spreadButton.textContent = spreadDirection`（生値代入）。markup 静的ラベルは `app/viewer.html:140` で既に「右→左」。正典 served markup に `#reader` は存在する（`app/viewer.html:154`）。
- 起動順序: `loadSpreadDirection()`（1504）→ 生値代入（1506）→ hash なしは `renderViewState()`（1512）、hash ありは `openGallery()`（1178）の cache-miss 経路の `renderGalleryLoading()`（774）経由で `renderViewState()`。いずれも `syncViewModeButtons()`（705-715）を呼び、同一同期タスク内・最初の `await` 前・初回描画前に日本語ラベルへ修正される。
- 実測（正典 hub.exe fresh instance・isolated origin `n19-assess`・Playwright・window 1666x932・MutationObserver+first rAF）:
  - 既定（storage なし）: 静的「右→左」(t=10ms) → 生値 "rtl" 代入 (t=11ms, painted=false) → 同一同期タスク内で「右→左」へ修正 (t=11ms) → 初回描画フレーム = 「右→左」。
  - storage=ltr での reload: 生値 "ltr" 代入 (t=75ms, painted=false) → 同一同期タスク内で「左→右」へ修正 → 初回描画フレーム = 「左→右」。
  - 結論: 生値はどの条件・タイミングでも一度も描画フレームに入らない（ユーザー可见の生値表示は確認されず）。
- 有効性: 一部有効。コード整合性の指摘（1506 が `spreadButtonText()` を使わない）は有効（事実）。ユーザー可见の表示バグとしては実測で無効。
- 対応の実現可能性: 修正は `app/viewer.html:1506` を `spreadButtonText(spreadDirection)` 使用への変更（1行）。または生値代入行を削除し `syncViewModeButtons()` に依存。推奨: `spreadButtonText()` 使用（理論上の例外経路 `!reader` 早期 return — served markup では到達不能 — でも安全）。
- 副作用・影響範囲: 表示文字列のみ。`app/viewer_test.go` にこの行の文字列アサーションなし（`TestEmbeddedUIHasSpreadDirectionToggle` はアサーション変更不要と確認）。価値は衛生修正（起動順序が将来変わり renderViewState が await 後へ回ると初めて可见化しうる Regression 防止）。
- 参照（変更なし）: `docs/ai/tasks/2026-11-03-viewer-keyboard-spread-direction.md`。作業前チェック読了 decision: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。正典文書間の状態記述矛盾なし。

## Verification

- `python -m pytest tests/ -v` → 60 passed（10.86s、`tests/test_canonical_build_freshness.py` 含む）。正典 build freshness 合格。
- 実機画面確認（正典 served UI・fresh server instance・isolated origin clean storage・実 window size 1666x932・要素 rect (212,126,67,34)・切り取りなし・実 load・実 reload）: 初回描画フレームは既定「右→左」、ltr 保存時「左→右」。生値ラベルの描画は 0 回。判定: 表示バグとして不合格（再現せず）／コード整合性として合格。
- 画面キャプチャ保存先: `I:\マイドライブ\kilo\2026-11-09-n19-viewer-spread-button-raw-label\`（`n19-spread-button-default-load-20261109.png`、`n19-spread-button-ltr-storage-load-20261109.png`、`spread-button-first-paint-confirmation-2026-11-09.txt`）。操作で表示が変わる項目ではないため前後1組は不要（ルール第5条、1枚ずつ）。ローカルコピー（プロジェクトルート）は正典 `kilo-safe-remove` でゴミ箱へ送信済み。Drive 側は保存済み。
- 注記: 確認ハーネス初回試行の console error 1件（`documentElement` null での observe 失敗）はハーネス由来、アプリ動作ではない。
- `go vet . && go test -count=1 -timeout 300s .`（app）は app 変更なしのため適用なし。コード変更なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
