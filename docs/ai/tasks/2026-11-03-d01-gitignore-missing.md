# D-01: .gitignore の不足

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

C-07実操作後のDB journal（`hitomi.db-shm`/`hitomi.db-wal`）が `git status` に残存するため、単独先行で `.gitignore` 不足を解消する。目的は記録汚染の防止で、journalファイル自体はコミットしない。

## 調査結果 / 変更内容

有効（実測）。`git status` で `hitomi.db-shm`, `hitomi.db-wal`, `Galleries/`, `app/docs/ai/tmp/hang.txt`（前タスク残置）が untracked。`.gitignore`（9 行）は `*.db-wal`/`*.db-shm` を含まない。対応: `.gitignore` に `*.db-wal`, `*.db-shm`, `Galleries/`（または完成フォルダの既定名）を追加。合格条件: `git status --short` に DB journal と完成フォルダが出ない。推定規模: trivial。

実施（2026-11-03）: `.gitignore` 末尾に正典 entry を変更せず追加: `*.db-wal`, `*.db-shm`, `Galleries/`, `app/app.exe`, `app/docs/`。
`app/docs/` 内容確認: `app/docs/ai/tmp/hang.txt` のみ（43375 B、Go test タイムアウトの goroutine ダンプ＝前タスクの runtime scratch 残置、API key なし）。正典 `docs/ai` 記録ではなく混淆しないため、ignore 対象として妥当と判断（commit 対象ではない）。ファイル自体は削除もコミットもせず。
`Galleries/` は完成フォルダ既定名の予防 entry。journal/exe/生成物自体はコミットしない。

## Verification

- docs-only変更のため project tests（pytest/go）は適用なし。正典 tool 変更なし。
- `git status --short`（実施後）: `M .gitignore` のみ。`hitomi.db-shm`/`hitomi.db-wal`/`Galleries/`/`app/app.exe`/`app/docs/` 不出（合格）。
- `git diff --stat`: `.gitignore` 6 行追加のみ。正典 entry の変更なし。
- 正典 Common_Operations tests（task_manager validation）: `python -m pytest tests/ -v -k task_manager` → 35 passed（0.96s）。正典 tool 変更なしのため smoke test（smoke_safe_rmdir.ps1）は不要と判断。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
