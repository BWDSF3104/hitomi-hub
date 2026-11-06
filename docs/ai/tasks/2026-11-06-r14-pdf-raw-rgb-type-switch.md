# R-14: rawRGB が pixel ごと img.At で遅く、alpha を捨てて透過が黒になる

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。案: *image.NRGBA / *image.YCbCr 型 switch か draw.Draw で RGBA 化。Flate raw RGB の肥大化は format 分岐（DCTDecode）判断を要る。記録: R-14（A-11 の memory 解消後の速度・alpha 点）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
