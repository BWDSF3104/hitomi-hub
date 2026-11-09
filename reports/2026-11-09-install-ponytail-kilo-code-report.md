# 報告：Kilo Code に Ponytail プラグインを導入

- タスク: 2026-11-09-install-ponytail-kilo-code（リモートタスク、task_manager receive 済み）
- 報告日: 2026-11-09
- 報告ファイル: reports/2026-11-09-install-ponytail-kilo-code-report.md

## 進捗

5 / 5 件 すべて完了

1. 保全：完了（kilo.jsonc のバックアップ作成。hitomi-hub 側の作業ツリーには他セッション由来の変更（current-task.tsv 等）があるが、それらはそのまま残し、保全コミットは行わない）
2. 現物と公式仕様の確認：完了（既存 `plugin` キーなし、キー名 `plugin`（単数形）、パッケージ名 `@dietrichgebert/ponytail`、npm 上実在・最新バージョン、Kilo Code バージョン）
3. 編集：完了（`plugin` キーの追加）
4. 検証：完了（読み直し・JSONC パース・バックアップとの差分）
5. プロジェクト側の差分確認：完了（hitomi-hub 側の変更対象は報告ファイル追加のみ）

## 公式ドキュメントの出典

- Ponytail 公式 INSTALL.md（Kilo Code の項）：グローバル導入は `~/.config/kilo/kilo.jsonc` のトップレベルに `{ "plugin": ["@dietrichgebert/ponytail"] }` を追加。キー名は `plugin`（単数形）、パッケージ名は `@dietrichgebert/ponytail`。
- Ponytail 公式 docs/agent-portability.md（Kilo Code の行）：同上。

## kilo.jsonc の編集内容

- パス: `C:\Users\fujim\.config\kilo\kilo.jsonc`
- 編集前: トップレベルに `plugin` キーなし
- 編集後: トップレベルに `"plugin": ["@dietrichgebert/ponytail"]` を追加
- その他: 他キー（provider, model, agent, skills 等）はすべて保持。ファイル全体の再整形・コメント削除は行わず、既存の書式を維持。

## 確認値

- npm `@dietrichgebert/ponytail` 最新バージョン: **5.3.3**（`npm view @dietrichgebert/ponytail version` で実在確認）
- Kilo Code バージョン: **7.8.15**（`kilo --version`）
- `$schema`: `https://app.kilo.ai/config.json`

## バックアップと保全

- バックアップ: `C:\Users\fujim\.config\kilo\kilo.jsonc.bak-20261109-ponytail`
- 保全コミット: なし（kilo.jsonc は hitomi-hub  git 管理外；hitomi-hub 側に変更対象ファイルなし。他セッション由来の変更はそのまま残し）

## 差分確認

- kilo.jsonc の変更: `"plugin": ["@dietrichgebert/ponytail"]` の追加および直前行へのカンマ追加のみ（`fc` により他変更なしを確認）
- JSONC パース: OK（コメント除去後も有効な JSON）

## 設定が効き始めるタイミング

次回 Kilo Code の新規セッション開始時から効く（プラグイン本体 `@dietrichgebert/ponytail` の取得もその時にプラグイン機構が行う）。目印は新規セッションで `/ponytail` を送り、現在のレベル（デフォルト full）が返ること。

※本報告に秘密情報（APIキー、トークン類）の値は含みません。
