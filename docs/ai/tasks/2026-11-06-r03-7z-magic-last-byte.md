# R-03: 7z magic の最終バイト誤り（27 38 → 27 1C）で 7z 未検出

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 正典 magic 37 41 7A BC AF 27 1C の fixture で detect magic = 7z（Go・Python 両側）。記録: R-03

## 意図 / 背景

- 2026-11-06 review findings（`docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`）の R-03（判定: 確定・実 fixture 再現実行なし）を、実測で閉じるための調査。
- 指摘: detect magic の 7z signature 照合が最終バイト `38`。正典（外部仕様）の signature は最終バイト `1C`（`37 7A BC AF 27 1C`）。よって実在 7z ファイルは 7z として検出されない。
- 本調査では (1) 正典値の外部仕様確認、(2) fixture での Go・Python 両側の実測、(3) 既存テストの固定有無、(4) YARA 側の 7z 扱いを確認した。

## 調査結果 / 変更内容

### 現在の実装

- Go: `case len(head) >= 6 && hex.EncodeToString(head[:6]) == "377abcaf2738": return "7z"`（`app/scan.go:89-90`）。
- Python: `if head[:6] == b"7z\xbc\xaf\x27\x38": return "7z"`（`src/scan.py:79-80`）。
- 両側は head 先頭 6 bytes を照合。現在の literal は `37 7A BC AF 27 38` で、実 7z signature の最終バイト `1C` と不一致。

### 正典値（外部仕様）の確定と矛盾

- py7zr の .7z format specification（normative・Appendix BNF）の Signature は `b'7z\xBC\xAF\x27\x1C'`（6 bytes = `37 7A BC AF 27 1C`）。Library of Congress FDD（magic numbers `37 7A BC AF 27 1C`・GCK 引用）、chardet の magic（`37 7a bc af 27 1c`）も一致。
- 項目原文・findings R-03 に記録された正典値 `37 41 7A BC AF 27 1C`（7 bytes）は、正典仕様にない `41`（'A'）を 2 byte 目に含み、外部正典と矛盾。最終バイト `1C` の指摘は正典と一致。
- 採用: 外部正典（py7zr normative / GCK / LoC）の `37 7A BC AF 27 1C`（6 bytes）。項目原文の正典列は転記誤りとして区別する。この矛盾のため、現行合格条件（`37 41 7A BC AF 27 1C` の fixture）は正典仕様の fixture ではない。合格条件の文言は `37 7A BC AF 27 1C` への再評価が必要（親の判断）。

#### 正典 2 系統の照合（差し戻し 3・2026-11-06・`41` の有無を確定）

- 系統 1（公式 7z format spec・`7zFormat.txt`）: https://github.com/ip7z/7zip/blob/main/DOC/7zFormat.txt（「7z Format description (18.06)」。LZMA SDK 正典文書）。SignatureHeader 定義の原文: `BYTE kSignature[6] = {'7', 'z', 0xBC, 0xAF, 0x27, 0x1C};`。signature は 6 bytes で `41` を含まない。7・8 byte 目は signature ではなく `ArchiveVersion`（`Major`=0・`Minor`=4）として分離定義。
- 系統 2（実ファイル hex dump・実装）:
  - 実測: 現マシン（7-Zip 26.05）で生成した実 7z `docs/ai/tmp/assess-20261106-1056/real.7z` の先頭 hex（`certutil -encodehex`・`real_hex.txt`）= `37 7a bc af 27 1c 00 04 …`。`41` なし。
  - libarchive `libarchive/archive_read_support_format_7zip.c`（https://github.com/libarchive/libarchive/blob/master/libarchive/archive_read_support_format_7zip.c）: `memcmp(p, _7ZIP_SIGNATURE, 6)` の 6 bytes 照合、`p[5] == 0x1C` の分岐と shift 判定 `case 0x37 / 0x7A / 0xBC / 0xAF / 0x27`。signature = `37 7A BC AF 27 1C`（6 bytes）。
  - py7zr（系統 1 の独立引用として上記と一致）。YARA 公開ルール群は raw 取得 404（直接引用不可）→ 実測 hex と libarchive で代替。
- 結果: 2 系統は一致（食い違いなし）。正典 signature = `37 7A BC AF 27 1C`（6 bytes）。前回の「項目原文の `41` は転記誤り」は訂正不要・2 系統で確定。
- 現行 6-byte 照合の必要変更（fixture 実測・両側）: (b) 最終バイト `38`→`1C`（position 6）のみが必要。(a) `41` の追加は不要（正典に `41` がない）。(c) 比較長 7 bytes への拡張は不要（正典 signature は 6 bytes、7 bytes 目は version `00 04` で値が可変）。
- 実測（正典列 fixture 1 組・成功 fixture 1 組・実ファイル・両側、`r03_check3.py`・`r03_check3.go`）:
  - `sig_canonical_6.bin`（正典 `37 7A BC AF 27 1C`）→ Go `unknown` / Python `unknown`（失敗・差分 position 6 のみ）
  - `sig_current_2738.bin`（成功 fixture・非正典）→ Go `7z` / Python `7z`（現 literal にヒット）
  - `real.7z`（7-Zip 26.05 実ファイル）→ Go `unknown` / Python `unknown`

### 実測（fixture・両側）

- fixture（`docs/ai/tmp/assess-20261106-1056/`）: `canonical_271c.bin` = `37 7A BC AF 27 1C` + version `00 04` + padding（64 bytes）、`current_2738.bin`（対照）= `37 7A BC AF 27 38` + 同。
- Python（実運用関数 `src/scan.py` の `detect_magic` を import して実測、スクラッチスクリプト `r03_check.py`）:
  - canonical fixture → `unknown`（7z 非検出）
  - 対照 `27 38` → `7z`（現 literal にはヒット）
- Go（`app` は package main で import 不可。正典 `app/scan.go:60-93` の verbatim copy をスクラッチ `r03_check.go` で `go run` 実測）:
  - canonical fixture → `unknown`
  - 対照 `27 38` → `7z`
- 結果: 指摘どおり、正典 signature の 7z ファイルは両側で `unknown`。実測で有効性確定。

### 照合（差し戻し・byte position 区別）

- 正典引用（2026-11-06 取得）:
  - py7zr .7z format specification（normative・Signature header 節）: "The first six bytes of a 7-zip file SHALL always contain `b'7z\xbc\xaf\x27\x1c'`"。7 bytes 目は signature の一部ではなく、signature の直後が version（major `00`・minor `04`）。BNF Appendix も `Signature ::= b'7z\xBC\xAF\x27\x1C'`。
  - Wikipedia List of file signatures: 7z = `37 7A BC AF 27 1C`（ISO/IEC 8859-1 表示 `7z¼¯'␜`・offset 0）。
  - 確定: 正典 signature は 6 bytes `37 7A BC AF 27 1C`。項目原文の `41`（'A'）は signature に存在しない（転記誤り）。最終バイト `1C` の指摘は正典と一致。
- 失敗モードの区別（fixture 3 種・byte position・両側一致）:
  - `sig_canonical_6.bin`（`37 7A BC AF 27 1C`）: position 1–5 一致。不一致は position 6（`1C` vs `38`）のみ。`41` は signature 由来の失敗要因ではない。
  - `sig_item7.bin`（項目原文の 7 bytes 列 `37 41 7A BC AF 27 1C`）: `head[:6]` = `37 41 7A BC AF 27`。position 1 一致・position 2–6 の 5 か所が `41` 挿入の shift で不一致（7 byte 目の `1C` は 6 bytes 比較に入らない）。7 bytes 列を signature と解釈した場合の失敗は「`41` 欠落と最終バイト両方」ではなく、insertion shift（position 2–6）。
  - 帰結: 実在 7z（正典 6 bytes）に対しては現行照合の失敗は position 6 最終バイトのみ。「`41` 欠落」失敗は項目原文の 7 bytes 列を前提とした解釈でのみ発生する。
- 現行照合での成功/失敗 fixture 1 組（両側）:
  - 成功 fixture: `37 7A BC AF 27 38`（`sig_current_2738.bin`・正典ではない）→ 両側 `7z`
  - 失敗 fixture（正典先頭 6 bytes）: `37 7A BC AF 27 1C`（`sig_canonical_6.bin`）→ 両側 `unknown`
  - 参考: `sig_item7.bin`（7 bytes 列）→ 両側 `unknown`
- 前回の「`41` は転記誤り」の判断は撤回せず、引用元（py7zr normative・Wikipedia）で確定として強化。合格条件の fixture 基準は `37 7A BC AF 27 1C` への再評価が必要（親の判断）はそのまま。

### 有効性

- 有効（確定）。実在 7z ファイルは Go・Python 両側で 7z として detect magic されない。根拠: 上記実測＋外部正典の照合。加えて、項目原文の正典列の `41` は正典仕様にない（上記）。

### 対応の実現可能性と推奨修正

- 両側 1 バイト literal の修正で閉じる（小規模・対称）。
  - 推奨 Go: `app/scan.go:89` の `"377abcaf2738"` → `"377abcaf271c"`（正典列の転記誤り修正も合格条件の fixture 基準へ反映）。
  - 推奨 Python: `src/scan.py:79` の `b"7z\xbc\xaf\x27\x38"` → `b"7z\xbc\xaf\x27\x1c"`。
  - 代替案: Go を byte 一致 `string(head[:6]) == "7z\xbc\xaf\x27\x1c"` に統一（Python 参照実装と同一形）。既存 hex literal 形式の維持（最小差分）を推奨。
- 既存テストはこの magic を固定していない（`app/scan_test.go:59-74` の `TestDetectMagic` に 7z case なし、`tests/test_scan.py` に 7z case なし）。既存テストの更新は不要。実装時に合格条件として両側へ 7z fixture case の追加を推奨。

### 副作用・影響範囲

- `expectedMagic`（`app/scan.go:23-34`）・`EXPECTED_MAGIC`（`src/scan.py:108-119`）に `7z` key がないため、修正前後で `magic_mismatch` finding の挙動は変わらない（誤検知リスクなし）。影響は `files` テーブルの `magic` 値のみ（`.7z` ファイル: `unknown` → `7z`）。
- `magic == "7z"` を特別扱いするコード・UI は不存在（grep 確認: literal 2 箇所のみ）。
- YARA 側: `rules/structural.yar` に 7z 規則なし（archive 規則は `zip.is_zip` ベース、PDF/MZ ののみ）。`scanTargets`（`app/scan.go:16`）＝ pdf・cbz・zip で、`.7z` は YaraGate（`app/scan.go:56-58`）を通らず YARA が走らない。7z の別扱い・矛盾なし（magic 値以外の差分は発生しない）。
- 記録済み DB 行の `magic` 値は再 scan（skip 解除時）で更新される範囲のみ変化。過去行の遡及更新は R-03 の実装範囲外（parent 判断）。

## Verification

- 実測（2026-11-06・実装変更なし・スクラッチ `docs/ai/tmp/assess-20261106-1056/` の `r03_check.py`・`r03_check.go`・fixture 2 件・即時実行（経過時間計測なし）):
  - Python 実運用 `detect_magic`: canonical（`37 7A BC AF 27 1C`）→ `unknown`、対照（`27 38`）→ `7z`
  - Go verbatim copy（`r03_check.go`）: canonical → `unknown`、対照 → `7z`
- `go vet . && go test -count=1 -timeout 300s .`（app）: pass（`ok hitomi-hub/app`・テスト 25.203s・vet 含め 1 分未満）。
- `python -m pytest tests/ -v`: 54 passed・1 failed・12.03s。failed は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md の pointer 条の内容検査）。R-03 と無関係・本調査でソース無変更・調査時点の既存状態。
- 正典値の実測確認: py7zr .7z format specification（normative BNF 定義 `b'7z\xBC\xAF\x27\x1C'`）を webfetch で確認（2026-11-06）。

### 差し戻し照合（正典 byte 列・byte position 区別）

- 正典引用元（2026-11-06 取得・6 bytes `37 7A BC AF 27 1C` で一致）:
  - py7zr archive_format（normative）Signature 節: "The first six bytes of a 7-zip file SHALL always contain `b'7z\xbc\xaf\x27\x1c'`"。7 bytes 目は signature ではなく archive version（`0x00` `0x04`）。BNF: `Signature ::= b'7z\xBC\xAF\x27\x1C'`。URL: https://py7zr.readthedocs.io/en/latest/archive_format.html
  - Wikipedia List of file signatures（row: `37 7A BC AF 27 1C` / `7z¼¯'␜` / offset 0 / `7z`）。URL: https://en.wikipedia.org/wiki/List_of_file_signatures
  - GCK 表は GCK 本人が 2025-04 に https://filesig.search.org/ へ移管（2026-11-06 確認）。Wikipedia が GCK を参照。いずれも `37 7A BC AF 27 1C`（6 bytes）。
- 確定（訂正）: 正典 signature は 6 bytes で、項目原文の `37 41 7A BC AF 27 1C`（7 bytes）は正典にない `41`（'A'）を含む転記誤り。最終バイト `1C`（7 bytes 目）の指摘は正典と一致。前回の「`41` は転記誤り」判断を引用元で確定として強化（撤回しない）。合格条件の fixture 基準は `37 7A BC AF 27 1C`（6 bytes）へ再評価（親判断）。
- byte position 区別（スクラッチ `r03_check2.py`・`r03_check2.go`・fixture 3 種・両側一致・python elapsed=0.15s / go elapsed=0.00s）:
  - `sig_canonical_6.bin`（正典 6 bytes `37 7A BC AF 27 1C`）→ `unknown`。不一致は position 6（`1c` vs `38`）のみ。→ 真の signature に対する現行 6-byte 照合の失敗要因は最終バイトのみ。
  - `sig_item7.bin`（項目原文の 7 bytes 列）→ `unknown`。`head[:6]` = `37 41 7A BC AF 27` で position 2–6（5 か所）不一致（`41` 挿入による shift）。7 bytes 列を signature と解釈した場合の失敗は「`41` 欠落」と「shift（position 2–6）」であって、7 bytes 目 `1C` は 6-byte 比較に入らない。→ 「`41` 欠落」と「最終バイト `38`」の両方は発生しない。
  - `sig_current_2738.bin`（正典ではない）→ `7z`。diff なし。
- 現行 6-byte 照合の成功/失敗 fixture 1 組（両側）:
  - 失敗 fixture（正典先頭 6 bytes）: `37 7A BC AF 27 1C` → Go `unknown` / Python `unknown`
  - 成功 fixture（現 literal と一致・非正典）: `37 7A BC AF 27 38` → Go `7z` / Python `7z`
- 既存テストは 7z magic を固定していない（`app/scan_test.go:59-74` `TestDetectMagic` に 7z case なし、`tests/test_scan.py` に 7z case なし）。更新不要。
- 差し戻し後の fresh 実測コマンド（ソース無変更・R-03 修正の実装は未実施のため fixture 実測は上記のとおり `unknown` のまま＝合格条件未達＝指摘有効）:
  - `go vet . && go test -count=1 -timeout 300s .`（app）: pass・25.681s（`TestDetectMagic` は 7z case がないため現 state で pass）。
  - `python -m pytest tests/ -v`: 54 passed・1 failed・10.02s。failed は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 条の検査）。R-03 と無関係・本調査でソース無変更。
- 差し戻し 3（正典 2 系統・実ファイル実測・2026-11-06・食い違いなし):
  - 系統 1（公式 spec）: `7zFormat.txt`（IP7Z/7zip 正典・18.06）SignatureHeader 原文 `BYTE kSignature[6] = {'7', 'z', 0xBC, 0xAF, 0x27, 0x1C};`。6 bytes・`41` なし・7・8 byte 目は signature ではなく ArchiveVersion（`0`/`4`）。URL: https://github.com/ip7z/7zip/blob/main/DOC/7zFormat.txt
  - 系統 2（実ファイル hex dump・実装）: 7-Zip 26.05（現マシン）で生成した `docs/ai/tmp/assess-20261106-1056/real.7z` の先頭 hex = `37 7a bc af 27 1c 00 04`（`certutil -encodehex`・`real_hex.txt`）。libarchive `archive_read_support_format_7zip.c`: `memcmp(p, _7ZIP_SIGNATURE, 6)`・`p[5] == 0x1C` 分岐（6 bytes 正典と一致）。YARA 公開ルール群は raw 404（直接引用不可、上記 2 source で代替）。
  - 実測（`r03_check3.py`・`r03_check3.go`・両側・即時実行・経過時間計測なし): `real.7z` → `unknown`（両側）、正典列 fixture `sig_canonical_6.bin` → `unknown`（両側）、成功 fixture `sig_current_2738.bin`（非正典）→ `7z`（両側）。必要変更は (b) position 6 最終バイト `38`→`1C` ののみ。(a) `41` 追加不要（正典に `41` がない）、(c) 比較長 7 bytes 拡張不要（7・8 byte 目は version `00 04` で可変）。
  - fresh 実測コマンド: `go vet . && go test -count=1 -timeout 300s .`（app）→ pass・25.069s。`python -m pytest tests/ -v` → 54 passed・1 failed・9.35s（failed は R-03 と無関係の `test_canonical_build_freshness_rule_exists`）。
- 変更したのは本タスク記録と `docs/ai/tmp/assess-20261106-1056/`（スクラッチ）のみ。`src/`・`app/` の変更なし、既存テストの変更なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
