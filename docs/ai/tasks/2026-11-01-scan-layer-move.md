# hitomi-hub 分割とスキャン層の移動（単一正典リポジトリ化）

**Status**: 進行中
**Date**: 2026-11-01
**Next Action**: 単体静的バイナリ化（Go）の承認取得。yr 導入時は規則実行検証

## 意図 / 背景

hitomi.la 用の作業（検索・閲覧・DL・既読・お気に入り・構造検査）を「コピペで動く完成フォルダ」として独立させたい。目的・ライセンス・リリース単位が汎用作業用の Common_Operations（正典ツール群）と違うため、リポジトリを分割した。単体 exe は版固定＋GitHub SHA-256 digest 検証済みのもののみ受け入れる方針も同じ理由から（正典ツールと混在させない）。

分割先 `S:\Documents\Kilo_Commons\hitomi-hub`、public・MIT。Kilo_Commons 自体は git リポジトリではないため nested repo 問題はなく、正典ルート判定は `hitomi-hub/.git` に解決される。正典ツール（`task_manager.py`・`kilo-safe-*` 等）は Common_Operations の `bin/` を PATH 経由で参照し、プロジェクトごとのコピーは作らない。

## 調査結果 / 変更内容

移動したファイル（Common_Operations からはゴミ箱経由で空ディレクトリを削除）:

- `src/scan.py`、`src/schema.sql`、`src/config.example.json`、`src/rules/structural.yar`（旧 `scripts/hitomi_hub/`）
- `tests/test_scan.py`（旧 `tests/test_hitomi_scan.py`。SCRIPT_DIR を `parents[1]/src` に修正）
- `tools/VERSIONS.md`（単体 exe の版固定と digest）
- `docs/ai/decisions/2026-11-01-hitomi-hub-scan-layer-portable.md`

新規作成: `LICENSE`（MIT）、`README.md`、`AGENTS.md`、`.gitignore`（exe・`*.db`・scratch/ 非追跡）、`docs/ai/`（memory-bank-guide・architecture・backlog・api-rate-limits・workflows）。

Common_Operations 側は `.gitignore` の `tools/*.exe` 行を取り消し（移動により該当ディレクトリが消滅）。

## Verification

- `python -m pytest tests/ -v`（本リポジトリ）→ 13 passed (0.93s)。うち `test_run_yara_rules_on_fixture` は `tools/yr.exe` を実際に呼ぶ integration test（`tools/yr.exe` が無い環境では skip）
- `python -m pytest tests/ -q`（Common_Operations・移動後）→ 109 passed (6.38s)
- `git status` で移動後の混入なし（exe・DB・scratch/ は非追跡）

## checkpoint 2（2026-11-01）: 単体 exe の取得と規則の実行検証

- `yr` v1.22.1 を公式 release から取得。GitHub API の asset digest と SHA-256 一致（`dbe8c1ec…b327`）。`gh attestation verify` は attestation 未登録（404）→ 判断根拠は digest 一致のみ。導入先 `tools/yr.exe`（非追跡）、`yr --version` = `yara-x-cli 1.22.1`
- YARA-X の zip モジュールフィールドを実測（`file_path`・`uncompressed_size`・`compressed_size`）。規則を実測ベースで書き換え
  - `Zip_Path_Traversal`（`..`、`/`、`\`）
  - `Zip_Executable_Entry`（exe/dll/scr/jar/js/vbs/ps1/bat/cmd/lnk）
  - `Zip_Decompression_Ratio`（`uncompressed_size > 100 * compressed_size`。`/` は正規表現デリミタと衝突するため割り算は使わない）
  - `MZ_Header_At_Offset0`（`$mz at 0`）。フル PE ヘッダ検査は scan.py 側のまま
  - `PDF_Structural_Risk`（`uint32be(0) == 0x25504446`）
- `--max-rules` は `yr` に存在しない（classic YARA のフラグ）。scan.py を `yr scan` の有効フラグ（`--timeout`/`--cpu-limit`/`--skip-larger`/`--fast-scan`）に修正。ndjson 解析を `parse_yara_output()` に分離
- 規則実行結果（fixture・`--fast-scan --timeout 10 --cpu-limit 50`）: cbz で `Zip_Path_Traversal`・`Zip_Executable_Entry`・`Zip_Decompression_Ratio`、拡張子偽装 PNG で `MZ_Header_At_Offset0`、PDF で `PDF_Structural_Risk`
- scan.py 端到端（`yr` 設定込み）: hit 3 / pass 0 / scan_error 0。findings に structural 8 件

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
