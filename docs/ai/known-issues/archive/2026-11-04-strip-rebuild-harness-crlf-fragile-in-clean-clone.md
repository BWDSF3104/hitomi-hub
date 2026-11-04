# C-06 harness は clean clone（CRLF checkout）で失敗し得る

**Status**: 解決（2026-11-04・本実施）
**Date**: 2026-11-04

## 内容

D-07 の clean clone 実測（2026-11-04・`git clone` → `python -m pytest tests/ -v`）で、freshness test の skip は正しく発火した（36 passed・3 skipped）が、`tests/test_strip_rebuild.py`（C-06・close 済み）が 1 件 failed。

- 原因（実測）: harness `tests/strip_rebuild_dom_harness.mjs:161` は `app/ui.html` に exact 文字列 `"function stripThumbSource(...) {\n  return displayViewerSource(...);\n}"` を期待する。git の `core.autocrlf=true` で clean clone の checkout は LF→CRLF になるため、`\n` を含む照合が失敗（`FAIL TN display source rule unchanged`）。`\n` を含まない check（line 162）は成功し、差分は line ending のみ。
- 現working tree（LF）では 16 checks 全通過（C-06 合格時の実測は無損）。

## 次の対応（実施時）

- harness の照合を line ending 正規化（`\r\n`→`\n`）後に执行、または check を `\n` 非依存の expression に変更。実装・修正は実施段階（C-06 は close 済み・スコープ外）。

## 解決（2026-11-04・本実施）

- `tests/strip_rebuild_dom_harness.mjs` の read 直後に line ending 正規化（`\r\n`→`\n`）を追加。exact 照合（line 161）は正規化後の内容に対して実行され、TN display source rule（`stripThumbSource` → `displayViewerSource` 委派）・TN display fallback 禁止の实质検証は維持（文字列マッチの除去ではない）。
- 回帰テスト追加: `tests/test_strip_rebuild.py::test_strip_rebuild_harness_passes_after_crlf_checkout`（CRLF 化された ui.html コピーに対して harness を実行し、全 check 通過と TN rule 2 件の PASS を要求）。

## Verification

- 実測: clean clone（`C:\Users\fujim\AppData\Local\Temp\kilo\d07-clean-clone`）で pytest 1 failed / 36 passed / 3 skipped。同一 harness は正典 working tree で全通過。

- 実測（本実施）: 正典 working tree で harness 単独 18 checks 全通過（修正前後同一 check 数）、正典 pytest 41 passed（回帰テスト 1 件追加）。
- clean clone 実測: 単独 commit 後の HEAD に対して `C:\Users\fujim\AppData\Local\Temp\kilo\c06-crlf-clone` で `python -m pytest tests/ -v` を実施（push 前）。結果（当該 check 通過・数値）は実行報告参照。
