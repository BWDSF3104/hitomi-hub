# hitomi-hub

hitomi.la 用クライアントのハブ。検索・閲覧・ダウンロードに加え、サイト側にない機能（既読管理・お気に入り・構造検査）をローカル DB で持つ。

## 共通規約

言語・実装開始ゲート・ターミナル安全・コマンド実行ルール・正典ツール（`bin/`）・コード検証の共通条・Memory Bank 運用・commit message 規約は global 正典 `AGENTS.md`（`C:\Users\fujim\.config\kilo\AGENTS.md`）を参照。このファイルにはプロジェクト固有項目のみ記載する。

正典ツールはプロジェクトごとにコピーせず `S:\Documents\Kilo_Commons\Common_Operations\bin\`（PATH 登録）の単一正典を使用。本プロジェクトでも `task_manager.py`・`archive_memory_bank.py`・`kilo-safe-*` は同じ正典を参照する（ルート判定は `hitomi-hub/.git` に解決される）。

## 絶対に壊してはいけない条件（フル画像）

詳細な経緯よりも、この節が優先する。変更が必要に見えても、ユーザーの明示承認なしに
この節に反する実装・合格条件の変更をしないこと。反する案を出す場合は、実装案ではなく
「方針変更の提案」として分けて出し、承認を得てからにすること。

- フル画像の取得経路は `app/viewer.go imageProxy` の順序を維持する:
  local full → 製品 root 配下の persistent full cache（既定 `cache/images`）→ remote `StorageURL` → 404 時のみ `EnsureGG(force=true)` で gg が変わった場合に1回再試行 → `no_full`
- 永続先を `scratch/` や `output/` へ移さない。それらは確認用／成果物用で永続先ではない
- shard URL は format（webp/avif）で組む。galleryinfo の `name` の拡張子（jpg/png 等）で組まない（`StorageURL`）
- gg は live 取得が既定で、`app/ggset.go` は snapshot = fallback。bucket/switch は約1時間で回転する
- TN は search-card の cover 専用。reader（`#reader` / `#thumbs`）の表示源にしない。TN display fallback は禁止
  - 根拠: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、
    `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- TN バイトを full cache に保存しない。`tn` / `tn-cache` / `no_full` / `error` を full actual pixels として数えない
- UI の full 判定（`x-cache`、no-store fetch、objectURL 表示、generation guard）を弱めない

### 作業前チェック（フル画像・viewer・download に触るタスクでは必須）

1. 上記2つの decision と `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を読んだことを、タスク記録にファイル名で明記する
2. 変更後に `go vet . && go test -count=1 -timeout 300s .`（app）と canonical build での実機確認を行い、次を報告する
   - CLI download: 実在 gallery で attempted と written が一致すること
   - viewer: `/api/image/<hash>?format=webp` が 200 / `x-cache: miss|cache|local` で、`no_full` でないこと（バイト数も記録）
3. 上記が確認できない場合は完了と書かず、未確認として報告する

## コード検証（本プロジェクト）

- **Python 変更後**: `python -m pytest tests/ -v`
- **Go app 変更後**: `app` で `go vet . && go test -count=1 -timeout 300s .`
- テストの外部 API 呼出し方針: `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`

## Canonical Latest Build / UI Confirmation Test Rule

- 正典（直接記載は廃止・ポインタ化 2026-11-04）: `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`
  - ポインタ化の記録: `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`

## Development

- dev server・build コマンドは適用なし。スクリプトは `python src/scan.py` で実行
- 単体静的バイナリ化（Go + pure-Go SQLite）が完成形の目標。Go toolchain は dev 依存であり実行依存ではない

## Code Standards

- Python: UTF-8、stdlib のみ（移植性のため外部依存を増やさない）
- 設定は JSON（Python 3.10 に `tomllib` がないため TOML は使用しない）
- **移植性の必須条件**: アプリ自身の書き込みを完成フォルダ内に閉じる。絶対パス・`%APPDATA%`・temp 展開を書かない。native DLL・runnable runtime を増やさない

## Code Scope

- 実装済み: search（nozomi・searchhtml・searchindex）・download・viewer/UI・既読/お気に入り書き込み口・scan（Go・Python 参照実装）・YARA・VT lookup。未実装は `docs/ai/backlog.md` の P2 項目のみ（#8 は対象外と決定 2026-11-04、#9 は用途定義 2026-11-04）
- ウイルス判定引擎をアプリ側で所有しない。構造検査とハッシュ評判のみ

## Rules

- Project rules folder: `docs/ai/rules/`
- Normal screenshot rule: `docs/ai/rules/normal-screenshot-rule.md`

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
