# UI改善実装: CSSトークン/操作状態/日本語ラベル（2026-11-05検討の承認範囲）

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: search.html/viewer.html へトークン化・focus/hover・日本語フォントスタック・aria・sticky pager・inputmode・日本語ラベル・view-mode単一化・キーガード対称化・reader aspect追従を実装しテスト再アンカー

## 意図 / 背景

- 2026-11-05 の UI 検討（見やすい・使いやすい・直感的・モダンの4要件）で、ユーザーが「不可・禁止以外」の改善を承認。加えて日本人ユーザー向け最適化として、閲覧画面の英語表記ラベルの日本語化をスコープ追加。
- 作業前チェック（viewer 触るタスクの必須）として以下を読んだ: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。
- 対象外（禁止条のまま維持）: Google Fonts（外部依存・移植性）、TN を源とする表示改善、card height 契約（130px）変更、full 判定（x-cache / no-store fetch / objectURL 表示 / generation guard）の弱化、永続先移動。

## 調査結果 / 変更内容

- `app/search.html` へ CSS トークン、focus-visible、hover/active、`@keyframes ui-pulse`、`data-busy` skeleton、`form#search` input/button focus 調整、`role="status" aria-live="polite"`、`inputmode="numeric"`、日本語ラベル（`リセット`、`読み込み中`、`ページ`、`合計`、`キャッシュ`、`サーバー`、`計測`、`ジャンプ`、`ジャンプエラー`、`リセットエラー`）を入れた。
- `app/viewer.html` へ CSS トークン、focus-visible、hover/active、reduced-motion、sticky pager、単一表示の aspect-fit、view-mode 単一化、segmented button、`syncViewModeButtons()`、`viewLabel()`、`spreadButtonText()`、ARIA、日本語ラベルを入れた。
- `window.addEventListener("keydown", async (e) => {` の制御順序を `f` / `Escape` の後に `ArrowLeft` / `ArrowRight`、その後に control-focus guard へ変更し、ボタン/セレクト focus でもページ送りキーが機能する状態へ揃えた。
- `app/viewer_test.go` の exact anchors を updated UI strings に再アンカーした。主な更新点:
  - `title='ローカル'`
  - shared tokenized body rule
  - `data-reset='all'>リセット`
  - `実表示ピクセル ` / `ローカル直接 ` / `objectURLキャッシュ ` / `フル画像なし ` / `TN表示フォールバック禁止`
  - `読み込み中` / `ギャラリー読み込み: `
  - `id="view-mode-buttons"` / `data-mode="single"` / `data-mode="double"` / `data-mode="fullscreen"` / `syncViewModeButtons()` anchors
  - `直前レイアウト " + viewLabel(lastViewMode)` / `全画面 " + viewLabel(fullscreenState)`
  - `失敗を再試行`
  - `位置: ` exact anchor
  - ArrowLeft が control-focus guard より前にある order assertion
- Harness anchors を updated UI strings に再アンカーした:
  - `tests/strip_rebuild_dom_harness.mjs`: `" | TN表示フォールバック禁止"`
  - `tests/read_lists_debounce_harness.mjs`: `TN表示フォールバック禁止`
  - check labels は変更しない。

## Verification

- Go app: `go vet . && go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 26.455s`
- Python tests: `python -m pytest tests/ -v` → `53 passed in 9.95s`
- Canonical build freshness: rebuilt canonical exe:
  - command: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
  - `tests/test_canonical_build_freshness.py` passed after rebuild.
- Canonical served UI confirmation at actual window size `1678 x 1025`, content viewport `1665 x 932`, `devicePixelRatio 2`:
  - Search page snapshot showed Japanese UI labels: `実行中…`, `前`, `次`, `ページ`, `合計`, `保存`, `名前`.
  - Viewer page snapshot showed Japanese UI labels: `表示モード`, `単一`, `見開き`, `全画面`, `開き方向`, `表示: 見開き`, `位置: 1 / -`, `失敗を再試行`, `前のグループ`, `移動`, `次のグループ`, `読み込み中`.
  - Served screenshot subjects are Playwright browser viewport images:
    - `scratch/p3_canonical_fresh/2026-11-05-ui-polish-viewer.png`
    - `scratch/p3_canonical_fresh/2026-11-05-ui-polish-search.png`
- CLI download real-gallery check:
  - `attempted=1`, `written=1`, `skipped=0`, `gallery_id=4252697`.
  - command used canonical `hub.exe download` with scratch root.
- Viewer `/api/image` real-gallery check:
  - request: `/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=webp`
  - result: `200`, `x-cache: cache`, `bytes=210784`, `content-type=image/webp`.

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（`task_manager.py end` で削除済み）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（該当なし）
