# 検索ボタン・初期表示サイズ fix and strict screen confirmation

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: UI/CSS修正、fresh tab・server baseline・pre-resize画面確認、Drive出力作成

## 意図 / 背景

Strict screen confirmation criteriaで設定した合格条件に基づき、検索ボタン stale response、duplicate `id="query"`, 初期表示サイズ異常を修正し、fresh tab・server direct baseline・`window.resize` 前のDOM/network一致・既存機能回帰で画面確認を通す。

## 調査結果 / 変更内容

変更:
- `app/ui.html`: `.side` に `min-width: 30rem` と `flex-shrink: 0` を追加。
- `app/ui.html`: `.card` を `height: var(--search-card-height); overflow: hidden;` にし、`.card .text` に `max-height` を追加。
- `app/ui.html`: `.tags` の `max-height` を `calc(var(--search-card-height) - 2rem)` に変更し、固定カード高さ内に収める。
- `app/ui.html`: `api()` fetch に `cache: "no-store"` を追加。
- `app/ui.html`: 保存検索 form を `id="saved-search"` に変更し、submit listener を同じ form に attach。
- `app/viewer.go`: `/api/search` response に `cache-control: no-store` を追加。
- `app/viewer_test.go`: `.side`, search card height, saved-search, UI api no-store, `/api/search` no-store を検査。

画面確認結果:
- canonical exe: `output\hitomi-hub\hub.exe`
- process: `bgp_1999b4fe2001cWLIk4zHWTvuJB`
- URL: `http://127.0.0.1:8731`
- fresh tab initial viewport: `958x938`
- server direct baseline: `total 32878`, `ids.length 25`, `cache-control: no-store`, `X-Search-Timing` present
- UI `#search-submit` click: request actual, `total 32878`, `ids.length 25`
- UI DOM before resize: `.side` `480px`, `#search` height `67.7734375px`, `#results` cards `25`, `pageStatus` `page 1 / 1316・total 32878`
- duplicate `id="query"` resolved: only `INPUT` remains, `form id="saved-search"` exists.

## Verification

- `go vet .`: pass.
- `go test .`: pass (`hitomi-hub/app` 24.920s).
- canonical build: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ..\output\hitomi-hub\hub.exe .` pass.
- Google Drive output folder: `I:\マイドライブ\kilo\2026-11-02`
- Google Drive screenshot: `I:\マイドライブ\kilo\2026-11-02\search-button-initial-display-screen-20261102.png`
- Google Drive operation text: `I:\マイドライブ\kilo\2026-11-02\search-button-initial-display-confirmation-20261102.txt`
- Google Drive operation text is rewritten in clear Japanese with the initial display size pass criteria included.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
