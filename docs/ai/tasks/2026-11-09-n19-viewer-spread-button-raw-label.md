# viewer.html 起動時の spreadButton が生値表示

**Status**: 完了
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
- 参照（変更なし）: `docs/ai/tasks/2026-11-03-viewer-keyboard-spread-direction.md`。作業前チェック読了 decision: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。正典文書間の状態記述矛盾なし（実装実行で再読し確認）。

### 実装（assess-20261109-1320 実行・2026-11-09）

- 採用: ユーザー同意済みの推奨案どおり、`app/viewer.html:1543` を `if (spreadButton) spreadButton.textContent = spreadDirection;`（生値）→ `if (spreadButton) spreadButton.textContent = spreadButtonText(spreadDirection);`（1行のラベル映射）へ変更。内部値 `spreadDirection`・保存値意味・`spreadButtonText()`（日本語映射）意味・テスト pin（`app/viewer_test.go`）は変更なし。根拠: 正典流儀 harness 13 checks + served 画面実測（下記）。
- 回帰テスト追加（正典流儀・`tests/harness_lib.mjs`・`bodyIn` 抽出 + `new Function`・mock・外部 API なし）: `tests/n19_spread_button_label_harness.mjs` + `tests/test_n19_spread_button_label.py`。確認内容: 表示ラベルが日本語映射（rtl=右→左・ltr=左→右）、内部値が生値 "ltr"/"rtl" のまま保たれる、保存値が生値のまま、切替動作・pin 不変。
- 試行: 実装 1 回で合格。harness 初回実行 2 回失敗（harness 側の欠陥: `bodyIn` openPattern が `syncViewModeButtons` 内出現で曖昧→起動 block 用 pattern を `\nif (spreadButton) spreadButton` で明確化、`SPREAD_DIRECTION_STORAGE_KEY` 未定義→注入）→修正後 3 回目で 13 checks passed。項目合格条件の再検証試行は 1 回（不合格なし）。

## Verification

- `python -m pytest tests/ -v` → 60 passed（10.86s、`tests/test_canonical_build_freshness.py` 含む）。正典 build freshness 合格。
- 実機画面確認（正典 served UI・fresh server instance・isolated origin clean storage・実 window size 1666x932・要素 rect (212,126,67,34)・切り取りなし・実 load・実 reload）: 初回描画フレームは既定「右→左」、ltr 保存時「左→右」。生値ラベルの描画は 0 回。判定: 表示バグとして不合格（再現せず）／コード整合性として合格。
- 画面キャプチャ保存先: `I:\マイドライブ\kilo\2026-11-09-n19-viewer-spread-button-raw-label\`（`n19-spread-button-default-load-20261109.png`、`n19-spread-button-ltr-storage-load-20261109.png`、`spread-button-first-paint-confirmation-2026-11-09.txt`）。操作で表示が変わる項目ではないため前後1組は不要（ルール第5条、1枚ずつ）。ローカルコピー（プロジェクトルート）は正典 `kilo-safe-remove` でゴミ箱へ送信済み。Drive 側は保存済み。
- 注記: 確認ハーネス初回試行の console error 1件（`documentElement` null での observe 失敗）はハーネス由来、アプリ動作ではない。
- `go vet . && go test -count=1 -timeout 300s .`（app）は app 変更なしのため適用なし。コード変更なし。

### 実装後の検証（assess-20261109-1320 実行）

- 正典 build（fresh・現 app から）: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 合格。
- 正典検証: `python -m pytest tests/ -v` → 65 passed（26.36s、`test_canonical_build_freshness.py` 合格、`test_n19_spread_button_label.py` PASSED）。`go vet . && go test -count=1 -timeout 300s .`（app）→ ok hitomi-hub/app 31.326s。
- harness（正典流儀・mock・外部 API なし）: `node tests/n19_spread_button_label_harness.mjs` → 13 checks passed（表示=日本語映射・内部値=生値維持・保存値=生値・切替後ラベル対応・pin 不変を決定論確認）。
- served 画面確認（正典ルール準守・fresh canonical build・fresh instance・isolated origin `n19-confirm-20261109`（測定前 storage null 確認）・window size inner 1666x932 実測・cold hygiene 条件1–3充足（起動前に hub.exe プロセス 0・port 8731 listen 0）): before（操作前）= button ラベル「右→左」・view-state「開き方向 右→左」・要素 rect (211.5,126,67.2,34) 切り取りなし → `#spread-direction` click → after（操作後）= ラベル「左→右」・view-state「開き方向 左→右」・localStorage 保存値 生値 "ltr"・aria-pressed false。before/after 1組（同一 window size・同一被写体範囲 full page・同一取得方法）。生値ラベルの描画 0 回。切替動作確認済み。console error は favicon.ico 404 のみ（アプリ動作ではない）。判定: 合格。
- Drive 保存（フォルダ＝項目 slug `2026-11-09-n19-viewer-spread-button-raw-label`）: `I:\マイドライブ\kilo\2026-11-09-n19-viewer-spread-button-raw-label\` へ `n19-spread-button-before-20261109.png`・`n19-spread-button-after-20261109.png`・`spread-button-served-before-after-confirmation-2026-11-09.txt` 保存。ローカルコピー（プロジェクトルート）2枚は正典 `kilo-safe-remove` でゴミ箱へ送信済み。Drive 側は保存済み。スクリーンショット視覚確認: before「右→左」/after「左→右」。
- ブラウザ: 保存・記録完了後閉じた（isolated tab・page close）。server（fresh instance）は確認後に停止。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
