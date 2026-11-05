# 保存済みステータスの定義（後で定義が必要）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 完成後ファイル削除の扱い確認（時点記録 vs state 再計算）。実装開始指示未受領

## 意図 / 背景

保存済み（saved）ステータスの定義を、後で確定する。ユーザー指示 2026-11-05 08:02 により本件（`docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`）から分離。

現状の正典定義は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項（指定フォルダ内に全 entry の local ファイルがあること・部分保存は badge に出さない）。本タスクではこの定義が将来何を基準とするかを確定する（正典の変更の有無・部分保存の扱い・算出源の恒久的な決め方を含む）。

本件 jump 機能の側は、本タスクの確定を待たず現行正典定義をそのまま使用することを継続判断として維持（算出は `v.info` の既存源・近似は表示不一致のため不採用）。

## 調査結果 / 変更内容

### 定義確定（ユーザー指示 2026-11-05 13:52）

- **saved = 当該 gallery のすべての page（全 entry）を指定フォルダ（config `gallery_dir`、既定 `Galleries`）へコピーし終わった時点**
- 既存正典 `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項との関係:
  - 正典の「指定フォルダ内に全 entry の local ファイルがあること」は、通常運用（削除なし）でコピー完了時点と等価 → **正典は無変更**
  - 部分保存の扱い: 無変更。部分保存は badge に出さず meta 側 `local N/M`（この定義と一致 — badge は完成時点のみ）
  - この定義は**時点（event）基準**。現行実装（card API の `entryLocalFormats` ループによる state 再計算）との唯一の分岐は**完成後のファイル削除**: state 再計算では saved が false に戻る、時点基準では true を保持
- 算出源の恒久的な決め方（保留・確認必要）:
  - 逐再計算（現行どおり・追加 I/O なし）→ 完成後削除で badge が戻る
  - 完成時点の記録（sticky）→ `read_state` 等に event 列が必要（schema 変更・迁移は非破壊規約準守）

### 残る確認点

1. 完成後ファイル削除時に saved を保持するか（時点記録）それとも state 再計算で false へ戻すか。これにより算出源（event 列 vs 既存 `entryLocalFormats`）が確定する。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
