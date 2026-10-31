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

- `python -m pytest tests/ -v`（本リポジトリ）→ 11 passed (0.88s)
- Common_Operations 側は移動後 `python -m pytest tests/ -q` → 109 passed（新テストは本リポジトリへ移動済みのため 120 → 109 に戻る）
- `git status` で移動後の混入なし（exe・DB・scratch/ は非追跡）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
