# 確認スクリプト正典化

**Status**: 完了
**Date**: 2026-11-04
**Next Action**: 正典 scripts と tests/harness_lib.mjs を実装し、Common_Operations/smoke と hitomi-hub pytest で検証

## 意図 / 背景

- `docs/ai/tmp` の7日保持により、確認用のスクリプトが毎回再作成されている。
- 通常 OS screen capture、fresh browser、CDP measurement、Drive confirmation record、Node harness helper を正典として残したい。
- Playwright page screenshot や `outerWidth/innerHeight` proxy は normal screen capture 基準では不合格。正典化では actual OS capture を維持する。

## 調査結果 / 変更内容

- `docs/ai/tmp` の7日保持で確認スクリプトが繰り返されていた。正典として以下を `S:\Documents\Kilo_Commons\Common_Operations\bin\` に追加:
  - `bin/kilo-screen-capture.ps1`
  - `bin/kilo-fresh-browser.ps1`
  - `bin/kilo-cdp-measure.ps1`
  - `bin/kilo-drive-confirm.ps1`
- `kilo-screen-capture.ps1` は actual OS screen capture、DPI-aware、physical/virtual screen 最大寸法、actual saved PNG dimension check、Drive task-specific copy、foreground window activation を含む。Playwright page screenshot や `outerWidth/innerHeight` proxy は capture proxy として扱わない。
- `kilo-cdp-measure.ps1` は DOM/network 数値 read 専用で、visual visual pass の capture proxy にしない。
- `kilo-drive-confirm.ps1` は Google Drive task-specific folder へ confirmation text を UTF-8 no BOM で書き込む。
- hitomi-hub 側で `tests/harness_lib.mjs` を共有 helper 正典化し、`strip_rebuild_dom_harness.mjs` と `read_lists_debounce_harness.mjs` の `bodyIn` / project file read を共有した。
- `tests/test_strip_rebuild.py` の CRLF regression で helper import が tmp_path で解析できるよう、`harness_lib.mjs` のコピーを追加した。
- Common_Operations に `tests/smoke_confirmation_canonical_scripts.ps1` を追加。dry-run だけ確認し、実際の browser launch、OS capture、CDP connect、Drive write は行わない。
- 正典 decision: `docs/ai/decisions/2026-11-04-confirmation-script-canonicalization-approved.md`。

## Verification

- `pwsh -NoProfile -File tests\smoke_confirmation_canonical_scripts.ps1`
  - 結果: 0 failure(s)
  - 確認内容: screen capture, drive confirm, fresh browser, CDP measure の dry-run plan。実測 capture / browser launch / CDP connect / Drive write は行っていない。
- `python -m pytest tests/ -v` in `S:\Documents\Kilo_Commons\Common_Operations`
  - 結果: 120 passed in 6.68s
- `python -m pytest tests/ -v` in `S:\Documents\Kilo_Commons\hitomi-hub`
  - 結果: 51 passed in 11.22s
  - `tests/strip_rebuild.py::test_strip_rebuild_harness_passes_after_crlf_checkout` が helper import 後も通過。
- `python docs/ai/tmp/check_bom.py` で新規 PowerShell script、`tests/harness_lib.mjs`、decision の BOM なしを確認。
- 新規ファイルは UTF-8 no BOM。PowerShell comments は ASCII-only。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
