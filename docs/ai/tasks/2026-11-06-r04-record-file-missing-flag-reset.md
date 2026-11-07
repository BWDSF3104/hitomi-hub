# R-04: recordFile が missing_at_verify をリセットせず消失検出が永続的に機能しない

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: external_removal 記録後に同 path 再 record で missing_at_verify = 0 へ戻り、verifyMissing が再度検出。記録: R-04

## 意図 / 背景

- `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` の R-04（判定: 確定）に対する読み取り調査と実測確認の記録。ソースコードは変更しない（変更したのは本記録と `docs/ai/tmp/assess-20261106-1056/` のみ）。
- 指摘: `recordFile` の upsert が `missing_at_verify` を更新しないため、1（external_removal）になった行は `verifyMissing` の走査条件（`missing_at_verify = 0`）から永久に外れ、同 path の以後の消失を検出不できる。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- Go: `recordFile` の upsert SET 句は `scan_status`・`scan_source`・`scan_time`・`sha256`・`magic`・`gallery_id` のみで `missing_at_verify` を触らない（`app/main.go:578-586`）。`verifyMissing` は `missing_at_verify = 0` の行だけを走査し、実在しない行を 1 へ立てる（`app/main.go:618-643`）。列は `schema.sql:21` の `INTEGER NOT NULL DEFAULT 0`。
- Python 参照実装も同型: `record_file` の upsert は SET 句でフラグを触らない（`src/scan.py:332-341`）、`verify_missing` は `missing_at_verify = 0` だけ走査（`src/scan.py:417-424`）。
- `external_removal` は `verifyMissing` の UPDATE だけが立てる（`app/main.go:636`）。`recordFile` へ `external_removal` を渡す呼び出し口はない（app 内 grep で UPDATE 1 箇所のみ）。
- A-12 の scan skip は `external_removal` を安全側（skip せず再ハッシュ）とする（`app/main.go:566-568`、テスト固定 `app/main_test.go:998-1067`）。したがって external_removal 後の再取得 path は再 scan され `recordFile` upsert に到達する。upsert がフラグを戻さないことが検出自体永続停止の唯一の経路。

### 有効性

- 有効（確定・実測で閉じた）。実測で、① external_removal 記録後に同 path 再 recordしてもフラグが 1 のまま（status と flag が矛盾した状態）、② その後の消失が検出されない（走査から永久除外）を再現。③ 修正案（SET 句へ `missing_at_verify = 0` 追加）で検出が回復することも実測（Verification 参照）。Python 参照実装で実測し、Go は同一 SQL 読み取り一致（実測は関数外-export のため読み取り確認として区別して記録）。

### 対応の実現可能性と推奨修正

- 推奨案 A: `recordFile` の `ON CONFLICT(path) DO UPDATE SET` へ `missing_at_verify = 0` を追加（`app/main.go:581-583`）。参照実装対称で Python `record_file`（`src/scan.py:336-338`）にも同追加。
  - 根拠: `recordFile` は scan/download 経路での呼出しであり、呼び出し時点でファイルは存在（read/write 済）。再 record はファイル存在の証拠であり、フラグ意味（「前回の verify で実在確認できた行」の走査キー・実測上は「最後に verify で消失確認された」状態）と整合。`external_removal` を `recordFile` へ渡す経路がないため誤リセット案例がない。
- 代替案 B: `verifyMissing` の `= 0` フィルタを外して全行走査。検出は回復するが全行 stat コスト、フィルタ最適化の喪失、flag=1 かつ status 新値の矛盾行を残す。非推奨。

### 副作用・影響範囲・歴史的記録の照合

- 「外部削除の歴史的記録が消える」意味の照合: `files` は正典上 history 表ではなく current state。`scan_status` の語彙固定（`docs/ai/decisions/2026-11-01-hitomi-hub-scan-layer-portable.md`）は現在状態であり、`external_removal` の事実は修正前から再 record 時点で既に失われる（upsert の status 上書きが既存挙動、テスト固定済み）。`usage-memo.md`（「`--verify` で消失確認」）と `architecture.md`（「欠損を external_removal として記録するのみ」）はフラグを history と定めていない。したがって案 A は新たな不可逆記録変更ではなく、修正前から存在した状態矛盾（flag 残存・status 更新）の解消。history 保存が必要なら別設計（audit 表）であり本項目外・未承認の論点。UX 影響なし（フラグは UI 露出なし、本項目は UI 項目ではないため画面確認該当なし）。
- schema 変更・migration なし。既存テスト固定への影響: `tests/test_scan.py::test_status_constraint_and_verify_missing` は verify 直後の flag=1 を assert するのみで案 A の影響なし。既存テストに「再 record でリセットされない」固定はない（Go 側テストでも確認なし）。

## 実装（2026-11-07・試行1）

- 案 A を適用（正典: 本記録の推奨案＋ユーザー承認済み判断「歴史記録の扱い（update・insert-only）は承認済み」＝本記録の照合結論どおり update 側）。
  - `app/main.go` recordFile の `ON CONFLICT(path) DO UPDATE SET` へ `missing_at_verify = 0` を追加（`app/main.go:583`）
  - `src/scan.py` record_file の SET 句へ `missing_at_verify = 0` を追加（`src/scan.py:338`）
- 矛盾記録: 決定文書 `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` に R-04 の個別項目記載がない（標題は R-05・R-08・R-11・R-13・R-15・R-20）。採用文書は本記録（日付新・内容一致）。承認内容は委譲記載の「歴史記録の扱い＝承認済み」＋本記録の照合結論（files は current state・案 A は不可逆記録変更ではない）を正典として採用した。
- schema 変更なし。テスト新規追加なし（承認範囲は上記2ファイル）。
- UI 項目ではない（フラグの UI 露出なし）ため画面確認該当なし。

## Verification

- 実測（scratch 一時 DB・既存テストと同形状 `scan.open_db` + `scan.ensure_schema(conn, schema.sql)`、実際の完成フォルダ・実 DB 不使用）: `docs/ai/tmp/assess-20261106-1056/r04_probe.py`（2026-11-06 20:41:04–20:41:06、1.78 s）
  - step1 record（ファイル存在）→ verify: 0
  - step2 削除 → verify: 1、row `('external_removal', 1)`
  - step3 再取得 → 同 path 再 record: row `('not_evaluated', 1)`（flag リセットされない・status と flag 矛盾）
  - step4 再削除 → verify: 0（検出されない・永久除外を再現）
  - step5 案 A 相当の upsert（SET 句に `missing_at_verify = 0` 追加）で再 record: row `('not_evaluated', 0)`
  - step6 verify: 1（`('external_removal', 1)`・検出回復）
- `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 24.822s`（20:41:05–20:41:32）。この挙動を固定するテストなし。
- `python -m pytest tests/ -v`: 54 passed / 1 failed in 9.38 s（20:41:39–20:41:49）。R-04 関連 `tests/test_scan.py::test_status_constraint_and_verify_missing` は PASS。1 件の失敗は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（project `AGENTS.md` に `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` のポインタ文字列を要求）で、disk 上の `AGENTS.md` に該当文字列なし（`Select-String` 一致 0・file LastWriteTime 2026-11-06 17:17）。R-04 と無関係な既存失敗として記録（この項目では未処置・未触）。
- ソースコード変更なし。`git status --short`: docs/ai 配下の記録のみ（本項目の追記は本記録）。

### 実装後の実測（2026-11-07・試行1）

- Python 参照実装（修正後・scratch 新規 DB `r04_accept_py.db`、`docs/ai/tmp/assess-20261106-1056/r04_accept_py.py`）:
  - step1 record+verify（ファイル存在）: 0、row `('not_evaluated', 0)`
  - step2 削除 → verify: 検出 1、row `('external_removal', 1)`
  - step3 再取得 → 同 path 再 record: row `('not_evaluated', 0)`（フラグリセット確認）
  - step4 再削除 → verify: 検出 1（再度検出・合格条件充足）、row `('external_removal', 1)`、PASS（1.62 s）
- Go 実機（修正後 build 単体 exe `docs/ai/tmp/assess-20261106-1056/r04_check.exe` を scratch root 上で CLI `scan -verify` 実行・`-no-vt`・実 DB 不使用、`r04_go_check2.py`・1.08 s）: step1 `('not_evaluated', 0)` → step2 `('external_removal', 1)`（verify 検出）→ step3 scan walk 再 record `('not_evaluated', 0)`（A-12 skip は external_removal 行を skip しないため upsert に到達）→ step4 verify 再検出 `('external_removal', 1)`。PASS。
  - Go 検証スクリプトの初回実行は path 区切り（`\` vs `/`）と fake magic（magic_mismatch hit）で 1 度失敗。検証スクリプト側の問題でコード修正ではないため修正・再検証試行には数えない（コードの試行回数は 1）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet 出力なし（合格）、`ok hitomi-hub/app 25.276s`。
- `python -m pytest tests/ -v`: 55 passed / 1 failed in 12.06 s。R-04 関連 `tests/test_scan.py::test_status_constraint_and_verify_missing` は PASS。1 件の失敗は既存 known-issue KI-20261106-04（`tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`・AGENTS.md ポインタ）で本項目と無関係（未処置・未触）。
- 合格条件充足: external_removal 記録後に同 path 再 record で `missing_at_verify = 0` へ戻り、verifyMissing が再度検出（Go・Python 両経路で実測）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
