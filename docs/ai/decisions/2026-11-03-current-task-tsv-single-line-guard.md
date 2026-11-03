# current-task.tsv 不正 fragment 削除と task_manager 単一行 guard

**Status**: 完了
**Date**: 2026-11-03

## 背景
- `docs/ai/current-task.tsv` に `task_file<TAB>status<TAB>next_action` の正典行ではない Irregular fragment が残っていた。
- 直接原因は A-10 の `**Next Action**` が複数物理行で書かれ、TSV 行ルールへ反映されたこと。
- 既存 fragment はユーザー指示により `edit` ツールで削除した。

## 決定
- 今後の TSV 更新では `--status` / `--next-action` を単一行のみとする。
- `--status` / `--next-action` に `\n`, `\r`, `\t` を含めない。
- `current-task.tsv` の data row は 3 field (`task_file`, `status`, `next_action`) でないと更新を拒否する。空 data row も不正行として拒否する。
- 実測、窓依存、長文、調査結果は `Next Action` へ置かず、タスクファイル本文、`decisions/`, `known-issues/`, `Verification` へ書く。

## 実装
- 正典 `S:\Documents\Kilo_Commons\Common_Operations\bin\task_manager.py`:
  - `_validate_single_line` を追加し、`add` / `update` の `--status` / `--next-action` を単一行で校验。
  - `_validate_tsv_row` / `_validate_tsv_rows` を追加し、既存 data row が 3 field でない場合、および空 data row を更新前に拒否。
  - `_write_tsv` で malformed row を `ValueError` として拒否し、command 側で `ERROR` へ変換。
- `S:\Documents\Kilo_Commons\Common_Operations\tests\test_task_manager.py`:
  - multiline next-action、tab status、existing malformed TSV row、`_write_tsv` malformed/blank row の拒否テストを追加。

## Verification
- 正典実装 commit: `645b89b`（push済み）。
- `python -m pytest tests/ -v`: 120 passed, 6.19s。
- `pwsh -NoProfile -File tests\smoke_safe_rmdir.ps1`: 0 failure(s)。

## Note
- `docs/ai/current-task.tsv` の直接編集は Irregular fragment の削除に限ってユーザー指示。今後の status / next_action 更新は正典 `task_manager.py update` を使う。
