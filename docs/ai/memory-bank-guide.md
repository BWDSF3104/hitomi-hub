# AI Memory Bank ガイド

docs/ai/ は git 追跡対象。1エントリ1ファイル方式。共通条（言語・実装開始ゲート・正典ツール・commit 前更新）は global 正典 `AGENTS.md` を参照。

## ファイル一覧

| ファイル・ディレクトリ | 役割 | 更新タイミング |
|----------|------|---------------|
| `current-task.tsv` | アクティブタスク参照（TSV 3 列: task_file / status / next_action） | `task_manager.py add` / `end`（正典 bin 参照） |
| `tasks/*.md` | 1タスク1ファイル。意図・背景・調査結果/変更内容・Verification・完了時チェックリスト | 開始時に add、設計確定時に即時更新、完了時に end |
| `decisions/` | 設計判断ログ（1エントリ1ファイル） | 構造・データフロー・新しい統合の設計判断時。作成後は不変、修正は新ファイル＋旧ファイル冒頭に Superseded by |
| `known-issues/`（+ `archive/`） | 既知問題。トップ=未解決 | 発見時に作成、解決時に archive へ mv |
| `backlog.md` | 未実装項目 | 追加・完了時削除 |
| `architecture.md` | アーキテクチャ | decisions の変更が構成に反映されたときのみ更新 |
| `api-rate-limits.md` | 外部 API のレート制限 | 制限が変わったとき |
| `tmp/` | スクラッチ（使い捨て） | `archive_memory_bank.py` で 7 日以上未更新を削除 |

アーカイブ操作は `kilo-safe-remove` / `kilo-safe-rmdir` 経由（ゴミ箱送り）。ファイル内容の書き換えはしない（`current-task.tsv` の行削除のみ例外）。

命名規則: `decisions/YYYY-MM-DD-short-name.md`、`known-issues/KI-YYYYMMDD-NN-short-name.md`。

## 中断からの復帰

1. `current-task.tsv` を読む
2. `git status` / `git diff` を確認する
3. 最後に記録したテストの状態を検証する
4. `next_action` から続ける。明示的な指示なしに既存の変更を破棄しない
