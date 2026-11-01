# P3 #16 タブタイトルを閲覧中の作品に

## 現在の実装状況

- `app/ui.html` の `<title>` は静的文字列で、`document.title` を更新する処理はない。
- gallery メタは `setGalleryMeta()` で `#gallery` へ HTML 描画されており、`g.title`、`g.japanese_title`、`language`、`type` を取得済み。
- cache gallery では `showGalleryMetaFromCache()` と `renderGalleryLoading()` が `searchCardCache` の title を即表示する。

## 実現可能性

- 極めて容易。
- 既存の gallery title / japanese_title を `document.title` へ代入するだけで成立。
- server 側変更は不要。

## 制限事項

- `ui.html` は `app/viewer.go` の `//go:embed` で exe に同梱されるため、変更には `hub.exe` 再ビルドと canonical server 再起動が必要。
- `newTabMode` の場合、タブごとに独立した `document` を持つため、各タブが own title を更新する設計で問題ない。

## 推奨実装案

- `setGalleryMeta()` 末尾で `document.title = japanese_title || title` を設定する。
- 可能なら接尾辞として「 - hitomi hub viewer」を付ける。
- `showGalleryMetaFromCache()` と `renderGalleryLoading()` の loading 表示にも同じタイトル規則を適用する。
- route 表示や hashchange でのフォールバックは `title || id` とする。

## その他実装案

- タブタイトルを gallery id だけにする最小案。
- reload 後に `restoreSearchState()` から gallery hash を開いた場合も `openGallery()` 経路を通るため、追加処理は不要。

## 検証方法

- `app/viewer_test.go` の embedded UI string test に、`document.title` 更新と `japanese_title` を使うことを assert するテストを追加。
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で Playwright 表示確認し、スクリーンショットを `scratch/hitomi-usage/` に保存。

## 採用決定

Recommended implementation is adopted. Update `document.title` in `setGalleryMeta()`, cache display, and loading display using `japanese_title || title`.