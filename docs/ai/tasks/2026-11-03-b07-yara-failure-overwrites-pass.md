# B-07: YARA 一時失敗で構造 pass が scan_error

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/main.go`（`processFileWithYara`, 513-517 相当。実装時点の実行行は 564-568）は structural pass の file でも `runYara` が error を返すと `scan_status = scan_error`（`scan_source = structural+yara_error`）へ上書きし、export は `scan_error` を skip。実測（scratch root, `yr` を存在しない binary へ）: `Galleries\105\clean.cbz`（structural `pass`）が `scan_error` / `structural+yara_error` になり、export summary の `pass` が 0。対応: `not_evaluated` を維持し `scan_error` は「判定不能の上位」ではなく別列（または retry 用列）で表現。合格条件: YARA error で `scan_status` が `not_evaluated`、export に `skipped.scan_error` が増えない。推定規模: 小〜中（テスト assertion 更新が必要）。

### 修正方法（実装前に確定）

- `processFileWithYara` の YARA error 分岐: `runYara` が error を返したとき `scan_status` を `scan_error` へ上書きしない。structural pass の file は `not_evaluated`（YARA 部分の判定不能＝合格条件で固定）。`hit` は `hit` のまま（hit 上書き禁止は既存分岐を維持）。
- YARA 失敗の表現は既存 `scan_source` 列（`structural+yara_error`）を維持 = retry 判定用の別列。schema 変更なし、findings 追加なし（既存「YARA error は findings を増やさない」assertion 維持）。
- export skip 条件（`hit` / `scan_error`）と export summary の `pass`（entries）意味は変更しない。`not_evaluated` は export 対象なので、YARA error で export に `skipped.scan_error` が増えず、entries が 0 にならない。
- 影響範囲（整合確認済み）: YARA error は `scan_error` ではなくなるため VT gate（`status != hit && status != scan_error`）に入る。これは `docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md` #3（`scan_error` は構造解析不能の意味に限定・VT 失敗は findings 記録・export skip は構造 status 基準）と整合。実測は VT 無効化（呼出 0）で行う。B-06 の VT 判定（`4f14d00` の mock 固定テスト）は YARA error 経路と別分岐で不変。YARA 規則 `rules/structural.yar` 変更なし。
- テスト: 既存 assertion 更新（`TestProcessFileYaraExecutionFailureIsScanError`、`TestProcessFileYaraExit1ErrorIsScanErrorAndNotFinding` → status `not_evaluated`、summary `not_evaluated`）。追加固定テスト: 回帰（hit+YARA error は hit、bad_zip `scan_error` は不変・YARA 呼出 0、YARA 成功 rule なしは `pass`）、YARA error の clean cbz が export で skip されない（entries=1、`skipped` に `scan_error` なし）。

## Verification

- 実装: `app/main.go` `processFileWithYara` の YARA error 分岐で `scan_status` を `not_evaluated` へ（hit 上書きなし・findings 追加なし）、`scan_source = structural+yara_error` を失敗表現列として維持。export skip 条件変更なし。
- `app` で `go vet .` clean・`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 38.079s`（変更前 baseline: ok 37.887s）。
- ルートで `python -m pytest tests/ -v` → 38 passed in 3.54s（`tests/test_canonical_build_freshness.py` 含む。Python 参照実装側の YARA error マッピングはスコープ外・不変）。
- canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` で再 build。canonical server（既定 http://127.0.0.1:8731）は開始時未稼働のまま（起動・停止なし）。製品 output の exe 再 build 以外汚染なし、永続 cache 非使用。
- 実測（scratch root `C:\Users\fujim\AppData\Local\Temp\kilo\b07-scratch`、config `yr` = 存在しない binary `missing-yr.exe`、VT 無効。fixture `Galleries\105\clean.cbz`（entry `ok.txt`、構造 pass）、scratch script `docs/ai/tmp/assess-20261103-1220/b07_make_fixtures.py` / `b07_show_db.py`）:
  - canonical exe `scan`: `{"cached":0,"hit":0,"not_evaluated":1,"pass":0,"scan_error":0}`（経過 0.13s）→ `scan_error` が増えない、`not_evaluated`=1
  - canonical exe `export`: `{"entries":1,"skipped":{}}`（経過 0.03s）→ export に `skipped.scan_error` が増えない、entries（export summary の `pass` 意味＝書き出された entry）= 1 で 0 にならない
  - DB files 行: `Galleries\105\clean.cbz` → `scan_status = not_evaluated` / `scan_source = structural+yara_error`、findings 0
- 合格条件固定テスト（scan_test.go）: `TestProcessFileYaraExecutionFailureKeepsNotEvaluated`、`TestProcessFileYaraExit1ErrorKeepsNotEvaluatedAndNotFinding`（既存 assertion を status `not_evaluated`・summary `not_evaluated`=1・`scan_error`=0 へ更新）、`TestProcessFileStructuralHitKeepsHitOnYaraError`（構造 hit 不変）、`TestProcessFileBadZipScanErrorKeepsScanErrorWithoutYaraCall`（構造 `scan_error` 不変・YARA 呼出 0）、`TestProcessFileYaraSuccessNoRulesKeepsPass`（YARA 成功時 verdict 不変）、`TestExportCBZExportsStructuralPassWithYaraErrorAsNotEvaluated`（export skip 条件維持・entries=1・skipped 空）。
- 読了 decision（export/scan_status/YARA 規則との整合確認）: `docs/ai/decisions/2026-11-01-yara-role-division-and-gate.md`、`docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md`（#3 の `scan_error`＝構造解析不能限定・export skip 基準と整合。YARA error により VT gate が入る可能性は VT 実呼出なしで確認）。B-06（`4f14d00`）の mock 固定テストは YARA error 経路と別分岐で不変。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
