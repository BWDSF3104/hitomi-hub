# D-07: canonical build test の前提

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（検証済み: 40 passed・skip 分岐 scratch 実測）。本項目分のみ単独 commit/push 実施（commit ID は commit 参照）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（限定）。`tests/test_canonical_build_freshness.py`（43-44 行）は `output/hitomi-hub/hub.exe` の存在と embedded ui.html bytes を要求する。`output/` は `.gitignore`（9 行）で追跡外 → clean clone/CI では test が fail。ここでは exe が存在するため `python -m pytest tests/ -v` = 25 passed（2026-11-03）。対応: exe 不在時は skip（または test 内で build）。合格条件: clean clone で test が skip/pass、exe がある環境では現状維持。推定規模: 小（テスト変更は実装段階）。

実施（承認済み方針 `docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md` D-07: exe 不在時は skip、test 内 build 不採用、exe がある環境は現状維持）。変更は `tests/test_canonical_build_freshness.py` のみ:
- `test_canonical_executable_embeds_current_ui` の冒頭（go toolchain チェックより前）に `CANONICAL_EXE.exists()` チェックを追加し、不在時 `pytest.skip("canonical executable is missing (output/ is not tracked in git)")`。clean clone/CI では go toolchain チェック・build より前に skip。
- 旧 43-44 行の `pytest.fail("canonical executable is missing")` を削除（冒頭チェックにより到達不能）。
- exe 存在環境の挙動は変化なし。実測: 変更後 `python -m pytest tests/ -v` = 40 passed。正典 exe の SHA256 は検証前後同一（90B06EA10DE1C0EA77576A2D576EDF02B3B2D02BDA646396E9B816F16674371D）、削除・変更なし。

## Verification

- 基準（変更前・exe 存在環境）: ルートで `python -m pytest tests/ -v` = 40 passed（8.14 秒、2026-11-03）。
- 変更後: ルートで `python -m pytest tests/ -v` = 40 passed（7.20 秒、2026-11-03）。`test_canonical_executable_embeds_current_ui` PASSED（exe 存在環境の現状維持を確認）。
- skip 分岐の実測（scratch `docs/ai/tmp/assess-20261103-1220/`、実ファイル削除なし・存在しない path への模拟）:
  - `python docs\ai\tmp\assess-20261103-1220\simulate_skip.py` → 実 exe 存在 True・模拟 path 存在 False、「skip branch fired -> canonical executable is missing (output/ is not tracked in git)」。go build 未実行。
  - `python -m pytest docs\ai\tmp\assess-20261103-1220\test_skip_sim.py -v` = 1 skipped（0.09 秒）。
- 正典 exe 無損確認: 検証前 SHA256 90B06EA10DE1C0EA77576A2D576EDF02B3B2D02BDA646396E9B816F16674371D = 検証後同一。Go 変更なし、canonical build 現状維持（freshness test 通過で確認）。
- 未確認: 実際の isolated clean clone 上での skip 発火は未実施（scratch 模拟での実測で代替）。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end 済み）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（該当なし）
