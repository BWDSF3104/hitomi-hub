# D-07: canonical build test の前提

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（限定）。`tests/test_canonical_build_freshness.py`（43-44 行）は `output/hitomi-hub/hub.exe` の存在と embedded ui.html bytes を要求する。`output/` は `.gitignore`（9 行）で追跡外 → clean clone/CI では test が fail。ここでは exe が存在するため `python -m pytest tests/ -v` = 25 passed（2026-11-03）。対応: exe 不在時は skip（または test 内で build）。合格条件: clean clone で test が skip/pass、exe がある環境では現状維持。推定規模: 小（テスト変更は実装段階）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
