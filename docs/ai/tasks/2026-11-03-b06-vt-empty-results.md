# B-06: VT の空 last_analysis_results が pass

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: B-06 実施完了（src/scan.py は無変更・premise は 8288151 で解消済みと確認・mock 固定テスト 5 件で合格条件を固定）。Go parity 差 app/vt.go:150（キーあり・空 map の 200 応答で pass）は親の判断要。live 200 応答の実測は未実施（hash-only 1 回は 404 で使用済み）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

原記録（調査段階・静的）: `vt_lookup` が `last_analysis_results` を iterate するだけで `stats` を参照せず、空 results の file が `pass` として return し VT cache に `pass` が保存される、という判定。

実装前の再確認で、この前提は HEAD `31f8342` では成立しないことを確認（実装変更なし、テストのみ）。

- `src/scan.py:246` は `attributes.last_analysis_stats` を参照し、`src/scan.py:253` の `if not results and positives == 0` で空 results を `not_evaluated` へ返す。この分岐は commit `8288151`（外部レビュー是正）で追加済みで、review 基準 commit `349778d` の祖先。原記録の記述は `8288151` 以前の実装に対するもので stale。
- cache へ `pass` を書かない条件も既存で満たす: `process_file` は `pass`/`hit` のときだけ `cache_store` する（`src/scan.py:452`-`454`）。`not_evaluated` は cache へ書かず findings へ `api_error:not_evaluated` を記録（`src/scan.py:462`-`463`）。
- VT cache の TTL は実際 `SCAN_CACHE_TTL_SECONDS = 30 日`（`src/scan.py:32`）、negative `2 時間`（`src/scan.py:33`）。原記録の「60 日」は誤り（Go も 30 日: `app/main.go:328`）。意味は変更していない。
- file レベルの `scan_status` は構造 verdict が優先（clean cbz は VT 不参加でも `pass`）。合格条件の `not_evaluated` は `vt_lookup` の返り値であり、この意味は承認範囲外なので変更していない。

変更は `tests/test_scan.py` へ合格条件固定テスト 5 件を追加（mock fetch・外部 API なし）:

1. `test_vt_empty_results_with_stats_total_is_not_evaluated` — `stats.total=100` + `last_analysis_results {}` → status `not_evaluated`、cache 行なし
2. `test_process_file_vt_empty_results_does_not_store_pass` — `scan_cache` 0 行、findings `virustotal / api_error:not_evaluated`
3. `test_vt_positives_hit_and_cache_verdict_unchanged`（回帰: `positives > 0` は `hit`）
4. `test_vt_404_unknown_to_vt_path_and_negative_cache_unchanged`（回帰: 404 → `unknown_to_vt`、negative cache 保存）
5. `test_pass_verdict_cache_keeps_positive_ttl_semantics`（回帰: `pass` は positive TTL、negative TTL 適用外）

`src/scan.py` は無変更。

判断必要な点（スコープ外・未実施）: `app/vt.go:150` は `last_analysis_results` キーの「有無」で判定するため、キーあり・空 map の 200 応答では Go 側 `pass` となり Python（`not results`）と分岐する。parity 差として親の判断要。

## Verification

- `python -m pytest tests/ -v` → 38 passed in 3.95s（baseline 33 + 追加 5。`tests/test_canonical_build_freshness.py` を含む。UI 未変更のため exe 再 build 不要。Go 変更なし）
- mock 実測（テストで固定）: 空 results + `stats.total=100` → `not_evaluated`・`scan_cache` 0 行（`pass` 保存なし）。回帰 3 件合格。
- 補強確認（live・hash-only 1 回のみ・key は `scratch/virustotal-api.key`（gitignore 済）から env `VIRUSTOTAL_API_KEY` へ読み、記録・テスト・commit に key 文字列なし）。スクリプト `docs/ai/tmp/assess-20261103-1220/b06_live_hash_check.py`（使い捨て・非 commit）: hash `3b89285e8428d2f167ac7c60afbdc1efd488ca99005764d05a7aaa837dc67dbb`（`hitomi.db` 記録の fixture パス）→ HTTP 404、`stats` 空、`last_analysis_results` 0 件、verdict `unknown_to_vt`、elapsed 1.08s。upload なし（hash-only）、呼出 1 回。
- 上記 1 回で許容回数を使用済みのため、`stats.total > 0` + 空 results の live 200 応答の実測は未実施（補強確認は未検証）。合格の根拠は mock 固定テスト。
- commit: 単独 commit（`tests/test_scan.py` + 本記録 + `current-task.tsv` 参照行削除）。commit ID は結果報告どおり（`task_manager.py end` で close）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
