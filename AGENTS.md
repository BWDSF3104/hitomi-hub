# hitomi-hub

hitomi.la 用クライアントのハブ。検索・閲覧・ダウンロードに加え、サイト側にない機能（既読管理・お気に入り・構造検査）をローカル DB で持つ。

## 共通規約

言語・実装開始ゲート・ターミナル安全・コマンド実行ルール・正典ツール（`bin/`）・コード検証の共通条・Memory Bank 運用・commit message 規約は global 正典 `AGENTS.md`（`C:\Users\fujim\.config\kilo\AGENTS.md`）を参照。このファイルにはプロジェクト固有項目のみ記載する。

正典ツールはプロジェクトごとにコピーせず `S:\Documents\Kilo_Commons\Common_Operations\bin\`（PATH 登録）の単一正典を使用。本プロジェクトでも `task_manager.py`・`archive_memory_bank.py`・`kilo-safe-*` は同じ正典を参照する（ルート判定は `hitomi-hub/.git` に解決される）。

## コード検証（本プロジェクト）

- **Python 変更後**: `python -m pytest tests/ -v`
- テストは外部 API を呼ばない（VirusTotal は mock）。公開エンドポイントで認証不要な RSS 以外の外部呼出しは禁止

## Development

- dev server・build コマンドは適用なし。スクリプトは `python src/scan.py` で実行
- 単体静的バイナリ化（Go + pure-Go SQLite）が完成形の目標。Go toolchain は dev 依存であり実行依存ではない

## Code Standards

- Python: UTF-8、stdlib のみ（移植性のため外部依存を増やさない）
- 設定は JSON（Python 3.10 に `tomllib` がないため TOML は使用しない）
- **移植性の必須条件**: アプリ自身の書き込みを完成フォルダ内に閉じる。絶対パス・`%APPDATA%`・temp 展開を書かない。native DLL・runnable runtime を増やさない

## Code Scope

- 検索・ダウンロード・UI・既読/お気に入りの書き込み口は未実装。実装は `docs/ai/backlog.md` に記録
- ウイルス判定引擎をアプリ側で所有しない。構造検査とハッシュ評判のみ

## Documentation

- VirusTotal public API: `https://developers.virustotal.com/reference/overview`（4 req/min・500 req/day、hash-only 照会は内容を送信しない）
- YARA-X: `https://github.com/VirusTotal/yara-x`（classic YARA は maintenance mode）
- AI Memory Bank: `docs/ai/memory-bank-guide.md`

## Project Structure

```
hitomi-hub/
├── .git/
├── AGENTS.md
├── LICENSE                  # MIT
├── README.md
├── src/                     # scan.py・schema.sql・config.example.json・rules/structural.yar
├── tests/
├── tools/VERSIONS.md        # 単体 exe の版固定と digest（exe 自体は非追跡）
└── docs/ai/                 # AI Memory Bank（1エントリ1ファイル）
```

## Workflows

commit・push 手順は `docs/ai/workflows.md` を参照。GitHub: `BWDSF3104/hitomi-hub`（public・MIT）。
