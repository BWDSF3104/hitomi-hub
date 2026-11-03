# A-13: ScanPDFKeys が key ごとに string(data) を作り直す

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（本項目分単独 commit で確定）。記録の実測修正: pdfKeys 7 key / 最大 14MB / 該当行 app/scan.go:175。テストは測定点のみ（範囲外の baseline 測定は削除）。次の対応なし

## 意図 / 背景

`ScanPDFKeys` が 1 回の読み込み（最大 `MaxScanBytes`）に対して key ごとに `string(data)` 変換を行い、key 数分だけ 2MB 相当の一時割当を重ねていた。割当を 1 回（読み込みバッファ）へ戻し、key は `[]byte` で保持する。

## 調査結果 / 変更内容

実測で確認（実装前の記録との差分を以下に明記）:

- `app/scan.go` の該当行は `app/scan.go:175`（記録は `164` としていた。コードが移動済みの漂移）。
- `pdfKeys` は **7 key**（記録は 10 としていた）。よって一時割当の上限は 2MB×7 = **14MB**（記録の 20MB ではない）。`MaxScanBytes` 既定 2097152（2MB）は記録どおり（`app/main.go:95`）。
- `pdfKeys` の参照箇所は `ScanPDFKeys` 内のみ（grep 確認: `app/scan.go` 以外に参照なし）。YARA rule・UI への影響なし。

変更（承認範囲内: `app/scan.go` と `app/scan_test.go`）:

- `pdfKeys` を `[][]byte` に変更（`app/scan.go:37`）。
- `ScanPDFKeys` は読み込んだ `[]byte` をそのまま `pdfKeyFindings(data)` へ渡し、`bytes.Contains(data, key)` で判定。key ごとの `string(data)` 生成を除去。findings 名は hit 時だけ `string(key)`（`app/scan.go:180-189`）。
- `found := []string{}` の返却 semantics は維持（nil 化しない）。判定結果は byte 単位比较のため非 UTF-8 データでも `strings.Contains(string(data), key)` と同一。

合格条件と測り方:

- 既存テスト（`TestStatusSemantics` の pdf hit/pass、`tests/test_scan.py::test_pdf_viewer_action_hit`）通過。Go テスト名 `ScanPDFKeys` は存在せず、既存の PDF key カバーは `TestStatusSemantics`（`app/scan_test.go`）と Python 参照実装テスト。記録の「existing test `ScanPDFKeys`」はこの既存 PDF key カバーを指すと解釈した。
- 追加テスト `TestScanPDFKeysDoesNotCopyDataPerKey`: 非 UTF-8 バイナリ 2MB（MaxScanBytes 既定相当）に対し `testing.AllocsPerRun` で key 走査の割当 **0** を検証。全 7 key の hit 判定（findings 名 `pdf_object:<key>`）を byte 単位で確認し、`ScanPDFKeys` 経由のファイル読みでも同一結果であることを確認。テストは測定点のみ（修正前方式の allocation をテスト内で測る baseline 部分は「測定対象の変更」として範囲外、削除）。
- canonical exe rebuild 後 `python -m pytest tests/ -v`（freshness 含む）通過。UI 未変更のため server 実機確認は不要（判定は UI 関連ではない）。

## Verification

実施順（2026-11-03・実行固有ID assess-20261103-1220）:

1. `app` で `go vet .` → clean（出力なし）
2. `app` で `go test -count=1 -timeout 300s . -run "TestScanPDFKeysDoesNotCopyDataPerKey|TestStatusSemantics" -v` → PASS 0.197s（`TestScanPDFKeysDoesNotCopyDataPerKey` 0.03s・10 runs 平均の key 走査割当 0、`TestStatusSemantics` 0.01s）
3. `app` で `go test -count=1 -timeout 300s .`（full suite）→ `ok hitomi-hub/app 67.121s`
4. canonical exe rebuild: `app` で `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 成功（エラー出力なし）。初回に repo root で実行して `cannot find main module` で失敗、`app` で再実行（正典 build コマンドは app ディレクトリ前提）。
5. ルートで `python -m pytest tests/ -v` → 40 passed in 7.17s（rebuild 後。`test_canonical_build_freshness.py` 2 件含む）

メモ: テスト内で修正前方式の allocation を測る比較 baseline 部分を追加した으나、測定対象の変更（範囲外）と判断して削除。削除後に vet / full suite / rebuild / pytest を再実行し、上記の数値で確定した。

未確認点:

- canonical exe の Go 差分そのものは exe 内容から検証不能（freshness テストは `app/ui.html` のバイト埋め込みのみ検証）。rebuild は現行ソースから実行済みという事実だけ記録する。
- server 実機確認は項目性格上不要（UI 未変更）。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager.py end` で 2026-11-03 実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [ ] `backlog.md` の完了項目を削除（該当なし: 本プロジェクトに `docs/ai/backlog.md` は存在せず、項目は assess-20261103-1220 の plan 側管理）
