# P3項目別採用決定

## 適用項目

`#10` から `#18`。項目別検討ファイルは `docs/ai/backlog-items/`。

## #10 検索速度改善の検証

推奨案を採用。

- `Client` に gate wait / network / decode / retry_sleep を分離する計測層を追加。
- `search()` に `SearchHTML`、`recordDiscovered`、JSON encode の phase timing を追加。
- UI は `performance.now()` で search/card fetch 時間を `#perf-status` として表示。
- cache-less runtime root と cache-warm runtime root を同一 query で比較。
- `x-cache` 分布を確認。

外部 API を呼ぶテスト/実測は `hitomi-hub` で許可。詳細は `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`。

## #11 現在表示位置の簡単表示

推奨案を採用。ただし検索結果側は対象外。

- Gallery / viewer 側のみで `#reader-position` を追加。
- `current.page / current.count` を表示。
- pair mode では `current.page-second / current.count` を表示。
- `showPage()`, `setGalleryMeta()`, `renderGalleryLoading()`, `hashchange`, `setViewMode()` から更新。
- `pagenum` input は page input として維持。
- Search position は追加しない。

## #12 まとめダウンロード / フル画像キャッシュ移動

推奨案を採用。

- Browser arbitrary download は採用しない。
- Server-side cache collect を採用。
- `POST /api/cache/collect` で `cache/images/<hash>.<format>` を runtime root 内の `Galleries/<id>/` へ move/copy。
- `dest` は runtime root 内のみ許可。
- `files` と `pageorder` を記録し、structural status を確定。
- UI は bundle/collect button、progress、abort を持つ。
- Single archive bundle は既存 `exportCBZ()` の HTTP 化で考える。

## #13 検索結果の前後ページボタン

推奨案を採用。

- UI 側で `#search-prev` / `#search-next` / `#search-page-status` を追加。
- `totalPages = Math.max(1, Math.ceil(total / 25))` を client で算出。
- `setSearchPage(delta)` で clamp してから route 更新と `runSearch()` を呼ぶ。
- 境界で button disabled。
- Search result memory cache は将来の最適化として扱えども primary implementation にはしない。

## #14 フルスクリーン機能

推奨案を採用。

- `viewMode` は layout intent、`fullscreenState` は presentation state で分離。
- fullscreen entry は user gesture-bound に限定。
- `lastViewMode` を保持し、exit 後に元の mode へ復元。
- `fullscreenerror` と `resize` で reader render / prefetch status を再評価。
- CSS only pseudo-fullscreen を explicit fallback として扱う。

## #15 シングル / 見開き / フルスクリーンのボタン化

推奨案を採用。

- `#view-mode-buttons` を追加。
- `data-mode="single|double|fullscreen"`。
- `aria-pressed` を mode state に同期。
- `#view-state` 行で mode, page, pair, fullscreen state を表示。
- fullscreen は単独 toggle button として扱える。
- keydown guard の BUTTON focus 問題は click 後 `blur()` か例外処理で解決。

## #16 タブタイトルを閲覧中の作品に

推奨案を採用。

- `setGalleryMeta()` 末尾で `document.title = japanese_title || title` を設定。
- loading/cache path でも同じ規則を使う。
- `newTabMode` では各 tab own title を更新。

## #17 同名タイトル作品が複数ある際、言語Japanese以外を非表示

全セット dedup は不採用。

採用案: low-risk anchor 案。

- client 側 `titleGroups` Map を保持。
- key は normalized `japanese_title || title`。
- `language == 'japanese'` best entry を保持。
- display card 上に `同名の日本語版: #id` anchor を出す。
- hide は既定にしない。
- group cache を localStorage に保存し、page change、reload、hash restore で anchor を再適用。
- full dedup は将来の opt-in option または別項目として扱う。

## #18 見開きは右から左がデフォルト、また、切り替えボタンも実装

推奨案を採用。

- default `spreadDirection = "rtl"`。
- `setSpreadDirection(dir)` で表示順だけ反転。
- route/hash の logical order `page-second` は変更しない。
- `#reader-controls` に direction toggle button を追加。
- `localStorage` に `hitomi_hub_spread_direction` を保存。
- keydown に方向切替 key、例 `r` を追加。
