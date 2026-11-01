# P3 #13 検索結果の前後ページボタン

## 現在の実装状況

- 検索フォームは `app/ui.html` に `#query`、`#lang`、`#page` input と `#search-submit` のみ。
- `runSearch()` が `#page` を読み、`/api/search?query=…&lang=…&page=…&limit=25` を呼ぶ。
- `saveSearchState()` は `localStorage` に `{query, lang, page, ids, total}` を保存する。
- `parseSearchHash()` と `hashchange` により `#/q/<query>/page/<page>` から再検索できる。
- `restoreSearchState()` は reload 後に page を復元する。
- 件数表示 `#count` は total 件と page のみ。位置表示は #11 の範囲。

## 実現可能性

- 高い。
- サーバー変更は不要。
- `total` は `SearchHTML()` が返す全件数、`galleriesPerPage = 25` を基に client 側で `totalPages = ceil(total / 25)` を算出できる。

## 制限事項

- 通常なし。ただし運用上の制約がある。
- ページ移動は毎回 `runSearch()` 経由の外部検索になる。nozomi 全体取得と rate gate `1000ms` の影響で連続移動は低速になりやすい。
- keydown handler は gallery open 状態や `INPUT` を除外するため、検索画面だけでは矢印キーが効かない。
- `history.replaceState` のためブラウザバックでページ履歴は回復しない。
- 検索カードの `data-page` は固定で、gallery を開くと route が gallery hash に変わる。

## 推奨実装案

- `#search-prev`、`#search-next`、`#search-page-status` を `#page` input の隣に追加。
- `searchTotalPages(total) = Math.max(1, Math.ceil(total / 25))` を client 側で算出。
- `setSearchPage(delta)` で `1..totalPages` に clamp し、`#page.value` 更新、`saveSearchState()`、route 更新、`runSearch()` を行う。
- 境界ではボタンを `disabled` にする。
- 二重実行を避けるため、`location.hash` 代入経由の `hashchange` と `runSearch()` 直接呼び出しを同一処理に混合しない。

## その他実装案

- `query + lang` の全文 ID 配列をメモリ cache し、prev/next は client 側 slice だけで描画する案。#10 と #17 の検索 cache 検討と共通化できる。
- `pushState` を使い、ブラウザバックで検索ページ履歴を復元する案。#11 の位置表示と統合しやすい。

## 検証方法

- `app/viewer_test.go` に UI 文字列テストを追加。例: `TestEmbeddedUIHasSearchPrevNextButtons`。
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で `#/q/furry/page/2` の prev/next、境界 disabled、route 維持を確認。
- CLI parity: `hub.exe search --term "furry" --lang all --page 2`。

## 採用決定

Recommended implementation is adopted. UI-side `#search-prev` / `#search-next` / `#search-page-status` with client totalPages and clamp is the implementation target. Search result memory cache may be added later as an optimization, but it is not adopted as the primary implementation.