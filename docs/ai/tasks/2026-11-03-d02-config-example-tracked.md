# D-02: 既定の設定ファイルが追跡対象

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（方針）。既定 config path は `<data-dir>/config.example.json`（`app/main.go:662,669-672`）で repo 追跡下。`config.example.json` 自体は `api_key: ""`（秘密なし）だが、運用で同一 path に key を書くと追跡ファイルへ秘密が入る。`--config` フラグは存在（prior review の「missing --config」は解消済み）。対応: 既定 config を非追跡（`config.json` + `.gitignore`）へ移すか、example を追跡し実 config path を分離。合格条件: 実 key を置く path が追跡下でないこと。推定規模: trivial。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
