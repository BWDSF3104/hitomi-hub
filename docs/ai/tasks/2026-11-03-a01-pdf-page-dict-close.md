# A-01/B-11: PDF page 辞書の閉じ >> 不足

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

実測で有効。`app/pdf.go:142-143` の page 辞書は `<<` が 3、`>>` が 2 で outer が閉じていない。scratch gallery 106（有効 PNG）で `hub.exe export --id 106 --format pdf` を実行し、object 3 が `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 1 1] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R`（末尾 `>>` なし）と確認。`/Type /Pages`（object 2）は `>>` で閉じており、page 辞書だけ壊れている。再現スクリプト: `docs/ai/tmp/assess-20261103-1220/show_pdf_page.py`。対応: page 辞書を `/Contents N 0 R >>` で閉じる。合格条件: export PDF の page object が閉じ、object 3 が PDF パーサで読める。推定規模: 小。フル画像ガード: 該当なし。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
