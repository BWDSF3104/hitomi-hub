# Fullscreen image display size fix and strict confirmation

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: CSS fullscreen size contract, actual UI f key confirmation, keyboard page movement, Drive output

## 意図 / 背景

Full screen image display size correction method and pass criteriaで設定した基準に基づき、fullscreen image display box が fullscreen content area を fill するように修正し、fresh tab・actual UI gesture・`window.resize` 前・keyboard page movement で画面確認を通した。

## 調査結果 / 変更内容

変更:
- `app/ui.html`: fullscreen CSS に fullscreen-specific size contract を追加。
  - `--fullscreen-page-padding: 12px`
  - `--fullscreen-page-height: calc(100vh - var(--fullscreen-page-padding) * 2)`
  - `--fullscreen-page-width: calc(100vw - var(--fullscreen-page-padding) * 2)`
  - fullscreen `#reader` content area uses border-box full viewport minus padding.
  - double fullscreen grid columns split available fullscreen width in two.
  - single fullscreen grid column uses full fullscreen width.
  - fullscreen image `img` display box uses `width: 100%`, `height: var(--fullscreen-page-height)`, `object-fit: contain`.
- `app/viewer_test.go`: fullscreen image display size, grid split, keyboard page movement を検査。

画面確認結果:
- canonical exe: `output\hitomi-hub\hub.exe`
- process: `bgp_199b7a1fb001uApMsaNdBv36kz`
- URL: `http://127.0.0.1:8731`
- fresh tab initial viewport: `1375x938`
- fullscreen target: `#reader`
- fullscreen content area: `1351x914`
- double grid columns: `675.5px 675.5px`
- image display box: `675.5x914` each
- actual displayed image inside `object-fit: contain`: `657.245x914` each
- keyboard:
  - `ArrowRight` moved page to `3`
  - `ArrowLeft` moved back to `1`

## Verification

- `go vet .`: pass.
- `go test .`: pass (`hitomi-hub/app` `23.790s`).
- canonical build: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ..\output\hitomi-hub\hub.exe .` pass.
- Google Drive output folder: `I:\マイドライブ\kilo\2026-11-02`
- Google Drive screenshot: `I:\マイドライブ\kilo\2026-11-02\fullscreen-image-size-20261102.png`
- Google Drive operation text: `I:\マイドライブ\kilo\2026-11-02\fullscreen-image-size-confirmation-20261102.txt`

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
  - `docs/ai/known-issues/2026-11-02-fullscreen-reader-image-not-full-viewport.md` was moved to `docs/ai/known-issues/archive/`.
- [ ] `backlog.md` の完了項目を削除（該当する場合）
