# settings.html の saveSettings 二重定義

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

実行固有ID: assess-20261109-1320（調査のみ・コード変更なし）

### 現在の実装と判断根拠

- `app/settings.html` に `async function saveSettings(event)` が 2 回存在することを実測で確認:
  - settings.html:291（前定義）: `s = data` を更新せず `renderSettings()` する版 → **dead code**
  - settings.html:318（後定義）: `s = data` で server 応答を `s` に反映する版 → **実際に効いている定義**
  - settings.html:334 の `addEventListener("submit", saveSettings)` は hoisting 後の後定義を参照（function declaration は同名スコープで後の宣言が勝つ）。page.evaluate の `saveSettings.toString()` に `s = data;` を含むことを実測（後定義が bound）。
- `s = data` の意味: `renderSettings()` は `s` から描画する（settings.html:202-203）。server が allowlist 正規化・read-only 処理・クランプで POST 応答をリクエストと異なる値にする場合、効いている定義は応答で `s` を更新するため UI は server 確定値を表示する。dead code 版だけ残ると stale な `s` から描画し、server 正規化と UI 表示が食い違う。実測で `s` が POST 後に reassign されることを確認（operational proof）。
- `#settings-status` の所在: 原項目の「HTML に存在せず」は**誤り**。`<div id="settings-status">` は main 内 settings.html:64 に存在（page.evaluate で存在確認）。`#cache-clear-status`（settings.html:89）とは別要素で、saveSettings → settings-status、runCacheClear → cache-clear-status と表示先が分離しており**実挙動上の混同はなし**（実測: 保存操作後 settings-status="Saved"、cache-clear-status=""）。混同リスクは挙動ではなく命名・視覚位置の明快さの問題にとどまる。原項目文は「存在せず」と「main 内にあるので存在はします」で自己矛盾しており、実測は「存在する」側。

### 有効性

- 二重定義: **有効**（実在を実測）。前定義は死んだコード、後定義が有効。
- status 表示先混同: **無効（実害なし）**。表示先は分離され実挙動で確認。原項目の存在否定部分のみ誤りとして訂正。

### 対応の実現可能性と適切な修正方法（推奨）

- 推奨: **前定義（settings.html:291-304）を削除**（死んだコードの除去）。効いている後定義は保持。
- 代替案: 1 定義へ統合（後定義が残るため削除と挙動同一。統合は「1 つの定義に寄せる」意図で可）。
- `viewer_test.go:4944` の string test は `async function saveSettings(event) {` の包含検査のみで、前定義削除後も残存定義で合格。テスト変更は不要（任意で「saveSettings 定義は 1 個・`s = data` を含む」回帰テストを追加可能）。

### 副作用・影響範囲

- 削除は UI スクリプト内関数定義の除去のみ。server 側 `/api/settings`・viewer・download・フル画像経路・TN・正典ルールへの影響なし。
- status 表示先についてはコード変更推奨なし（実害なし）。命名・位置の明快さ改善は任意の保守項目であり UX 意味変更になるため承認が必要（有効性判断は実測済み、実装は現行ゲート内の別判断）。

### 画面確認（確認テストルール適用）

- 正典 rules（`docs/ai/rules/confirmation-test-rule.md`）と decision `2026-11-02-canonical-latest-build-confirmation-rule.md` を辿って適用。cold hygiene 3 条件（fresh canonical build・fresh server instance・isolated origin）を満たして実測。
- 保存先: `I:\マイドライブ\kilo\2026-11-09-n18-settings-save-settings-duplicate\`
  - `n18-settings-save-settings-duplicate-before-20261109.png`（操作前: settings-status 空）
  - `n18-settings-save-settings-duplicate-after-20261109.png`（操作後: settings-status="Saved"、cache-clear-status 空）
  - `settings-save-settings-confirmation-2026-11-09.txt`（手順・実測値・合否）
- window size（fresh target 実測）: inner 1666x932 / outer 1679x1025。人工 resize なし、要素切り取りなし。
- 操作内容: 「設定を保存」を現在の値のまま submit（設定値の変更なし）。before → after で settings-status に "Saved" 表示（cache-clear-status は不変）。

## Verification

- `python -m pytest tests/test_canonical_build_freshness.py -v` → 2 passed（4.15s、canonical exe が現行 app ページバイトを埋め込み）
- fresh canonical build `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（app）→ 成功。canonical server（fresh instance・port 8731）から served 画面を実測。
- 実測結果（served 画面・isolated origin）: `saveSettings.toString()` に `s = data` を含む（後定義 bound）、POST 後 `s` reassign = true、settings-status / cache-clear-status 分離動作。合否: 合格。
- コード変更なしのため `go vet`/`go test` の再実行は該当なし（AGENTS 共通条: docs/ai 変更のみは検証不要）。
- ローカルのスクリーンショットコピー（scratch 経由）は正典 `kilo-safe-remove` でゴミ箱へ送出済み。Drive 側は保持。
- 補足: canonical server（fresh instance・port 8731）を本調査で起動し、Playwright browser の tab を複数兄弟セッションが同一 origin で共有している状況。server はこの子セッションの lifetime で停止するため、兄弟の cold claims で同一 instance 共有による汚染（warm 化）Occurred 点を親へ板で共有。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
