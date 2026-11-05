# 保存済みステータスの定義（後で定義が必要）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 定義の確定（既存正典 2026-11-05 badges 3項との関係を確定）

## 意図 / 背景

保存済み（saved）ステータスの定義を、後で確定する。ユーザー指示 2026-11-05 08:02 により本件（`docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`）から分離。

現状の正典定義は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項（指定フォルダ内に全 entry の local ファイルがあること・部分保存は badge に出さない）。本タスクではこの定義が将来何を基準とするかを確定する（正典の変更の有無・部分保存の扱い・算出源の恒久的な決め方を含む）。

本件 jump 機能の側は、本タスクの確定を待たず現行正典定義をそのまま使用することを継続判断として維持（算出は `v.info` の既存源・近似は表示不一致のため不採用）。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
