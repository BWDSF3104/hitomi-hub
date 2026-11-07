# R-02: Go の IsPE が先頭 64 バイト窓外 PE を誤検知（Python D-04 と非対称）

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: e_lfanew >= 64 の実在 PE fixture で mz_without_pe_header 非 hit（Python 版 seek 実装と対称）。記録: R-02

## 意図 / 背景

- 外部指摘（Claude）R-02 の現コード照合と実測による有効性確定。実装は行わない（記録のみ更新）。
- Go の `IsPE` は先頭 64 バイト窓だけ見て `e_lfanew + 4 > len(head)` で false を返す。`e_lfanew` が窓外の実在 PE（例 0x80）で `mz_without_pe_header` が hit する。Python 参照実装は D-04 で seek 判定へ修正済みで、Go 側が非対称。
- 正典基準: `app/scan.go:189` のコメントが「ScanFile mirrors the Python reference implementation's status semantics」と明記。D-04 決定（`docs/ai/tasks/2026-11-03-d04-ispe-header-window.md`）と `src/scan.py` の `is_pe`（`src/scan.py:88-105`）を正典として照合した。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- Go: `IsPE(head []byte)` は `head`（`readHead(path, 64)` の結果、`app/main.go:74-83`）だけ見る。窓外は `e_lfanew + 4 > len(head)` で false（`app/scan.go:103-104`）。`ScanFile` は `magic == "pe" && !IsPE(head)` で `mz_without_pe_header` を append（`app/scan.go:200-201`）。`IsPE` は seek 手段を持たない（`head` のみ引数）。
- Python 参照実装（正典・D-04 修正済み）: `e_lfanew + 4 <= len(head)` なら窓内 slice、窓外は `path` を open して seek → 4 bytes 読み判定。open 失敗（OSError）は False、EOF 超えは read が空になり False（`src/scan.py:96-105`）。`e_lfanew <= 0` は False（`src/scan.py:94-95`）、`len(head) < 0x40` は False（`src/scan.py:91-92`）。
- 呼び出しは Go 全体で `app/scan.go:200` の 1 箇所のみ（grep 確認）。Go テストに `IsPE` / `mz_without_pe_header` の固定テストなし（`*_test.go` grep ヒットなし）。

### 有効性: 有効（実測）

窓外実在 PE での分岐を実測で確定（スクラッチ `docs/ai/tmp/assess-20261106-1056/`）。Python 列は正典 `src/scan.py` の実関数を実行、Go 列は `app/scan.go:95-107`（`IsPE`）と `app/main.go:74-83`（`readHead`）の verbatim copy を単独実行（app/ へのファイル書き・テスト追加が制約で不可のため。機構は実コードの読み取りで確定済み、copy 実行は fixture ごとの帰属の裏付け）。

| fixture | e_lfanew | sig 実在 | Python 実測（scan_file） | Go copy 実測（IsPE） | 分岐 |
|---|---|---|---|---|---|
| f1.bin | 0x80 | 有 | 非 hit（status=not_evaluated） | hit（false） | **分岐** |
| f2.bin | 0x38 | 有 | 非 hit | 非 hit（true） | 一致 |
| f3.bin | 0x80 | 無 | hit | hit（false） | 一致 |
| f4.bin | 0（真の MZ-only） | 無 | hit | hit（false） | 一致 |
| f5.bin | 0x40（=64・実在 PE で最小の窓外） | 有 | 非 hit | hit（false） | **分岐** |
| f6.bin | 0x20000000（EOF 超え） | 無 | hit | hit（false） | 一致 |

- 分岐範囲: `e_lfanew >= 64` の実在 PE（f1・f5）。`e_lfanew` が 61〜63 の fixture は e_lfanew フィールド（0x3C–0x40）自身と signature が重複するため実在 PE ではあり得ない（DOS ヘッダ 64 バイト後の offset が e_lfanew）。当初 f5=61 で生成し不正 fixture として検出、境界を 64 に修正して再実測（自参照 fixture の生成アーティファクトは結論に影響なし）。
- 指摘の例（0x80）は実測で再現。指摘・判定（確定）は有効。
- 引用の細部: 指摘記録（findings R-02 欄）の `app/main.go:193-201` は `app/scan.go:193-201` の誤記（`readHead` は `app/main.go:74-83`）。指摘の主張内容自体は正しい。

### 対応の実現可能性と適切な修正方法（推奨）

- 案 A（推奨）: Python と対称にする。`IsPE` を `IsPE(path string, head []byte) bool`（または `ScanFile` 内）で窓外は seek 判定: `e_lfanew + 4 <= len(head)` で窓内 slice（従来経路維持）、窓外は open → seek → 4 bytes 読み `PE\x00\x00` 判定。open 失敗・read 不足は False。`e_lfanew <= 0` と `len(head) < 64` は False のまま（Python 一致）。呼び出し 1 箇所（`app/scan.go:200`）の更新のみ。テスト: Python の 4 テスト（`tests/test_scan.py:111-150`）を模した Go テスト追加（`make_pe_stub` 相当・窓外 hit 非 hit / 窓内 regression / 窓外 offset 宣言無 signature は hit 維持 / 真の MZ-only hit 維持）。
- 案 B（代替・非推奨）: `readHead` の窓を 0x400 等に拡大して窓内判定。EOF 超え・巨大 e_lfanew をカバーできず、Python 参照実装の窓基準（readHead(64)）と意味がずれ、正典の「Python 参照実装ミラー」方針に反する。

### 副作用・影響範囲

- ScanFile 中に窓外 PE 1 件あたりファイル open + seek + read 4 bytes が 1 回増える（コスト negligible。Python 参照実装が同じ動作を済ませ済み）。
- 影響は scan findings（DB findings 記録）の hit/非 hit の向きのみ。schema・YARA・VT・viewer・UI への波及なし。YARA 規則 `MZ_Header_At_Offset0` は `$mz at 0` のみで「PE 判定は scan.py に残す」と description に明記（`rules/structural.yar:22-33`・D-04 確認済み）。「MZ の有無」と「PE signature の実在」の互补関係は維持され、規則変更は不要。
- UI 関連項目ではない → 画面確認はルール上不要（実装変更なしのため未実施）。

### 変更内容（実施 2026-11-07・案 A 採用）

- `app/scan.go` `IsPE(path string, head []byte) bool`（Python `src/scan.py:88-105` と対称）: `e_lfanew + 4 <= len(head)` は head slice 判定（従来経路維持）、窓外は `os.Open` → `Seek(io.SeekStart)` → 4 bytes 読みで `PE\x00\x00` 判定。open 失敗・seek 失敗・read 不足（EOF 超え）は false。`e_lfanew <= 0`・`len(head) < 64`・`MZ` 非頭は false のまま（Python 一致）。
- 呼び出し 1 箇所を `IsPE(path, head)` へ更新（`app/scan.go:222`・実施前は `app/scan.go:200`）。
- `app/scan_test.go` にテスト追加（Python `tests/test_scan.py:117-150` の 4 テストを模し、加えて境界 0x40 と EOF 超え）: `makePEStub` + `TestPEWindowParityWithPythonReference`（6 ケース）+ `TestIsPEUsesPathOnlyForWindowOutside`（窓内経路は path 依存なし・窓外 path open・64 バイト未満 head は false・`e_lfanew` 0 は false）。
- 採用した判断: 推奨案 A（記録の推奨案）。「実装開始指示待ち」の状態は本実行（実作業担当としての委譲）で解除。案 B（窓拡大）は正典の参照実装ミラー方針に反するため不採用。

## Verification

- 実測（正典 Python 実行・Go copy 実行）: 上表の 6 fixture（`docs/ai/tmp/assess-20261106-1056/r02_scan_check.py`・`r02_go_check.go`）。分岐 2 件（f1・f5）・一致 4 件。f5 は 61 生成時に e_lfanew フィールド重複を検出、64 に修正し再実測（`r02_debug_f5.py` の解析で確認）。実装は変更していない（変更したのは本記録とスクラッチのみ）。
- `go vet . && go test -count=1 -timeout 300s .`（app・2026-11-06 20:09:02〜20:09:32・全 30 s・go test 内部 25.682 s）: **ok**（既存テスト全通過。`IsPE` 固定テストは現状なし）。
- `python -m pytest tests/ -v`（ルート・2026-11-06 20:09:03〜20:09:18・13.81 s）: **54 passed / 1 failed**。D-04 の 4 テスト（`test_pe_signature_beyond_head_window_is_not_mz_hit` ほか）は通過。失敗 1 件（`test_canonical_build_freshness_rule_exists`）は AGENTS.md の pointer 検査で、R-02 と無関係の既存失敗（照合開始時点で HEAD 側に該当 pointer 条なし・decision ファイルは存在。R-02 の合格条件には影響しない）。
- 照合開始時点の git status: 他項目のタスク記録変更あり（R-01・findings 記録）。R-02 以外には触れていない。

### 実施後の検証（2026-11-07）

- `go vet . && go test -count=1 -timeout 300s .`（app・2026-11-07 23:17〜23:19・go test 内部 25.145 s）: **ok**（全テスト通過。追加テスト `TestPEWindowParityWithPythonReference`・`TestIsPEUsesPathOnlyForWindowOutside` 含む。`gofmt -l` の残差分は既存ファイルの整形で本項目の変更分は差分なし）。
- 合格条件（`e_lfanew >= 64` の実在 PE fixture で `mz_without_pe_header` 非 hit）: 追加テストの 6 ケースで Go 実装と Python 正典実装の帰結が一致。同一 fixture 生成条件（`docs/ai/tmp/assess-20261106-1056/r02_impl_parity.py`）で Python 正典 `scan_file` を実測し、Go テスト（実コード）と比較:

| case | e_lfanew | sig | Go（ScanFile 実測） | Python（正典 scan_file 実測） | 一致 |
|---|---|---|---|---|---|
| beyond-window | 0x80 | 有 | 非 hit / not_evaluated | 非 hit / not_evaluated | yes |
| boundary-64 | 0x40（窓外の最小） | 有 | 非 hit / not_evaluated | 非 hit / not_evaluated | yes |
| within-window | 0x38 | 有 | 非 hit / not_evaluated | 非 hit / not_evaluated | yes |
| 窓外宣言・signature なし | 0x80 | 無 | hit | hit | yes |
| 真の MZ-only | 0 | 無 | hit | hit | yes |
| EOF 超え | 0x20000000 | 無 | hit | hit | yes |

- `python -m pytest tests/ -v`（ルート・2026-11-07 23:20・12.59 s）: **55 passed / 1 failed**。失敗は既存 known-issue の `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 検査・R-02 と無関係・本項目の不合格ではない）。D-04 の 4 テストは通過。Python 側コードは変更していない。
- 試行回数: 1。初回 `go test` でテスト helper（`makePEStub`）の fixture サイズ生成バグ（窓外 case が 64 バイトで生成され signature 書き込みスキップ）を検出、helper を修正して再実行し全通過。実装本体の修正はなし。
- 実機確認（canonical build・served 画面）: 本項目は UI・viewer・download・full image 経路に触れないため適用外（作業前チェックの該当条は full image/viewer/download タスク向け）。scan 判定の証跡は上表の実測。
- R-03（7z magic 最終バイト・`829d08a`）には触れていない（`DetectMagic`・`rules/structural.yar` 変更なし）。

## 優先順位付けに効く要素

- 重要度: 中（scan 判定の正しさ・参照実装対称性の実務基準）。実在 PE の e_lfanew が 64 未満の窓内 PE（従来 DOS header 64 バイト前の offset 例 0x38）でも窓内経路は正しいが、実在 PE の e_lfanew は通常 64 以上（f5 の境界）で分岐が実務側に効く。
- 緊急度: 低（常時稼働で hit 誤検知が続くが、既存 DB の findings の整合は R-17 の別論点）。
- リスク: 低（修正は scan.go の 1 関数 + 呼び出し 1 箇所。UI・DB schema・ネットワークなし）。
- 依存: D-04 決定と `src/scan.py` 正典実装（実装済み・テスト固定済み）。Go 側の実装は単独で完了可能。既存 Go テスト変更不要（新規テスト追加のみ）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
