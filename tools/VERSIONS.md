# 単体 EXE の版固定（digest 検証済みのもののみ）

フォルダコピーで動く形を維持するため、exe はコミットしない（`.gitignore` の `tools/*.exe`）。このファイルに版と digest を記録し、取得時に照合する。

## yr（YARA-X CLI）

- 版: **v1.22.1**（2026-10-26 release）
- asset: `yara-x-v1.22.1-x86_64-pc-windows-msvc.zip`
- 公開 SHA-256: `dbe8c1ec5d59d8a81c777d949c518fb5e75f1026d3419bca7817727c5d47b327`
- 出典: VirusTotal 公式リポジトリの release（github-actions 発行。`gh attestation verify` で provenance 確認可）
- 用途: 構造規則 `scripts/hitomi_hub/rules/structural.yar` の実行引擎。任意（単体 exe、ランタイム依存なし）
- 留意点: このビルドは msvc target。クリーン環境では `vcruntime140.dll`（VC++ ランタイム）を要求する可能性がある。純粋なコピペ運用が優先なら `yr` なしで `scan.py` の構造検査のみで完結させられる
- 実行時の上限フラグ: `--timeout 10 --max-rules 200 --cpu-limit 50 --output-format ndjson`

## sqlite3 CLI（任意）

- sqlite.org の snapshot 単体バイナリのみ使用（第三者ミラー・バンドル版は不使用）
- DB 点検・schema 検証用。アプリの動作には不要

## 不使用

- ClamAV の非公式 Windows ビナル（正典はソース配布、公式 installer なし）
- `yextend`（Perl 依存）
- 第三者ミラー・ブログ配布 exe、署名不明 exe

## 単体 exe の受入手順

1. GitHub API の `digest`（SHA-256）と照合。attestation があれば `gh attestation verify` で確認
2. Authenticode 署名確認（`yr` は unsigned が正常。判断基準は provenance と digest）
3. 取得直後に自前規則（PE 系）で 1 回 scan
4. 更新は明示操作とし、このファイルの版記録を変更
