# B-06: VT の空 last_analysis_results が pass

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（静的）。`src/scan.py` の `vt_lookup`（`fetch=None` path, 約 203 行）は `results.get("last_analysis_results") or {}` を iterate するだけで、`source_stats`/`stats` を参照しない。空 results の file は `pass` として return され、VT cache（60 日）に `pass` が保存される → verdict 更新後に `pass` へ巻き戻る。live 確認は API key がないため不可（config は `enabled: false`）。テスト変更は現段階不可。対応方針: `stats.total > 0` の場合、results が空なら `not_evaluated`（または `unknown_to_vt`）扱い。合格条件: mock fetch で空 results + stats → status `not_evaluated`、cache に `pass` を書かない。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
