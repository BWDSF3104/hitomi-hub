# scan の Python 参照実装と Go の二重管理

**実行固有ID**: assess-20261109-1320

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

項目 #18（保守性・方針）。原文の「parity 修正(B-06/B-07/B-10)が繰り返し発生」は記録からの実件数として検証済み（下記実数え上げ）。参照実装の役割限定案と Go テスト一本化案の正典適合を確認し、正典ルール変更を要する側は承認案件として分離する。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `src/scan.py`（554 行）= Python 参照実装。**本番経路ではない**: 本番は正典 build の単体 exe（`hub.exe`・Go）。正典 build command（`go build -o ...\output\hitomi-hub\hub.exe .`）と `tests/test_canonical_build_freshness.py` が Go 側を正典実装として照合している。AGENTS.md Development の「スクリプトは `python src/scan.py` で実行」は参照スクリプトの実行入口であり、「単体静的バイナリ化（Go）が完成形。Go toolchain は dev 依存であり実行依存ではない」＝ランタイムに Python 依存なし（Code Standards の「stdlib のみ・移植性」も参照実装の単体実行性を保つ趣旨）。
- Go 側 scan 系: `app/scan.go`（249 行）＋ `app/vt.go`（353 行）＋ `app/main.go` `processFileWithYara`/`scanSkipStatus` ＋ export/pdf/yara。Python 側は同ロジックを単一ファイルでミラー。
- テスト二重: `tests/test_scan.py` 49 件（896 行）↔ `app/scan_test.go` 37 件（1104 行）。**現ツリーに同名対応のミラー固定 17 組**（例: `test_zip_dotdot_is_path_segment_only`↔`TestZipDotDotIsPathSegmentOnly`、`test_vt_empty_results_with_stats_total_is_not_evaluated`↔`TestVTEmptyResultsWithStatsTotalIsNotEvaluated`、YARA error 表現 6 組（B-07 両側固定）、`test_findings_unique_index_key_includes_path`↔`TestNewDBFindingsUniqueIndexKeyIncludesPath`（D-03）、`test_rescan_content_change_marks_stale_findings_resolved`↔同名 Go、`test_add_finding_ignores_duplicates`↔`TestAddFindingIsIdempotentForSameFinding`）。明示的 cross-parity テスト `TestPEWindowParityWithPythonReference`・共有正典 data 照合 `TestDataFilesAtRoot` が Go 側に存在。

### 実数え上げ（parity 修正の実件数）

正典 policy（`docs/ai/tasks/2026-11-01-scan-layer-move.md` parity 方針・2026-11-01）確立以降、Python↔Go の cross-implementation 整理事件は 7 日（2026-11-01〜11-07）で 5 件:

1. `8288151`（2026-11-02・review 項目14）: Python を schema/Go 基準へ整理（INSERT OR IGNORE・TTL/negative cache・`last_analysis_stats`・`engine_name`/`result`・YARA error 分類・トランザクション。`src/scan.py` 224 行変更）
2. B-06（`4f14d00`・11-03・Python 側テスト固定）→ 残件 Go 側統一（`7369066`・11-04）＝**原記録と反対側で修正発生**
3. B-07（`bfdc83e`・11-03・Go 側）→ 残件① Python 側を Go 正典表現へ（`35ece41`・11-04）＝**反対側で修正発生**（Python 側テスト 6 件改名・追加）
4. B-08（`865c325`・11-03）: `..` 判定を Go/Python/YARA の基準で揃える（両側＋規則変更）
5. D-04（`38fb12b`・11-03・Python `is_pe`）→ Go 未修正の非対称が残ったまま → R-02（`33d2da8`・11-07）Go を Python 基準へ対称化＝**反対側で修正発生**

B-10（`a303a4e`・11-03、残件 `6a18e14`・11-04）は Python 単側（Go の fetch は status code 返却・mapping 一致と記録済み）。修正発生ではなく参照実装単体の保守。原文の「B-06/B-07/B-10 の繰り返し」は上記 1〜5 と整合（3 件が両側分割修正）。

未修正 drift（記録済み・実測済み・本項目の現存コスト）: skip 機構 Go のみ/A-12（N-01・2026-11-08 実測）、scan meta 書き込み（Go 常時／Python なし・B-06 残件記録で差異固定）、`DEFAULT_LIMITS` 二重既定表（decision 2026-11-09 承認・実装待ち・`tests/test_scan.py:417` 更新待ち）。

### 正典上 Python 参照実装の現在の役割

- Code Scope（AGENTS.md）: 「実装済み … scan（Go・Python 参照実装）」＝参照実装は納品物として列挙。
- decision 2026-11-09（最新正典）: 参照実装の自前既定表廃止時に「参照実装の単体実行性は保つ」と明示し、`tests/test_scan.py:417` の更新を implementation list に残す。＝**正典上、参照実装とそのテストは継続する前提**（本項目の最新正典として採用。正典同士の状態記述矛盾は確認されず、contradiction rule の適用事例なし）。
- 正典検証条: Python 変更後 `python -m pytest tests/ -v`。`tests/` には scan 以外の Python テスト（freshness 照合・UI harness ラッパー 5 件）があり、scan テスト一本化しても pytest コマンド自体は残る。
- **parity の正典方向が未確定**: `8288151`（Python→Go 基準）・B-07 残件①（Python→Go 正典表現）と R-02（Go→Python 基準）で方向がflip しており、これ自体が二重管理コストの発生源。

### 有効性

**有効（実測・記録実測済み）**。二重管理の保守コストは事実（7 日で 5 事件・両側分割 3 件・未修正 drift 3 件・現ツリーミラー 17 組）。ただし現ツリーは健全（下記検証両 suite 合格・失败 0）で、問題は失败ではなく構造費用と drift 許容の未定義。

### 対応の実現可能性と推奨修正方法

- **推奨（案A）「仕様の例示」限定＋正典方向の確定**: Go を実装正典と宣言し、Python は核心語彙（status・findings 文字列・unique key・TTL 意味）の例示として单向同期（Go 変更→Python 反映）。ミラーテストは既存 17 組を「核心語彙照合」として維持し、新機構では増やさない（A-12 skip・scan meta 行・exception path 等の非語彙差异は「例示差異」と decision で明記し parity 修正対象から外す）。実装は decision 記録＋テスト分類のみ（本番コード変更なし）。**案A も parity 義務と方向の definition 変更なので承認案件**（判断待ち減方針の「正典rule変更」領域）。
- **案B（Go 側テストへ一本化）**は `src/scan.py`（554 行）と `tests/test_scan.py`（896 行）の削除＋Code Scope（AGENTS.md・write-protected）と Development 条の変更を要し、最新 decision 2026-11-09（参照実装単体実行性保持）を supersede する。明示的承認なしには進められない。**承認案件として分離**。

### 副作用・影響範囲・代替案

- cross-check ネット: 実証済み価値あり（R-02 は Python 基準との対称化として発見、B-08 は 3 実装整理事件）。案A は核心語彙のネットを残し、案B は失う。
- Python↔Go DB 互換は共有 `schema.sql`（正典）で担保済み（D-03・test 両側固定）。案B だと Go DB と参照実装の互換検証口が消える（Python テスト削除）。実 DB・完成フォルダは非変更。
- N-01 との依存: skip 機構は Go 固有で Python に mirror 不可。drift 許容を「例示差異」と確定すると、N-01 の修正方法（Go 側固定テスト新設）は影響を受けないが、Python 側 mirror は不要と明記できる。
- 代替案（最小）: 方向確定のみ（テスト・ファイル変更なし）。両側整理義務は残るため現行 cost と同じ、ただし flip による往復修正を止める。

推定規模: decision 記録・小（docs のみ）。テスト分類（案A の核心語彙固定）: 小（既存 17 組の明示注記）。案B 実施（承認後）: 中（削除 1450 行＋正典条 2 変更）。

優先順位付け要素: 重要度=中（保守性。ユーザー影響なし・cross-check 価値は実証済み）、緊急度=低〜中（現在失败 0。drift 3 件は別項目で pending）、リスク=案B 中（正典変更＋検証ネット喪失）、依存=decision 2026-11-09 implementation list（`tests/test_scan.py:417`）・N-01（pending）・AGENTS.md Code Scope（write-protected）。

UI 確認: UI 項目でない（画面該当なし）→ 省略。キャプチャ保存なし。

## Verification

- ルート `python -m pytest tests/ -v`（2026-11-09・assess-20261109-1320）: **60 passed / 0 failed・25.66 s**（`tests/test_scan.py` 49 件・`test_canonical_build_freshness.py` 含む合格）。2026-11-08 の N-01 実測で 1 failed で記録された `test_canonical_build_freshness_rule_exists`（KI-20261106-04）は archive 済みの該当 issue と整合し、今回合格。
- `app` で `go vet . && go test -count=1 -timeout 300s .`（2026-11-09・同ID）: vet clean・**ok 47.503 s**（Go 変更案があるため実施。本項目はソース無変更）。
- 実 DB・完成フォルダ・config 非変更。使い捨てスクリプトなし（静的確認＋suite 実行のみ）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
