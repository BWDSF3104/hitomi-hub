# A-11: rawRGB が全ページ分保持される

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/pdf.go:47-58`（`rawRGB`）の結果を `pdfPage` に保存し、`app/pdf.go:112` で全 page を slice に溜める。1740x2400 の image 1 枚 = 約 12.5MB。gallery 1300 枚なら約 16GB。対応: page ごとに encode して sequential に書く（zlib 生成後に解放）。合格条件: 大きな gallery（mocked fixture）で peak memory が page 1 分＋header に収まる。推定規模: 中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
