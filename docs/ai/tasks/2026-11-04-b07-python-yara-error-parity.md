# B-07残件①: Python 参照実装 YARA error 表現を Go 表現へ

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 委譲（第11実行②・単独 commit・テスト固定）

## 意図 / 背景

B-07（close `bfdc83e`・Go 側）の残件①。Python 参照実装（`src/scan.py` process_file）は YARA error → `scan_error` のまま（Go の `not_evaluated` 維持＋`scan_source`=`structural+yara_error` と対応付が drifted）。推奨承認（`decisions/2026-11-03-remaining-item-recommendations-approved.md`）で Python 側を Go 表現へ揃える。export skip 条件（hit/scan_error）は維持・テスト固定。B-07 残件②（YARA error ファイルが VT gate に入り得る副作用）は decision 2026-11-02 #3 と整合済みで本項目では変えない。

## 調査結果 / 変更内容

- Go 正典表現（参照・読取のみ）: `app/main.go` `processFileWithYara` の YARA error 分岐は `status != hit` のとき `not_evaluated`（構造 pass を `scan_error` へ上書きしない）＋ `scan_source = structural+yara_error`（retry 用表現）。findings 追加なし・export skip 条件（hit/scan_error）不変（`app/scan_test.go` で固定済み・`bfdc83e`）。
- Python 変更（承認範囲）: `src/scan.py` `process_file` の YARA error 分岐で `status = "scan_error"` → `status = "not_evaluated"` へ（hit 上書きなし）。`source = "structural+yara_error"` は維持。VT gate 条件（`status not in ("hit", "scan_error")`）はそのまま → YARA error ファイル（`not_evaluated`）は VT gate に入る（decision 2026-11-02 #3 と整合・現状許容）。export skip 条件（hit/scan_error）は変更なし（Python 参照実装に export 経路なし。Go 側 `TestExportCBZExportsStructuralPassWithYaraErrorAsNotEvaluated` 固定済みで YARA error は export される＝同一挙動）。VT gate・quota・TTL（30日/2時間）・negative cache の意味は変更なし。YARA 規則・config・viewer・フル画像経路非触。
- テスト: 既存 `test_process_file_yara_exit1_error_is_scan_error` を Go 表現へ更新し `test_process_file_yara_exit1_error_keeps_not_evaluated_and_not_finding` へ改名（`not_evaluated`＋`scan_error` 0＋findings 0＋source＋export skip 集合非所属）。追加固定 5件（mock runner 実測）: `test_process_file_yara_execution_failure_keeps_not_evaluated`（runner 例外）、`test_process_file_structural_hit_keeps_hit_on_yara_error`（hit 維持・findings 1・fixture `../evil.txt` は Go 同一）、`test_process_file_bad_zip_scan_error_keeps_scan_error_without_yara_call`（YARA 呼出 0）、`test_process_file_yara_success_no_rules_keeps_pass`、`test_process_file_yara_error_file_enters_vt_gate`（残件②の gate 入りを mock 1 回呼出しで固定・negative cache 行維持）。
- 読了 decision/記録: `2026-11-03-remaining-item-recommendations-approved.md`（B-07残件①）、`2026-11-03-implementation-judgment-approval.md`（B-07）、`2026-11-02-high-priority-fix-methods-and-acceptance.md`（#3）、`2026-11-01-yara-role-division-and-gate.md`、B-07 個別記録 `2026-11-03-b07-yara-failure-overwrites-pass.md`。フル画像・viewer 非触のため AGENTS.md 作業前チェックの viewer decision 2件は非該当。

## Verification

- ルートで `python -m pytest tests/ -v` → **46 passed 7.69s**（委譲前 baseline 41 passed の無損＋追加 5件・`test_canonical_build_freshness.py` 含む・経過 7.69s）。Go 変更なしのため `go vet`/`go test` 該当なし。server 起動・停止なし・実機 UI 確認該当なし。
- 途中経過（記録）: 1回目 45 passed 1 failed（8.84s）— `test_process_file_structural_hit_keeps_hit_on_yara_error` の fixture を `../evil.exe` にすると `path_traversal`＋`executable_entry` で findings 2。Go 同一 fixture `../evil.txt` へ修正後全通過。
- mock 実測（YARA tool 実 binary 経由ではなく mock runner/fetch 固定・既存方針どおり）:
  - exit1 error → status `not_evaluated`・summary `not_evaluated`=1・`scan_error`=0・findings 0・`scan_source=structural+yara_error`・skip 集合（hit/scan_error）非所属（export される＝Go 同一挙動）
  - runner 例外 → 同表現（`not_evaluated`＋`structural+yara_error`・findings 0）
  - 構造 hit（`../evil.txt`）＋YARA error → `hit` 維持・findings 1・source `structural+yara_error`
  - bad_zip → `scan_error` 維持・YARA 呼出 0・source `structural`
  - YARA 成功・rule なし → `pass` 維持・source `structural`
  - YARA error＋VT enabled → VT gate に入る（mock fetch 呼出 1・`/files/<sha>`）・status `not_evaluated`・negative cache 行（`unknown_to_vt`）維持（decision 2026-11-02 #3 整合）
- 回帰確認（既存テスト・無損）: positives>0 → hit（`test_vt_positives_hit_and_cache_verdict_unchanged`）、404 → `unknown_to_vt`＋negative cache（`test_vt_404_unknown_to_vt_path_and_negative_cache_unchanged`）、YARA error なしで構造 pass → `pass`（`test_clean_cbz_is_pass`）。TTL 30日/2時間意味維持（`test_pass_verdict_cache_keeps_positive_ttl_semantics`）。
- 未確認点: なし（YARA tool 不在環境の実測は方針どおり mock で固定。live YARA 実行は不要）。判断持ち帰り: なし。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end 経由）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
