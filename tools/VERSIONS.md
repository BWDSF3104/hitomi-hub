# 単体 EXE の版固定（digest 検証済みのもののみ）

フォルダコピーで動く形を維持するため、exe はコミットしない（`.gitignore` の `tools/*.exe`）。このファイルに版と digest を記録し、取得時に照合する。

## yr（YARA-X CLI）

- 版: **v1.22.1**（2026-10-26 release）
- asset: `yara-x-v1.22.1-x86_64-pc-windows-msvc.zip`
- 公開 SHA-256: `dbe8c1ec5d59d8a81c777d949c518fb5e75f1026d3419bca7817727c5d47b327`
  - 2026-11-01 取得・照合済み。実測 `dbe8c1ec5d59d8a81c777d949c518fb5e75f1026d3419bca7817727c5d47b327` = GitHub API の asset digest と一致
  - `gh attestation verify` は attestation 未登録（HTTP 404）のため判断根拠は GitHub API の digest 一致。出典は VirusTotal 公式リポジトリ（github-actions 発行）
- 導入先: `tools/yr.exe`（非追跡）。`yr --version` → `yara-x-cli 1.22.1`
- 用途: 構造規則 `rules/structural.yar`（リポジトリルート＝完成フォルダ直下）の実行引擎
- 留意点: このビルドは msvc target。クリーン環境では `vcruntime140.dll`（VC++ ランタイム）を要求する可能性がある。純粋なコピペ運用が優先なら `yr` なしで `scan.py` の構造検査のみで完結させられる
- 実行時の上限フラグ（`yr scan` に有効なフラグのみ）: `--timeout 10 --cpu-limit 50 --skip-larger 67108864 --fast-scan --output-format ndjson`。classic YARA の `--max-rules` は `yr` に存在しない
- 実測 exit code（2026-11-02）: `tools/yr.exe` の scan/match/no-match は exit `0`、invalid rule などの error は exit `1`。したがって `runYaraWithRunner` は非 0 exit を scan_error として扱い、rule output の有無だけで hit にしない

## Go toolchain（開発環境・winget でインストール済み）

- 版: **GoLang.Go 1.27.0**（winget package）
- 実測 toolchain version: `go1.27.3 windows/amd64`（`app/go.mod` が `go 1.27.3` を要求し、winget MSI 起動時に toolchain 1.27.3 が解決される）
- 導入コマンド: `winget install --id GoLang.Go -e --source winget --accept-package-agreements --accept-source-agreements`
- 実測インストール path: `C:\Program Files\Go\bin\go.exe`
- User PATH に追加済み: `C:\Users\fujim\go\bin` と `C:\Program Files\Go\bin`
- 留意点: 起動済み Kilo tool process には PATH refresh が反映されないことがある。この shell で `go` が失敗しても、正典 Go path `C:\Program Files\Go\bin\go.exe` を絶対パスで使う。Kilo プロセスを再起動すると `go` として使える想定。
- scratch Go path は今後の開発環境参照用として不適切。この project の dev command では `go` を使う。
- ビルド: `CGO_ENABLED=0`（pure-Go・VC++ ランタイム不要）。SQLite ドライバは `modernc.org/sqlite` v1.61.1（CGO 不要）
- 完成フォルダの形: `hub.exe`（単体静的バイナリ・非追跡）+ data ファイル（`schema.sql`・`rules/`・`config.example.json`）を同フォルダ直下に置く

## sqlite3 CLI（任意）

- sqlite.org の snapshot 単体バイナリのみ使用（第三者ミラー・バンドル版は不使用）
- DB 点検・schema 検証用。アプリの動作には不要

## YARA-X zip モジュール（実測フィールド）

`yr dump --module zip` で確認した実際のフィールド名（文書推測でなく実測）。規則はこれに従う:

- `zip.is_zip`
- `zip.entries[i].file_path`（`name` ではない）
- `zip.entries[i].compression`
- `zip.entries[i].uncompressed_size`
- `zip.entries[i].compressed_size`

配列反復は `for any entry in zip.entries : (expr)`。正規表現デリミタと衝突するため割り算は使わず `a > 100 * b` 形式で書く。

## gg.js（hitomi の sharding 表・client ロジックの根拠）

- 取得元: `https://ltn.gold-usergeneratedcontent.net/gg.js`（2026-11-01）
- SHA-256: `f388cd6d64e0b845016354355b6de02eb117c81cf29edcc5c7a485b76f19e489`
- `m(g)` の 0 集合（2112 件）を `docs/ai/tmp/gen_ggset.py` で `app/ggset.go` に生成。endpoint 実測値（`w2/1793469601/2488/<hash>.webp` = HTTP 200）で照合
- 更新は明示操作とし、このdigest と生成物を同じ日に更新

## 不使用

- ClamAV の非公式 Windows ビナル（正典はソース配布、公式 installer なし）
- `yextend`（Perl 依存）
- 第三者ミラー・ブログ配布 exe、署名不明 exe

## 単体 exe の受入手順

1. GitHub API の `digest`（SHA-256）と照合。attestation があれば `gh attestation verify` で確認
2. Authenticode 署名確認（`yr` は unsigned が正常。判断基準は provenance と digest）
3. 取得直後に自前規則（PE 系）で 1 回 scan
4. 更新は明示操作とし、このファイルの版記録を変更
