# 単体 distribution startup script 追加

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: output/hitomi-hub に run.cmd を追加し、script folder を root として view を起動する

## 意図 / 背景

単体 distribution folder として `output\hitomi-hub` をほかの端末へコピーしたときに、cwd に依存せず、startup script のある distribution folder を runtime root として `hub.exe view` を起動できるようにする。

## 調査結果 / 変更内容

- `output\hitomi-hub\run.cmd` を追加。
- `run.cmd` の動作:
  - `pushd "%~dp0"` で script folder を cwd にする。
  - `hub.exe view` を実行する。
  - 起動後に `pause` でエラー終了時にも console を残す。
- Decision `docs/ai/decisions/2026-11-01-output-release-build-and-root-policy.md` に `run.cmd` を release build の必要ファイルとして追記。

## Verification

- Scratch script for path logic:
  - `docs\ai\tmp\verify_run_cmd_path.cmd`
  - Run from `C:\Users\fujim\AppData\Local\Temp\kilo`.
  - Result showed script folder and cwd:
    - `SCRIPT_DIR=S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\tmp\`
    - `CWD=S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\tmp`
- Production `run.cmd` content is `pushd "%~dp0"`, `hub.exe view`, `pause`.
- Canonical `hub.exe` latest-update correction is not executed in this task because the canonical server process is still running.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
