# P3 #18 見開きは右から左がデフォルト、また、切り替えボタンも実装

## 現在の実装状況

- `readerItems()` は `[current page, second page]` を返す。
- `renderReader()` は parts join 順のまま `.reader` grid に左から右へ描画する。
- `movePage(delta)` は pair logical order を維持する。
- route は `route(id, page, second)`。hash parse は `parseReaderHash()` が `page-second` を扱う。
- view mode は `setViewMode()` で `single`、`double`、`fullscreen` を扱う。
- landscape entry は `isAlreadyDoublePage()` で single landscape。
- RTL display order / toggle / key は未実装。

## 実現可能性

- 高い。
- logical pair、page、route、prefetch order を維持し、display order だけを反転すればよい。

## 制限事項

- 通常なし。UI 側 client 変更で完結。
- `ui.html` は `//go:embed` で exe に同梱されるため、反映には `hub.exe` 再ビルドと canonical server 再起動が必要。
- `window.innerWidth < 720` の single fallback では方向切替の影響は 1 枚のみ。
- fullscreen double は display direction の影響を受ける設計にする。
- toggle button focus 時に keyboard handler が `BUTTON` を除外するため、key handling を設計する必要がある。

## 推奨実装案

- `let spreadDirection = "rtl"` を追加。
- `setSpreadDirection(dir)` を追加。
- `renderReader()` の display order を `spreadDirection` に応じて反転。
  - default RTL: first page right、second page left
- `#reader-controls` に direction toggle button を追加。
- `localStorage` に `hitomi_hub_spread_direction` を保存。
- keyboard handler に方向切替 key、例 `r` を追加。
- route / hash は logical start page を維持。`page-second` の意味を変えない。
- badge、page number、prev/next の logical meaning を RTL display order に対応させる。

## その他実装案

- CSS `direction: rtl`、または reader grid を `flex` 化し `flex-direction: row-reverse` を使う案。DOM order は変わらない。
- `data-page` selector と strip tests に友好。
- `#view-mode-buttons` の `double` button を toggle で `double-ltr` / `double-rtl` に拡張する案。

## 検証方法

- `app/viewer_test.go` の existing `TestEmbeddedUIHasReaderSpreadAndFullscreen` に方向変数、toggle、keyboard handler を assert。
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で Playwright screen verify。
- 例: `#/g/4252697/page/2-3`。
- double portrait で `readerImageCount: 2`、表示順 left page `3`、right page `2` を確認。
- toggle button click で left/right swap、route `page-second` stable、page meaning stable を確認。
- landscape regression は `reader single landscape` を確認。

## 採用決定

Recommended implementation is adopted. Default RTL display order is implemented by reversing `renderReader()` output order only. Logical pair/page/route/hash semantics remain `page-second`. Add toggle button, localStorage persistence, and key handler.