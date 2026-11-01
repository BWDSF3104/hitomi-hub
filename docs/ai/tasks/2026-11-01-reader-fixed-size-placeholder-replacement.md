# 見開き固定サイズ・隙間なし・プレースホルダー置換修正

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: 完了（検証済み）

## 意図 / 背景
- 見開き表示のページ間に隙間をなくす。
- サムネイルプレースホルダーの表示サイズを、フル画像表示時と同等の固定サイズにする。
- cached full image では `onload` が発火せず placeholder がそのまま残る事象を直す。

## 調査結果 / 変更内容
- `app/ui.html` の reader grid を `gap: 0` に変更。
- reader item display size を `--reader-page-width: min(45vw, 500px)`、`--reader-page-height: min(78vh, 700px)` で固定化。placeholder/full 同じ `object-fit: contain` box を使う。
- fullscreen height は `min(90vh, 700px)`。
- `loadFullImage()` で full image load event だけでなく、`full.src` 設定後に `full.complete` を見て immediate replace する。これにより browser/memory cache hit で `onload` が発火しないケースでも placeholder が full に置き換わる。

## Verification
- `go vet ./...` passed.
- `go test -count=1 ./...` in `app`: `30.372s` first, `29.963s` fixed size variant, `29.953s` final complete replacement.
- Playwright screen verify at `http://127.0.0.1:8731/?x=final1920#/g/2648601/page/2-3`:
  - `readerGap: 0px`
  - `gridColumns: 500px 500px`
  - displayed full images: `displayWidth: 500`, `displayHeight: 700`
  - placeholder immediate replacement test: `replacedNow: true` for cached full.
- Placeholder display size check: TN natural `107x150`, fixed display box `500x700` with `object-fit: contain`.
- Screenshot: `scratch\hitomi-usage\reader_fixed_size_no_gap.png`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
