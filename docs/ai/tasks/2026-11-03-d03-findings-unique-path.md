# D-03: findings の UNIQUE に path がない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

findings の unique key に path が無いため、同一 content の別 path が first row だけ残り、新規 DB には unique index 自体が無い（重複 accumulate）。合格条件（固定）: 新規 DB で `INSERT OR IGNORE` が重複を skip し、scan を 2 回実行しても findings の行数が変わらない。Go 作成 DB と canonical DB で unique index の定義（key に path を含む）が一致する。

## 調査結果 / 変更内容

有効（実測、2 点）。(1) unique index が `(sha256, rule, detail)` で path を含まない（`schema.sql:81`, `app/main.go:299`）→ 同一内容の別 path は first row ののみ。(2) Go が新規作成した DB には unique index が無い：scratch DB `PRAGMA index_list(findings)` = `idx_findings_sha256` のみ、canonical DB には `idx_findings_unique` あり（migrate 済み）。scan を 2 回実行すると findings が 5 → 10 行へ重複（`docs/ai/tmp/assess-20261103-1220/repro_scan.py`）。対応: `ensureSchema` の DDL に unique index を追加（path を key に含めるか別解）。合格条件: 新規 DB で `INSERT OR IGNORE` が重複を skip、2 scan で行数不変。推定規模: 小〜中。

実施（2026-11-03、subagent D-03）。変更:
- `schema.sql:81`（正典データファイル。AGENTS.md 構成上 `src/` 表記だが実ファイルはプロジェクトルート。`schema.sql` のパス差異のみで記録と実コードは一致）→ `ON findings(path, sha256, rule, detail)`。
- `app/main.go`: `schemaVersion` 3→4。`ensureFindingsUniqueIndex`（既存定義を sqlite_master 定義比較で canonical key へ置換、既存行は削除しない）を追加し `ensureSchema` で実行（dataDir の schema.sql コピーが古い場合でも新規 DB に path key の unique index を作成）。`migrateSchema` は重複行 DELETE を除去し定義置換のみ（定義一致なら何もしない、新 key に重複が残れば CREATE 失敗→エラー返却）。
- 設計判断（decisions/ はスコープ外につき親へ報告、この記録が旧 dedup 挙動 supersede）: マイグレーションは非破壊（行削除禁止）。重複行が存在して unique index を作成できない DB は失敗させ、運用判断へ委ねる。
- テスト: `app/main_test.go` の `TestMigrateSchemaCollapsesFindingsDuplicatesAndSetsVersion` を非破壊挙動へ更新（定義置換 test・重複 DB は拒否 test を追加）。`tests/test_scan.py` に key に path を含む定義検証（`PRAGMA index_list`/`index_info`）、別 path 同一 content の別 recording、2 scan 行数不変を追加。

既存 DB 実測（`docs/ai/tmp/assess-20261103-1220/d03_inspect.py` / `d03_migrate.py`、read-only 確認後非破壊迁移のみ実施）:
- canonical `output/hitomi-hub/hitomi.db`: before key=['sha256','rule','detail']（rows=0）→ 定義置換後 key=['path','sha256','rule','detail']、rows 0→0（行数変化なし）。合格条件 2 一致確認。
- root `hitomi.db`（スコープ外につき変更せず）: rows=8・旧 key・重複 0。次のアプリ実行で `ensureSchema` が非破壊で揃える（判断: 親）。
- Go 作成 DB `repro\root\repro.db`: unique index 無し、rows=20、新 key 重複 group=5 → 行削除なければ index 作成不可。copy で実測し失敗を確認、original 未変更（判断要: 重複除去か DB 再生成）。
- 実アプリ経路（現行 code 現 scratch build・dataDir=product 旧 copy 使用）新規 DB 2 scan: findings rows 2→2 不変、`PRAGMA index_list(findings)` = idx_findings_unique(unique) key=['path','sha256','rule','detail']（`d03_scan_check.py`）。合格条件 1 実測。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app）: vet ok 1s / `ok hitomi-hub/app 37.201s`（test 実行 39.3s・新 test 含む）
- `python -m pytest tests/ -v`（ルート）: 29 passed 3.89s（`tests/test_canonical_build_freshness.py` 含む。UI 未変更につき exe 再 build 不要）
- 実測 evidence（経過含む）: `d03_inspect.py`（既存 3 DB の before 状態、read-only）、`d03_migrate.py`（canonical 定義置換 rows 0→0、copy 拒否 0.86s）、`d03_scan_check.py`（scan1 rows=2 → scan2 rows=2、1.01s）。ファイル・出力は `docs/ai/tmp/assess-20261103-1220/` に保存。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager.py end`）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [ ] `backlog.md` の完了項目を削除（該当なし）
