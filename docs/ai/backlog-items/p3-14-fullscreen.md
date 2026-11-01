# P3 #14 フルスクリーン機能

## 現在の実装状況

- 部分実装済み。
- `app/ui.html` の `#view-mode` select に `double`、`single`、`fullscreen` がある。
- `applyFullscreen()` は `document.body.classList.toggle("fullscreen", viewMode === "fullscreen")` を行い、`document.documentElement.requestFullscreen()`、`document.exitFullscreen()` を呼ぶ。
- `fullscreenchange` で Esc 終了時に view mode を `single` へ戻す。
- `readerClass()` は fullscreen double active で `reader fullscreen double` になる。
- `view_mode` select の change、`f` key、CSS `.side` non-display、fullscreen class が既存テストで確認済み。
- 既存記録: `docs/ai/decisions/2026-11-01-fullscreen-double-and-x-cache-status.md`、`docs/ai/tasks/2026-11-01-reader-fullscreen-double-x-cache-status.md`、`docs/ai/usage-memo.md`、`scratch/hitomi-usage/reader_fullscreen_double_xcache.png`。

## 実現可能性

- 可能。
- ブラウザ Fullscreen API を使う案は実現済み。
- 残りは表示領域、表示モード、状態表示、exit 後の mode 復元、フォールバックの設計。

## 制限事項

- 通常なし。ただし以下を設計に織み込む必要がある。
- Fullscreen API は transient activation 必須。load/restore 経由の `requestFullscreen()` は失敗し、現行では `.catch(() => {})` で黙示する。
- fullscreen 対象が `documentElement` のため `h1`、`#reader-controls`、`#nav`、`#strip-controls`、`#note`、`#prefetch-status`、`#thumbs` が残る。
- `doubleActive()` は `window.innerWidth >= 720` に依存する。fullscreen でも幅条件で single fallback がありうる。
- landscape entry は fullscreen でも single landscape。
- `fullscreenchange` 終了時に view mode が無条件で `single` 化し、ユーザーの以前のモードを失う。
- `window.resize` listener がないため fullscreen 切替後 layout/state が stale になりうる。

## 推奨実装案

- `viewMode` と `fullscreenState` を概念分離する。
  - `viewMode` は layout intent: `single` / `double`
  - `fullscreenState` は presentation state: `on` / `off` / `fallback`
- `applyFullscreen()` を user gesture-bound に限定する。
- `lastViewMode` を保持し、fullscreen exit 時に元の layout mode へ復元する。
- `fullscreenerror` と `resize` で reader render / prefetch status を再評価する。
- fullscreen 時の表示領域を reader overlay に寄せるか、UI chrome を明確に non-display 化する。
- フォールバック順序を明示する。
  1. `requestFullscreen()`
  2. `webkitRequestFullscreen()`
  3. CSS only pseudo-fullscreen

## その他実装案

- `document.getElementById("reader").requestFullscreen()` で reader 領域だけ fullscreen 化する案。UI chrome 分離が自然。
- hitomi 本家寄りの CSS only pseudo-fullscreen を主とし、Fullscreen API を任意オプションにする案。native/browser API 依存を増やさない方向に親和。

## 検証方法

- `app/viewer_test.go` の `TestEmbeddedUIHasReaderSpreadAndFullscreen` を拡張。
- `#view-state`、`lastViewMode`、`webkitRequestFullscreen` fallback、`fullscreenerror`、`resize` handler を string test で assert。
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で Playwright screen verify。
- `d/s/f`、`f` による fullscreen double、`.side` non-display、Esc exit 後の mode 復元、narrow fallback を確認。

## 採用決定

Recommended implementation is adopted. Separate `viewMode` layout intent from `fullscreenState`, preserve last mode on exit, add error/resize handling, and make CSS-only pseudo-fullscreen an explicit fallback.