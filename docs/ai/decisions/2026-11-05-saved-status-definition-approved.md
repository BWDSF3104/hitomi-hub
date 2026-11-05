# 保存済み（saved）ステータスの定義確定（2026-11-05）

**Status**: 承認済み（ユーザー 2026-11-05 13:52）
**Date**: 2026-11-05
**対象タスク**: `docs/ai/tasks/2026-11-05-saved-status-definition.md`
**実装開始指示**: 未受領（本記録は定義の確定のみ・ソースコード無変更）

## 確定した定義

- **saved = 当該 gallery のすべての page（全 entry）を指定フォルダ（config `gallery_dir`、既定 `Galleries`）へコピーし終わった時点**。

## 既存正典との関係

- `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項（「指定フォルダ内に全 entry の local ファイルがあること・部分保存は badge に出さない」）は通常運用でこの定義と等価 → **正典は無変更（supersede しない）**。
- 部分保存: badge に出さず meta 側 `local N/M`（無変更・定義と一致）。
- jump 機能（`docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`）側は現行正典定義と算出源 `v.info`（`entryLocalFormats` strict 判定）を継続。

## 保留点（実装前の確認必要）

- 完成後のファイル削除時の扱い: 定義は時点（event）基準。現行の state 再計算（`entryLocalFormats`）では完成後削除で saved が false へ戻る。時点記録（sticky）を選ぶ場合は event 列の追加（schema 変更・非破壊迁移）が必要。選択により算出源の恒久的な決め方が確定する。
