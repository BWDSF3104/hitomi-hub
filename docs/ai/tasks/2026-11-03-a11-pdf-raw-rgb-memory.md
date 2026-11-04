# A-11: rawRGB が全ページ分保持される

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/pdf.go:47-58`（`rawRGB`）の結果を `pdfPage` に保存し、`app/pdf.go:112` で全 page を slice に溜める。1740x2400 の image 1 枚 = 約 12.5MB。gallery 1300 枚なら約 16GB。対応: page ごとに encode して sequential に書く（zlib 生成後に解放）。合格条件: 大きな gallery（mocked fixture）で peak memory が page 1 分＋header に収まる。推定規模: 中。

### 実装前確定方針（2026-11-04・承認済み streaming 方式）

- `exportPDF` を page ごとの decode→rawRGB→zlib→file 直接書き出し（streaming）へ変更。page ごとの結果をメモリに保持しない。
- xref offset は object 番号ごとの map（メタデータのみ、O(page) の byte ではない）で記録し、file へ sequential に書く。
- `/Pages`（object 2）は `/Count` と `/Kids` が全 page 数を決めてから書けるため、物理的には page objects 之後（末尾）に書く。PDF は object の物理順が非連続でも xref offset が正しければ有効（strict parser は xref 経由で object を読む）。byte 形式（`N 0 obj`・`stream`/`endstream`・xref 10桁 offset 行・trailer）は現行と同一。
- decodable page 0 件の場合: 既存動作（ファイル作成前 error）を維持するため、最初の decodable page が確定した時点で file を lazy create する。
- A-8 済みの page 辞書構文（閉じ `>>`）は byte 単位で維持。skipped セマンティクス（`hit`/`scan_error`/`missing`/`undecodable`/`entry_count_exceeded`・MaxEntries 判定）と result の key は不変。
- 合格条件の gate（テストで固定）: mocked fixture（1300 page・300x400 渐变 PNG・page rawRGB = 360,000 B）で `runtime.ReadMemStats` の peak HeapAlloc delta（export 中に goroutine で 1ms 間隔サンプリング・毎 tick `runtime.GC()` 完了後 read）が、page 1 分の working set 理論上界（decoder image 4/3P + rawRGB P + zlib 結果上限 P + zlib writer internal 80KB）+ header/metadata margin（xref offset map・kids・entries）1MB + GC noise margin 8MB（sweep 前 garbage・live 分散による span 保持分。sampling goroutine の MemStats 割当は loop 外へ移出してノイズ最小化）に収まること。全 page 保持（旧実装）なら 468MB で gate 不合格。加えて export 後の live heap delta（post-GC）が header/metadata（1MB）以内であること、xref offset が object 番号順（1..3N+2）で file 実バイト上の `N 0 obj` を指すこと（strict parser 回帰ガード）をテストで検証。
  - 当初 gate（理論上界＋1MB のみ・SetGCPercent(20)）は実測で超過（garbage slack と span 保持が理論 live 上界より約 1.3〜4.5MB 大きい）。10 run 実測（peakDelta 4.4〜6.2MB・max top 6.7MB、liveDelta 33〜77KB）に基づき noise margin 8MB を固定。gate = 10,719,104 B。

### 変更内容（実施 2026-11-04）

- `app/pdf.go`: `exportPDF` を page ごとの decode→rawRGB→zlib→file 直接 sequential 書き出しへ変更。`pdfPage` 構造体削除。`rawRGB`/`zlibBytes` は page 間で buffer 再利用（append 再割当て・page ごとの copy 排除）。`/Pages`（object 2）は `/Kids`・`/Count` 確定のため物理的に末尾へ（xref は object 番号順 offset を map で記録）。byte 形式と A-8 済み page 辞書 `>>` 構文は byte 単位維持。skipped セマンティクス・result key 不変。decodable 0 件の error は file 作成前（lazy create）で既存動作維持。
- `app/export_test.go`: `TestExportPDFStreamedPeakMemoryFitsSinglePageWorkingSet` 追加（上記 gate 固定＋ liveDelta ≤1MB ＋ xref offset 実バイト照合）。既存 PDF 回帰テスト（`TestExportPDFWritesPagesAndSkipsUndecodable`・`TestExportPDFPageObjectDictCountsAreBalanced`）通過。

## Verification

- `app` `go vet .` → clean（gofmt 済み）
- `app` `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 17.270s`（全テスト通過。A-11 gate テスト単独 11.8s）
- A-11 gate テスト実測（最終 run）: peakDelta=3,116,344 B ≤ gate 10,719,104 B、liveDelta=32,568 B ≤ 1,048,576 B、pages=1300、xref offset 照合通過。10 run 安定性確認: peakDelta 4.4〜6.2MB（max top 6,770,136 B）で全 PASS
- canonical exe 再 build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`・2.08s）→ ルート `python -m pytest tests/ -v` → 40 passed（8.97s、`test_canonical_build_freshness.py` 含む）。UI 未変更のため server 実機確認は rules 上不要
- 実機（live gallery）での large gallery export 実測は未実施（合格条件の測定は mocked fixture 基準の定め通り。実測根拠は gate テストの実測値）

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end で実施）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（該当なし。A-11 は個別タスク記録のみ）
