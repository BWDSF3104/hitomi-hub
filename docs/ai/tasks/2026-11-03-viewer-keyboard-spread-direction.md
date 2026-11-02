# キーボード左右操作を見開き方向に合わせる

**Status**: 完了
**Date**: 2026-11-03
**Next Action**: keydown の ArrowLeft/ArrowRight を spreadDirection に応じてforward/backに変換

## 意図 / 背景

ユーザー報告: 見開きの向き（spread direction）とキーボード左右操作があっていない。RTL（既定）ではページは右→左に読むが、keydown の ArrowLeft/ArrowRight は常に `movePage(-1)` / `movePage(1)`（＝LTR の物理方向）で、物理キー方向が reading order と逆になる。見開き方向に合わせて左右キーの forward/back を変えるべき、という指示。

作業前チェック（フル画像・viewer タスクの必須）: 以下の decision を読了 —
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

他の進行中タスク 2 件（scan-layer-move / external-code-review-findings）と競合しないため本依頼で並行継続。

## 調査結果 / 変更内容

- 原因: `app/ui.html` の keydown で `ArrowLeft` が常に `movePage(-1)`、`ArrowRight` が常に `movePage(1)`（＝LTR の物理方向固定）。既定の RTL では読む方向が右→左なので、物理キー方向が reading order と逆になっていた。
- 変更: `app/ui.html` keydown を方向対応化 — `ArrowLeft` → `movePage(spreadDirection === "rtl" ? 1 : -1)`、`ArrowRight` → `movePage(spreadDirection === "rtl" ? -1 : 1)`（`app/ui.html:1527-1528`）。forward は常に論理ページ +1（reading order）で、物理左右キーを spreadDirection に合わせて翻字。
- `app/viewer_test.go`: `TestEmbeddedUISeparatesFullscreenStateAndRestoreLastViewMode` の keydown アサーションを新文字列へ更新、`TestEmbeddedUIHasSpreadDirectionToggle` に方向対応アサーションを追加。
- prev/next ボタン（「前」「次」）は論理ラベルであり変更なし。keydown のみ修正。

## Verification

- `go vet .`（app）合格。`go test -count=1 -timeout 300s .`（app）合格（36.53s）。
- 正典 exe 再ビルド → 正典サーバー再起動（`hub.exe --root S:\Documents\Kilo_Commons\hitomi-hub view`、port 8731）。
- 実機 UI 確認（fresh tab・viewport 1280x900・実 search submit・server baseline 一致・gallery 4299126 実 click、実 keypress）:
  - RTL（既定）: ArrowLeft → page 3-4（forward）、ArrowRight → 1-2（back）
  - `r` で LTR: ArrowRight → 3-4（forward）、ArrowLeft → 1-2（back）
  - RTL 単一: ArrowLeft → page 2（forward）
  - reader img 実像素: naturalWidth 2054（full pixels、TN ではない）
- viewer: `/api/image/b17f157a…40fb3c53?format=webp` → 200 / `x-cache: miss` / image/webp / 153458 bytes、`no_full` なし。
- CLI download（実 gallery 4299126・limit 3・no-vt）: `attempted 3 == written 3`。
- `python -m pytest tests/ -v` 25 passed（canonical build freshness 含む）。
- screenshot: `docs/ai/tmp/2026-11-03-keyboard-spread-rtl.png` → `I:\マイドライブ\kilo\2026-11-03-viewer-keyboard-spread-direction\`（タスク別フォルダ）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
