# A-01/B-11: PDF page 辞書の閉じ >> 不足

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（実装・テスト・実機確認済み）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

実測で有効。`app/pdf.go:142-143` の page 辞書は `<<` が 3、`>>` が 2 で outer が閉じていない。scratch gallery 106（有効 PNG）で `hub.exe export --id 106 --format pdf` を実行し、object 3 が `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 1 1] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R`（末尾 `>>` なし）と確認。`/Type /Pages`（object 2）は `>>` で閉じており、page 辞書だけ壊れている。再現スクリプト: `docs/ai/tmp/assess-20261103-1220/show_pdf_page.py`。対応: page 辞書を `/Contents N 0 R >>` で閉じる。合格条件: export PDF の page object が閉じ、object 3 が PDF パーサで読める。推定規模: 小。フル画像ガード: 該当なし。

実装（A-01/B-11）: `app/pdf.go` の exportPDF page object を `fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>", ...)` に修正し、page 辞書の outer `>>` を追加した。テスト `app/export_test.go` の `TestExportPDFPageObjectDictCountsAreBalanced` で object 3 内の `<<` と `>>` 数を比較し、page object が閉じていることを固定した。

## Verification

- `go vet .`（`app`）: clean.
- `go test -count=1 -timeout 300s .`（`app`）: `ok hitomi-hub/app 37.193s`. New test `TestExportPDFPageObjectDictCountsAreBalanced` compares `<<` count and `>>` count in object 3.
- Canonical build from `app`: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded.
- `python -m pytest tests/ -v`: 25 passed, including `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui`.
- CLI download from canonical exe: `.\hub.exe download --id 4300588 --limit 1` returned `attempted=1`, `written=1`.
- Canonical server for non-UI network confirmation: started `bgp_1a0477d0d001xbu5abS11fmc8X` PID `15056`, `http://127.0.0.1:8731/`. Served API image response for hash `09e35f8e1696fd8a9f9967a4492d79613722c2ce520cb3d60f26bc8bc674df08?format=webp` was `200`, `x-cache=miss`, bytes `171738`, not `no_full`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
