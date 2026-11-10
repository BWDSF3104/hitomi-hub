# N-02: ScanPDFKeys が先頭 2MB しか見ていない

**実行固有ID**: assess-20261108-1332

**Status**: 進行中
**Date**: 2026-11-08
**Next Action**: 調査完了・実施未。窓分割（head+tail・合計 MaxScanBytes 不変）は本体単独実施可。pass→hit 波及と xref 解凍案は承認サブ分

## 意図 / 背景

- 原文: `ScanPDFKeys`（Go）・`scan_pdf`（Python 参照実装）は `MaxScanBytes`（既定 2,097,152 B）ぶん先頭だけ読み、`/OpenAction`・`/AA` 等の危険 key を判定する。Catalog は実 PDF でファイル後方に置かれ得るため、2MB 超の PDF では key を見逃す。先頭と末尾の両方を読むのが現実的、という指摘。
- 本調査は読み取り＋スクラッチ fixture 実測のみ。ソースコードは変更していない。実 gallery・実完成フォルダ・実 DB は不使用（fixture は `docs/ai/tmp/assess-20261108-1332/fixtures/`）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- Go: `app/scan.go:187-198` `ScanPDFKeys` は `io.LimitReader(f, l.MaxScanBytes)` で先頭のみ読み、`pdfKeyFindings`（`app/scan.go:201-209`・`bytes.Contains`・A-13 済み）で 7 key を判定。
- Python: `src/scan.py:150-157` `scan_pdf` は `fh.read(limits["max_scan_bytes"])`（先頭のみ）で `PDF_KEYS` を判定。既定 `max_scan_bytes` = 2,097,152（`src/scan.py:26`・`app/main.go:95`・`config.example.json:34`）。
- 判定根拠: `ScanFile`/`scan_file` の findings → status（hit / pass）。key はファイル全体のどこにでも raw substring で照合されており、照合範囲が先頭窓に限定されている点が原文の指摘点。
- YARA は全文照合（`rules/structural.yar:3-20` `PDF_Structural_Risk` は magic 条件＋ strings anywhere、`--skip-larger` 67,108,864 B）。ただし gate は `yr` 設定時だけ（`YaraGate`）。`config.example.json:4` の `yr` は null（既定は YARA 不参加）。

### 有効性

**有効**（実測で閉じた。原文の機構どおり、2MB 超の key を構造化検査単独で見逃す）。

実測数値（fixture・正典関数呼び出し）:

| fixture | 構成 | size | 現行で走査 bytes | 現行 findings | scan_file status | full read | tail 64KB 窓 |
|---|---|---|---|---|---|---|---|
| f1 | key が先頭窓内（対照） | 63 B | 63 | hit（`pdf_object:/OpenAction`） | hit | hit | — |
| f2 | Catalog（`/OpenAction`）が offset 2,300,035（>2MB）・classic xref/trailer | 2,300,170 B | 2,097,152 | 0 findings（miss） | pass | hit | hit |
| f3 | Catalog が compressed `/ObjStm`（>2MB） | 2,200,368 B | 2,097,152 | 0 findings（miss） | pass | miss（raw） | miss（raw）。`/ObjStm` マーカーは窓内に見える。zlib 解凍時 hit |
| f4 | `/Root` 参照は xref/trailer 内、catalog object は先頭窓と末尾窓の両外（offset 2,100,009） | 2,700,162 B | 2,097,152 | 0 findings（miss） | pass | hit | substring miss（trailer のみ）。xref 解決＋ object 窓 2,048 B 読みで hit（`/AA`） |

- YARA 実測（tools/yr.exe 存在・`run_yara` 呼出）: f1 hit・f2 hit（全文照合で 2MB 超 key を `PDF_Structural_Risk` で捕える）・f3 []（圧縮内容は見えない）。→ 緩和の範囲: YARA 設定済みかつファイル ≤ `yr_skip_larger` かつ key が raw（非圧縮）なら検出。既定 config（`yr` null）では構造検査単独が detector で、miss がユーザーに見える。

### 対応の実現可能性と適切な修正方法（推奨）

- 実現可能。既存 gate を壊さない形は「窓分割」: 現行 `MaxScanBytes` を head+tail に分割（合計 2,097,152 のまま）。例: head 2,031,616 + tail 65,536。`MaxScanBytes` に上乗せする案（合計 2,162,688）は max bytes gate の意味変更 → 承認案件として分離。
- 推奨（案 A）: head+tail 窓（分割）・key 照合は `bytes.Contains` を窓ごとに（A-13 の 0 alloc 意味を維持）＋ xref/trailer 基準の `/Root` 解決（`startxref` → xref entry → object 窓を上限付き（例 2,048 B・object dictionary 限定）で読む）。f4 が示すように、substring だけの末尾窓は object が窓外だと miss するため、xref 解決が正確性の本体。
- 区別可能（実測で確定）: xref/trailer に Catalog 参照がある場合（f2・f4: 解決可。f4 は raw 末尾窓では miss）と、Catalog が object stream（`/ObjStm`）へ圧縮された場合（f3: raw 窓は miss、`/ObjStm` 検出と解凍（Python `zlib`・Go `compress/flate`・stdlib のみ）が必要なことを区別できる）。圧縮 key の完全検知は困難（前批 B-4 対応「object dictionary 範囲限定、または known-issues 正典化＋YARA 役割分担」）。検出手段としての解凍追加は判定意味の変更なので、既定では known-issues 限界として記録し、解凍案は承認要のサブ案として分離する。
- 誤検知率（実測・判断保留なしで閉じた）: 64KB 窓の乱数二进制で `/AA` 0.45%（n=2000・seed 固定）／`/OpenAction` 0.0000%。B-4 の「2MB 窓で各 10% 強」と窓サイズの縮小で整合（期待値は窓に比例）。object dictionary 限定（案 A）はこれをさらに下げる。

### 副作用・影響範囲・代替案

- 判定結果の意味の変更: 同じファイル（sha 不変）で 2MB 超 key が pass → hit になる（ユーザーに見える。hit は export skip を経由して完成物にも波及）。→ 検出範囲拡大そのものは正典 Code Scope（構造検査・hash 評判のみ）内に収まるが、「同じファイルで判定が変わる」ため意味変更の承認案件。実装案と分離して提示。
- 不可逆な過去記録の書き換えは不要（findings は `INSERT OR IGNORE`・D-03 非破壊・status は upsert の既存挙動）。ただし A-12 skip（`scan_meta` size+mtime・R-06）が不変ファイルの再評価を飛ばすため、過去ファイルへの適用は再 scan・full scan の運用条件次第。
- A-11 gate（pdf.go export の peak memory）は走査 key 読みと別経路で非触。scan メモリは窓サイズぶん（tail ≤ 65,536 B）で bound。
- YARA 規則・status 語彙・findings 語彙（`pdf_object:<key>`・engine `structural`）・Python/Go parity・正典 build は変更なし。
- 既存テストの照合: 危険キー検出の意味は「key が窓内にあれば hit」までしか固定していない（Go `TestStatusSemantics`・`TestScanPDFKeysDoesNotCopyDataPerKey`（2MB 窓・0 alloc・key hit 判定）、Python `test_pdf_viewer_action_hit`、YARA 固定テスト）。先頭のみ・窓外 miss を固定するテストはなし → head+tail 化で既存 assertion は壊れない（2MB fixture は hit なし・0 alloc は bytes.Contains 維持で成立）。新意味（後方 key hit）は現テストで固定されていないため、実装時に fixture テスト追加が必要。
- 代替案 B: 末尾窓のみ（key 照合）で xref 解決なし。f4 型（object が窓外）を捕えず、正しさの半分。非推奨。代替案 C: `MaxScanBytes` 上乗せ（gate 変更）は承認要。

## Verification

- Go 実測（現行関数呼出・overlay 注入テスト `docs/ai/tmp/assess-20261108-1332/n02_probe_test.go`・`app/` へファイル書き込みなし・既存テスト非触）: `go vet .` clean → `go test -count=1 -overlay docs\ai\tmp\assess-20261108-1332\n02_overlay.json -run "TestN02ScanPDFKeysHeadOnlyMissesTailCatalog" -v .` → PASS 1.280s（test 0.05s）。数値: f2 findings_current=[]（bytes_scanned=2,097,152）・full_read_has_key=true・tail64k hit（`pdf_object:/OpenAction`）。f3 findings_current=[]・raw miss・tail64k に `/ObjStm` 見える。
- 全 suite（既存テスト非触）: `go vet . && go test -count=1 -timeout 300s .`（app）→ ok 25.913s。
- Python 実測: `python docs\ai\tmp\assess-20261108-1332\n02_measure.py`（正典 `src/scan.py` 関数呼出・実測表上の数値・elapsed 54.61s（FP sampling 2000 ブロックを含む））。
- YARA 実測: `python docs\ai\tmp\assess-20261108-1332\n02_yara_check.py` → f1 hit・f2 hit・f3 []・elapsed 0.55s（tools/yr.exe 存在。既定 config `yr` null）。
- `python -m pytest tests/ -v` → 59 passed / 1 failed（12.89s）。failed は `test_canonical_build_freshness_rule_exists`（KI-20261106-04 として報告済みの既存 failure・本項目非接触）。
- 画面確認: UI 項目でないため該当なし（未確認・Drive 保存なし）。
- 読了正典: `docs/ai/decisions/2026-11-01-yara-role-division-and-gate.md`（役割分担・findings 語彙）、`docs/ai/tasks/2026-11-02-external-code-review-findings.md` B-4（誤検知・圧縮 key false negative・対応方針）、`docs/ai/tasks/2026-11-03-a13-scan-pdf-keys-string-alloc.md`（0 alloc 意味）、`docs/ai/rules/confirmation-test-rule.md`（UI 確認基準・本項目該当なし）。矛盾なし。

推定規模: 小〜中（scan.go＋src/scan.py 窓分割と xref 解決数十行、fixture テスト 4 前後、Go/Python 対称）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
