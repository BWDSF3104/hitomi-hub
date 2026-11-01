# YARA と構造検査の役割分担・main ループの gate

**Date**: 2026-11-01
**Status**: 決定・実装済み

## 決定

- YARA は scan target の**全ファイル**で走らせる（構造検査で hit が出たか否かに依存しない）。main ループの条件を `status != hit` から `YaraGate(ext, cfg)`（Python `yara_gate`）に変更。
- 役割分担:
  - 構造検査（`scan.go` / `scan.py`）: 単独の key / entry 条件（magic 判別、zip entry 名、PDF key 単独、比率・件数上限）
  - YARA（`rules/structural.yar`）: 複合条件・参照 entry（単独では意味が確定しないもの）

## 追加した YARA 固有規則

- `Zip_Reference_Entry` — entry 名が `.url` / `.ini` / `.desktop`（参照先を指す entry）
- `PDF_OpenAction_Launch` — `/OpenAction` と `/Launch` の併存
- `PDF_AA_Script` — `/AA` と `/JS` の併存
- `PDF_XFA_Embedded` — `/XFA` と template stream の併存

構造検査側は単独条件のまま（`PDF_Structural_Risk` は PDF magic のみ）。

## findings 記録の語彙

- `findings.rule` = engine 名（`structural` / `yara` / `virustotal`）
- `findings.detail` = 検知内容（規則名または structural の文字列）
- 同一ファイルで structural hit がある場合でも yara findings を追加記録する（engine 列で区別するため衝突しない）

## 検証

- `python -m pytest tests/ -q`（hitomi-hub）→ 16 passed。gate・YARA 固有規則・hit 済みファイルへの yara 記録を確認
- `go vet ./...` clean、`go test -count=1 ./...` → ok。`TestYaraGateRunsForAllScanTargets`、`TestReferenceEntryIsYaraOnlyFinding`
- 端到端（`docs/ai/tmp/yaracheck.py`・fixture cbz）: `yara | Zip_Reference_Entry` + `yara | Zip_Path_Traversal` + `yara | Zip_Executable_Entry` + structural findings が記録される

関連: `docs/ai/known-issues/archive/2026-11-01-yara-no-op-in-main-loop.md`
