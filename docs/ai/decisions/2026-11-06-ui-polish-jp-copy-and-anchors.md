# UI改善実装の設計決定（コピー対応・トークン・アンカー再配置）

**Status**: 決定（ユーザー承認 2026-11-05 15:37「不可・禁止以外を承認」。実装開始指示を含む）
**Date**: 2026-11-06
**Task**: `docs/ai/tasks/2026-11-06-ui-polish-jp-optimization.md`

## 作業前チェック（ファイル名明記）

- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md` 読了
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md` 読了
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` 読了

## 決定

1. **日本語コピー対応**（閲覧画面の英語表記ラベル → 日本語。UI 表示文字列のみ、機構文字列は変更しない）:
   - ボタン: single→単一 / double→見開き / fullscreen→全画面 / acquisition→取得 / abort→中止 / retry failed→失敗を再試行 / go→移動 / save→保存 / reset→リセット / strip group 前・次→前のグループ・次のグループ / rtl・ltr 表示→右→左・左→右（`spreadButtonText` マッピング、内部値 `spreadDirection` は rtl/ltr 維持）
   - status 行: position→位置 / view→表示 / last layout→直前レイアウト / spread→開き方向 / page→ページ / source→表示源 / fullscreen→全画面 / actual pixels→実表示ピクセル / local direct→ローカル直接 / objectURL cache→objectURLキャッシュ / no full→フル画像なし / TN display fallback disabled→TN表示フォールバック禁止 / strip group→グループ / acquisition progress→取得 進捗 / loading→読み込み中 / gallery read load→ギャラリー読み込み / page out of range→ページ範囲外 / image load failed→画像読み込み失敗 / read post error→読み状態送信エラー / jump→ジャンプ / perf→計測 / cards→カード / cache→キャッシュ / server→サーバー / total→合計
   - `data-mode` 値・`data-reset` 値・x-cache 値・generation guard・`cache: "no-store"` 等の機構文字列は英字のまま維持（テスト・サーバー判定の正）。
2. **CSS トークン化は両ファイル各自に複製**（decision 2026-11-04 の「ページごとに自己完結」を維持。共有 CSS ファイルは追加しない）。トークン化しない例外（合格条件の正典文字列として exact-match を維持）: policy color（flag-viewed/read/saved/favorite、`.error`、`.card.has-status`）、card height contract（`--search-card-height: 130px`、`-webkit-line-clamp: 9`）、`.cols`/`.side`/`.main` 構造規則、reader fullscreen 規則。
3. **テスト/harness アンカーは表示文字列変更に合わせて再配置**（意味・合格条件は弱めない: x-cache・no-store・generation guard・TN display fallback 禁止・actual pixels 分離はそのまま assert）。CRLF回帰テストとpython wrapper の check label 文字列は変更しない（harness 内部の assert 文字列のみ更新）。
4. **キーガード対称化**: ArrowLeft/ArrowRight を BUTTON/SELECT フォーカスでも有効化（INPUT と modifier キーのガード、f/Escape の優先順位は現行 assert のまま）。d/s/r は control フォーカス時にスキップ維持。
5. **view-mode 単一化**: `<select id="view-mode">` を削除し、`#view-mode-buttons` の segmented control（`data-mode` + `aria-pressed`）に統一。`setViewMode`・`modeButton.blur()`・`f` トグルは維持。
6. **reader aspect 追従**: double の固定高さ（ペア揃え）は維持し、non-double（single/landscape）のみ `height: auto; max-height: var(--reader-page-height)` でレタボックス縮小。fullscreen 規則の exact-match は維持。
7. **アクセシビリティ追加**: `color-scheme: dark`、全操作子 `:focus-visible`、hover/pressed フィードバック（レイアウトシフトなし、≤160ms、`prefers-reduced-motion` 対応）、status 行 `role="status"`、loading 領域 `aria-busy`、`inputmode="numeric"`、`scroll-behavior: smooth`（reduced-motion 時無効）、viewer pager `form#nav` sticky。日本語フォントスタックはローカルフォントのみ（Google Fonts 使用不可・移植性条）。
8. viewer pager sticky と skeleton pulse は中性 box（TN を placeholder 源にしない。TN display fallback 禁止条維持）。

## Supersede 対象

なし（新決定の追記。既存 decision の policy は変更しない）。
