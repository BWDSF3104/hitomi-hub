# P3 #15 シングル / 見開き / フルスクリーンのボタン化

## 現在の実装状況

- 表示モードは `app/ui.html` の `<select id="view-mode">` のみ。ボタンはない。
- `setViewMode(mode)` が単一の source of truth として `viewMode` を更新し、`select.value` を同期し、`applyFullscreen()`、`renderReader()`、`startFullPrefetch()` を呼ぶ。
- keyboard operation は `d`、`s`、`f` を受け付ける。
- `#prev` / `#next` は viewer 用の prev/next として `movePage(delta)` を呼ぶ。
- current page indicator は `input#pagenum`。
- fullscreen state は `body.fullscreen` class と `select.value` にしか反映されない。

## 実現可能性

- 高い。
- 完全に UI 側で可能。
- `setViewMode()` は既存 single source of truth として機能しており、keyboard operation はそのまま維持できる。

## 制限事項

- 通常なし。ただし以下に注意。
- keydown handler が `INPUT`、`BUTTON`、`SELECT` を除外するため、ボタン click 後 focus が BUTTON に残ると `d/s/f` が効かない。
- `f` は現在 one-way で fullscreen を入る方向のみ。
- fullscreen entry は user gesture 必須。button click は transient activation を満たすため安定性上有利。
- `doubleActive()` は `window.innerWidth >= 720` に依存する。
- 現在の mode name は `double`。`spread` に rename すると `displayModeFor()`、`readerClass()`、string tests、keyboard handler に影響。
- #18 の RTL 方向切替は別項目。pair logic と route/hash は logical order を維持すべき。

## 推奨実装案

- `#view-mode-buttons` を `#reader-controls` に追加。
- 各ボタンに `data-mode="single|double|fullscreen"` を持たせ、`setViewMode(mode)` を呼ぶ。
- `aria-pressed` を `viewMode === mode` で更新。
- fullscreen ボタンは `#fullscreen-toggle` として独立 toggle にし、layout mode と presentation state を分離する。
- `#view-state` 行を `#strip-group-status` と同型で追加。
  - mode
  - current page
  - pair page indicator
  - fullscreen state
  - display mode fallback
- `movePage()` の境界 false を UI に反映する。`#prev` / `#next` を disabled にするか、`#note` に「page out of range」を表示。
- keyboard compat を維持するため、button click 後 `blur()` するか、button focus を例外化する。

## その他実装案

- #11 と統合し、`#pagenum` input を readonly indicator と prev/next に置き換える。
- `#view-state` に `page X/Y` を表示する。
- 検索側の `#search-page-status` と同じ表示書式にすると UI 一貫性が上がる。

## 検証方法

- `app/viewer_test.go` に `TestEmbeddedUIHasViewModeButtonsAndState` を追加。
- `data-mode`、`aria-pressed`、`#view-state`、`renderViewState`、resize handler、`f` toggle、keydown guard exception を assert。
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で Playwright screen verify。
  - button click で reader class と `aria-pressed` が変わる
  - button click 後も `d/s/f` が有効
  - `page X/Y`、fullscreen state、fallback display が更新される
  - narrow で single fallback、landscape entry で single landscape
- スクリーンショットは `scratch/hitomi-usage/`。

## 採用決定

Recommended implementation is adopted. Add `#view-mode-buttons`, `data-mode`, `aria-pressed`, `#view-state`, and handle button focus with `blur()` or keydown guard exception. Fullscreen toggle can be separated from layout mode.