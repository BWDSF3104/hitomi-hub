# 2026-11-09 — scan の parity 方向確定（実装正典＝Go・Python 参照実装＝仕様の例示）

**Status**: 方向定義として記録（推奨案A・ユーザー同意 2026-11-09）。本項目での変更は decision と個別タスク記録のみ。テスト実ファイル・`src/scan.py`・`app/scan.go` は非変更。テスト分類は本 decision の記載として行う（ファイルの移動・改名・削除なし）。正典 suite の合格条件は不変。
**タスク記録**: `docs/ai/tasks/2026-11-09-n24-python-go-scan-parity-double-maintenance.md`

## 決定（正典）

1. **実装正典は Go**。本番経路は正典 build の単体 exe（`hub.exe`）。scan 系正典実装は `app/scan.go`（249 行）・`app/vt.go`（353 行）・`app/main.go` の `processFileWithYara`/`scanSkipStatus`。`tests/test_canonical_build_freshness.py` が Go 側を正典実装として照合している現状を正典方向として確定する。
2. **Python 参照実装（`src/scan.py`・554 行）は「仕様の例示」に限定**。本番経路でも基準源でもなく、同期義務は单向（Go 変更 → Python 反映）。逆方向の修正（Go を Python 基準へ整える）は行わない。
3. **核心語彙（例示義務の対象）**: `scan_status` 語彙（`pass`/`hit`/`not_evaluated`/`external_removal`/`scan_error`・decision 2026-11-01 固定）、findings 文字列、findings unique index key（D-03）、TTL 意味、共有 `schema.sql`（正典）。
4. **drift の扱い**: 非語彙差異は「例示差異」として記録し、parity 修正対象から外す。該当: skip 機構（Go のみ・A-12）、scan meta 書き込み（Go 常時／Python なし）、`DEFAULT_LIMITS` 二重既定表（廃止は decision 2026-11-09 settings-centralization 確定）。drift 3 件自体は本項目で修正しない（別項目 pending）。N-01 の修正方法（Go 側固定テスト新設）は影響を受けず、Python 側 mirror は不要。
5. **テスト分類（記載）**: 既存のミラー固定テスト組（記録上の分類 17 組）は「核心語彙照合テスト」として維持し、新機構では増やさない。実測補足: 名前完全一致のミラーは 10 組（`tests/test_scan.py` 49 件 ↔ `app/scan_test.go` 37 件）、17 組は cross-name pair を含む分類数で、D-03 等の cross-name pair は `app/main_test.go` に所在（`TestNewDBFindingsUniqueIndexKeyIncludesPath`・`TestAddFindingIsIdempotentForSameFinding`・`TestRescanContentChangeMarksStaleFindingsResolved`）。明示 cross-parity テスト `TestPEWindowParityWithPythonReference` と共有正典 data 照合 `TestDataFilesAtRoot` は核心語彙照合として維持。
6. **正典検証条は不変**: Python 変更後 `python -m pytest tests/ -v`、Go 変更後 `app` で `go vet . && go test -count=1 -timeout 300s .`。合格条件に未確認点を残さない基準（evidence-based）もそのまま。

## Reason

- 二重管理コストは実測済み（7 日＝2026-11-01〜11-07 で cross-implementation 整理事件 5 件、うち 3 件が両側分割修正、未修正 drift 3 件）。反復の発生源は parity 方向の flip（`8288151`・B-07 残件①＝Python→Go 基準、R-02＝Go→Python 基準）であり、单向確定で往復修正を止める。
- cross-check ネットの実証済み価値（R-02 の対称化発見・B-08 の 3 実装整理事件）を、核心語彙照合として維持する。
- Python↔Go DB 互換は共有 `schema.sql`（正典）で担保済み（D-03・両側テスト固定）。現ツリーは健全（正典 suite 全緑・失败 0）。

## AGENTS.md 正典条との両立（AGENTS.md は write-protected・非変更）

- Code Scope「scan（Go・Python 参照実装）」: 参照実装は納品物として列挙されたまま。本決定はそれを削除せず、役割を「仕様の例示」として定義する。
- Development 条「スクリプトは `python src/scan.py` で実行」: 参照実装スクリプトの実行入口であり、本番経路の定義ではない。「単体静的バイナリ化（Go）が完成形の目標」＝実装正典 Go と整合。
- Python 条（stdlib のみ・JSON・単体実行性）は参照実装の単体実行性を保つ条件として維持（decision 2026-11-09 settings-centralization と一致）。

## 案B（Go 側テストへ一本化）

不採用。承認案件として分離維持。要件: `src/scan.py`（554 行）と `tests/test_scan.py`（896 行）の削除、Code Scope・Development 条（write-protected）の変更、decision 2026-11-09 settings-centralization（参照実装単体実行性保持）の supersede、cross-check ネットと Python↔Go DB 互換検証口の喪失。明示的な承認なしには進めない。本 decision は同 decision を supersede しない。

## 関連決定

- `docs/ai/decisions/2026-11-01-hitomi-hub-scan-layer-portable.md`（status 語彙固定・完成形 Go）
- `docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md`（unique key・共有 schema）
- `docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md`（`DEFAULT_LIMITS` 廃止・参照実装単体実行性保持）
- `docs/ai/tasks/2026-11-01-scan-layer-move.md`（2026-11-01 parity 方針）: 当時は方向未確定。本 decision が方向を確定する
