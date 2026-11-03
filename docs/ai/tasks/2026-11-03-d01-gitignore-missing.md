# D-01: .gitignore の不足

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`git status` で `hitomi.db-shm`, `hitomi.db-wal`, `Galleries/`, `app/docs/ai/tmp/hang.txt`（前タスク残置）が untracked。`.gitignore`（9 行）は `*.db-wal`/`*.db-shm` を含まない。対応: `.gitignore` に `*.db-wal`, `*.db-shm`, `Galleries/`（または完成フォルダの既定名）を追加。合格条件: `git status --short` に DB journal と完成フォルダが出ない。推定規模: trivial。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
