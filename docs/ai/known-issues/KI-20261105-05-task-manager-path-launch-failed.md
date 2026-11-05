# 正典 `task_manager.py` の PATH 起動失敗（pwsh）

**Status**: 未解決（記録のみ・本項目での修復は行わない）

## 症状

- 2026-11-05、hitomi-hub の cwd で `task_manager.py add ...` を pwsh で実行すると失敗:
  `プログラム 'task_manager.py' を実行できませんでした: An error occurred trying to start process 'S:\Documents\Kilo_Commons\Common_Operations\bin\task_manager.py' ... アプリケーションが見つかりません`
- 正典ファイル自体は存在し、`python <正典パス>` での実行は成功した（ルート判定は cwd の `.git` から `hitomi-hub` に解決され、正常動作）。

## 想定原因（未確認）

- PATH 登録は `bin\` ディレクトリのみで、`.py` を直接起動する仕組み（実行関連付け・shebang・`.cmd` shim）が pwsh 側で機能していない。
- PowerShell は PATH 上の非実行ファイル（`.py`）をそのまま起動できない可能性が高い。Python ファイル関連付けが無効・または shebang が無視されている。

## 回避策（使用済み）

- `python "S:\Documents\Kilo_Commons\Common_Operations\bin\task_manager.py" ...` で正典単一コピーを直接参照して実行（正典一元化の規約は維持）。

## 次の対応

- 正典 `bin\` に `task_manager.cmd`（`python "%~dp0task_manager.py" %*`）等の shim を追加するか、ファイル関連付けを確認する。修复には別の実装承認が必要（正典ツール変更後は Kilo_Commons で `python -m pytest tests/ -v`）。
