# R-17: 同一 path の内容が変わっても旧 sha256 の finding が残る

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 完了（B1 実装済み・pytest 59 passed・go test ok・probe 再実行で内容変化後 join view 0 行/stale マーキング/同一 content 不変を確認。コミット待ち）

## 意図 / 背景

- `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` の R-17（判定: 確定）に対する読み取り調査と実測確認の記録。ソースコードは変更しない（変更したのは本記録と `docs/ai/tmp/assess-20261106-1056/` のみ）。
- 指摘: `addFinding` の `INSERT OR IGNORE`（`app/main.go:572-576`）は unique key 衝突時に更新せず、内容（sha256）が変わっても findings が消えない。item 単位の更新・解決経路がない。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `addFinding` は `INSERT OR IGNORE INTO findings (path, sha256, rule, detail, found_at)`（`app/main.go:572-576`）。unique key は `(path, sha256, rule, detail)`（`schema.sql:86`、Go 側 `findingsUniqueIndexSQL` `app/main.go:467`・`ensureFindingsUniqueIndex` 475-496 定義置換のみ）。指摘原文の key 表現（gallery hash + item kind + item path）は `detail` 省略の近似（sha256=内容 hash・rule=kind・path=一致）。
- findings への本番読み取り口は**ない**: `FROM findings` の本番引用は app 内 0 件（テストのみ）。Python 参照実装も `add_finding` の `INSERT OR IGNORE` のみ（`src/scan.py:325-329`）。
- findings への DELETE・UPDATE 経路は本番コードに**一切ない**。指摘原文の `deleteFindingsForGallery` はリポジトリ内に存在しない（grep 0 件）。かつて `migrateSchema` にあった重複行 DELETE は D-03 で除去済み（`docs/ai/tasks/2026-11-03-d03-findings-unique-path.md` line 18）。
- 表の可変性の非対称: `files` は current state の upsert（`app/main.go:578-586`）、`scan_cache` は `ON CONFLICT DO UPDATE` で上書き可能（`app/main.go:525,537`・`src/scan.py:316-322`）。findings だけが insert-only（D-03 正典）。

### 有効性

- **一部有効**。
  - 有効（確定・実測で閉じた）: 同一 path で内容（sha256）が変わると旧 sha256 の行が残存し、item 単位の更新・解決がない。現行判定（files の pass）と findings 行が食い違うことを実測で示した（Verification 参照）。
  - 原文の誤り 1: findings の rule 語彙は `structural`/`yara`/`virustotal` のみ（`usage-memo.md:253`、decision `2026-11-01-yara-role-division-and-gate.md`）。`no_full`・`error` は findings rule ではない（`no_full` は viewer の in-memory negative cache の x-cache 値・`app/viewer.go:895`）。実測で観測 rule = `structural` のみ。
  - 原文の誤り 2: 「viewer での再確認で stale finding が残る」は findings 表には当てはまらない。viewer 側に addFinding 呼出し口がなく、viewer の stale 状態（no_full negative cache）は TTL 期限切れ・force/manual retry で解決済み（テスト固定 `app/viewer_test.go:629-653`、decision `2026-11-04-remaining-residual-approval-split.md`）。DB の findings 関与なし。
  - 原文の誤り 3: 削除が「gallery 単位のみ」という記述は、削除経路そのものが存在しないため誤り（上記）。

### UI 経路照合

- `listFindings`（grep 0 件）、`/api/findings`（route 現-list に無し・`app/viewer.go:133-152`）、search card 応答（`searchResult`・`app/viewer.go:656-663`）に findings/scan 字段なし、UI html に `finding` 文字列なし。badge は read_state/viewed/saved（decision `2026-11-05-search-result-state-badges-approved.md`）で findings 非表示。
- したがって stale finding のユーザーに見える意味は直接 SQL 照会（`usage-memo.md:214-218` の「結果の読み方」）のみ。UI 項目ではないため画面確認・Drive スクリーンショット該当なし（UI 読み取り口なしを grep で確認）。

### 正典記録照合（副作用）

- D-03（`docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md`）: findings は path ごとに意味を持つ記録、迁移は非破壊（既存行の削除・改変をやらない・暗黙 dedup へ戻さない）。→ 「記録として残す」案は D-03 と整合。「旧行を削除する」案は D-03 と矛盾（正典決定変更なしには実施不可）。
- B-01（verify missing batch）: 追加指示の参照名で docs/ai を辿ったが、verify missing batch の正典記録は**見つからない**（grep 0 件）。正典名 B-01 は `docs/ai/tasks/2026-11-03-b01-enter-implicit-submit.md`・decision `2026-11-03-b01-nav-enter-noop.md`（UI Enter 挙動・本件と無関係）。verifyMissing（`app/main.go:618-643`）の意味は R-04 が正典（`files` は正典上 history 表ではなく current state、flag は走査キー）。矛盾: 参照名と内容が不一致。採用文書: R-04。
- A-04（missing flag reset）: 正典名 A-04 は `docs/ai/tasks/2026-11-03-a04-plain-term-lang.md`（plain term lang・内容不一致）。missing flag reset として内容一致の正典は R-04。採用文書: R-04。
- 歴史記録の意味との矛盾照合: findings = insert-only 歴史（D-03 正典）であり、item 単位の「更新」（旧行rewrite）は矛盾。非破壊の列追加＋マーキング（`ensureScanCacheSizeMtime`・read_state `viewed`・`saved_at` パターン）は承認済みパターンで矛盾しない。

### 修正案

- 案 A（記録として残す・コード変更なし): findings は (path, sha256) ごとの歴史として維持。「現行判定」は `files.scan_status` と join view（`findings.sha256 = files.sha256`）で定義できる（現コードで導出可・実測で current view = 0 行・残存 1 行を示した）。usage-memo/記録の明確化のみ。D-03 と矛盾せず、承認不要。
- 案 B（item 単位で解決）:
  - B1（非破壊）: findings へ `resolved_at` 等の列追加（列追加のみ・D-03 非破壊規約・schemaVersion 7→8）し、scan tx 内で同 path の旧 sha256 行を再判定時にマーキング。D-03 非破壊規約と整合、合格条件（path 基準で旧 finding を更新/無効化）を満たす。
  - B2（旧行 DELETE）: D-03 の既存行削除禁止と矛盾 → 正典決定の変更が必要で非推奨。
- ユーザーに見える意味変更の分離: UI 表示はないが、案 B（B1）は findings 照会結果の意味を変える（直接 SQL で user-visible）＋ schema 変更を要る → 承認が必要として分離。案 A は意味を変えない。
- **推奨**: 案 B1（非破壊マーキング）。D-03 の unique key と非破壊規約を保ったまま合格条件を満たす最も適切な案。ユーザーが「歴史として残す」を選んだ場合は案 A（実装不要・記録明確化のみ）で閉じ可。

### 実装方針（確定・B1 承認済み・実装前確定）

- 採用: 案 B1（ユーザー承認済み・委譲指示）。`resolved_at` 列追加のみ・既存データ削除・不可逆 migration なし（D-03 非破壊迁移規約）。残存行の扱い = 歴史として残す＋再判定時に stale マーキング（`resolved_at` に時刻設定）。unique key（path, sha256, rule, detail）は維持（D-03）。
- 矛盾記録: 委譲文は decision `2026-11-06-review-findings-approval-decisions.md` の R-17 記載を正典として採用するよう指示したが、実際の同文書に R-17 の項目記載がない（記載は R-11・R-15・R-20・R-05・R-08・R-13）。B1 承認は委譲文（ユーザー承認・追加指示）を正典として採用した。矛盾の旨をここに記録。
- schemaVersion: 記録の推定は 7→8 だが実測の現行値は 9（R-06 = 8・R-15 = 9）。本項目は 9→10。
- マーキング規則（scan tx・再判定時のみ）: `UPDATE findings SET resolved_at = ? WHERE path = ? AND sha256 <> ? AND resolved_at IS NULL`（Go `resolveStaleFindings`・Python `resolve_stale_findings` 対称）。同一 content（sha 一致）の行は非更新 → D-03 の既存テスト（`test_second_scan_does_not_change_findings_count`・Go の findings snapshot 固定）と衝突なし。skip（A-12）は tx 内に入るためマーキングも走らない。
- 現行判定の定義: join view（`findings.sha256 = files.sha256`）を維持し、`resolved_at IS NULL` を stale マーカーの現判定視として併記。known nuance（記録・実装変更なし）: 内容が過去の sha へ戻ると INSERT OR IGNORE の意味（D-03 正典・insert 意味変更なし）により旧行の resolved_at は cleared されない → join view は正、resolved view は非追従。B1 範囲（列追加＋UPDATE 1）の境界としてこの形を採用。
- 既存 DB への列追加（非破壊・列追加のみ）: Go `ensureFindingsResolvedAt`（migrateSchema の `current < 10` 追加）、Python `ensure_schema` に `ensure_findings_resolved_at`（列追加のみ）を対称追加。
- 固定テスト: Go 2 件（migration 列追加非破壊・内容変化再 scan の stale マーキング/join view）、Python 1 件（probe 同形状・現判定と食い違わないことの固定）。
- UI: 読み取り口なし（grep 確認済み）→ 画面確認該当なし。
- 検証方法（合格条件固定）: `python -m pytest tests/ -v`、`go vet . && go test -count=1 -timeout 300s .`（app）、実装後 probe 再実行（scratch・`docs/ai/tmp/assess-20261106-1056/`）で 内容変化後 stale 行マーキング・join view 0 行・同一 content 再実行不変を証拠として記録。

### 推定規模・優先要素

- 推定規模: 小〜中（B1 = 列追加 1 + scan tx 内 UPDATE 1 + Go/Python 対称 + 固定テスト。A = 記録のみ）。
- 重要度: 中（UI 表示・本番読み取り口がなく、直接影響は DB 直接照会意味と将来の報告機能限定。ただし構造検査記録の整合は D-03 規約の実験地）。
- 緊急度: 低（summary・search card は files current state 基準で findings 非依存・常時稼働挙動は不変）。
- リスク: B1 低（非破壊・列追加のみ）。B2 高（正典決定矛盾）。B1 の既存テスト固定（D-03 の scan 2 回行数不変、`tests/test_scan.py::test_second_scan_does_not_change_findings_count`）とは同一 content 時はマーキングが発生しないため衝突なし。
- 依存: D-03（正典決定）、R-04（files current state 意味）、A-12 skip path（skip は findings 再 insert を走らせない・skip 時はマーキングも走らない）、scan_cache updatable 意味（cache と history の区別）、意味変更の承認ゲート。

## Verification

- 実測（scratch 一時 DB・既存テストと同形状 `scan.open_db` + `scan.ensure_schema` + 正典 `schema.sql`、実際の完成フォルダ・実 DB 不使用、structural のみ・VT/YARA 呼出しなし）: `docs/ai/tmp/assess-20261106-1056/r17_probe.py`（出力 `r17_probe_out.txt`、2026-11-06 15:17Z・elapsed 1.69 s）
  - scan1 hit: findings 1 行（rule=structural・detail=magic_mismatch:cbz_vs_png・sha256=361df9b8…）、files = hit・sha256=361df9b8…
  - scan2（同 path・内容変更 → clean zip → pass）: findings **1 行のまま残存**（sha256=361df9b8… の stale 行）、files = pass・sha256=ae54d63c…。join view（`findings.sha256 = files.sha256`）= **0 行**、stale 行（sha256 ≠ files.sha256）= **1 行**。
  - scan3（同一 content 再実行）: findings 1 行不変（INSERT OR IGNORE skip・D-03 合格条件維持）。
  - scan4（内容変更・再 hit）: findings 3 行（stale 1 + 新 2）、join view = 2 行（新 sha256 のみ）。残存 rule 観測 = `structural` のみ（`no_full`/`error` rule は生成されない）。
- UI 照合（読み取り・grep 実測）: `listFindings` 0 件・`/api/findings` route なし・search card 応答に findings なし・UI html に `finding` なし → findings の UI 読み取り口なし（画面確認該当なし・Drive スクリーンショット該当なし）。

### 実装後の検証（2026-11-08・B1 実施）

- 変更ファイル: `schema.sql`（findings へ `resolved_at TEXT` 追加）、`app/main.go`（schemaVersion 9→10・`ensureFindingsResolvedAt`・`resolveStaleFindings`・scan tx 内呼出し）、`src/scan.py`（`ensure_findings_resolved_at`・`resolve_stale_findings`・`process_file` の tx 内呼出し・対称）、固定テスト 3 件（`app/main_test.go` 2 件・`tests/test_scan.py` 2 件追加）。
- `python -m pytest tests/ -v`: **59 passed / 1 failed in 13.31 s**。1 件の失敗は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（既存 failure・R-17 と無関係・本項目では未処置）。R-17 固定テスト: `test_ensure_schema_adds_findings_resolved_at_without_deleting_rows`・`test_rescan_content_change_marks_stale_findings_resolved` PASSED（内容変化後 stale 行マーキング・join view 0・同一 content 再実行 rows_identical=True・再 hit で join view=新行のみ）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet clean / `ok hitomi-hub/app 24.168s`（2026-11-08）。R-17 固定テスト: `TestMigrateSchemaAddsFindingsResolvedAtWithoutDeletingRows`（列追加のみ・既存行Survive・unique key 維持・user_version 10）と `TestRescanContentChangeMarksStaleFindingsResolved`（run1 hit→run2 内容変化 pass で stale マーキング・join view 0→run3 再 hit で新行 unresolved・join view=2→run4 同一 content 再ハッシュで resolved_at 再マーキングなし・行数不変）PASSED。
- 試行回数: 2。試行1 の失敗はテスト側の照会パラメータ誤り（`files.path` は rel 基準・絶対 path を渡した）。原因分析: run4 の stale 行照会が不一致で no rows。変更点: rel 基準の照会へ修正（product code の変更なし）。試行2 で合格。
- probe 再実行（scratch・同形状 `docs/ai/tmp/assess-20261106-1056/r17_probe_after.py`・出力 `r17_probe_after_out.txt`・elapsed 1.94 s・VT/YARA 呼出しなし）:
  - scan1 hit: findings 1・unresolved 1
  - scan2（内容変更 → pass）: findings 1 残存・marked 1（resolved_at 設定）・unresolved 0・join view **0 行** → 現行判定と食い違わない（合格条件）
  - scan3（同一 content）: findings 1 不変・rows_identical=True（D-03 維持）
  - scan4（再 hit）: findings 3・marked 1・unresolved 2・join view 2（新 sha256 のみ）
- 画面確認: 該当項目なし（UI 読み取り口なし grep 確認済み）。
- 実装前実測（記録のみ・ソース未変更）は上記の節のまま。commit 前に local==remote（HEAD `5db9042`・clean）確認済み。


- `go vet . && go test -count=1 -timeout 300s .`（app）: vet clean / `ok hitomi-hub/app 24.903s`（2026-11-06）。この挙動（旧 sha256 行の残存）を固定するテストなし。
- `python -m pytest tests/ -v`: 54 passed / 1 failed in 9.81 s。1 件の失敗は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md ポインタ）で R-17 と無関係な既存失敗（R-04 と同一記録・本項目では未処置・未触）。
- ソースコード変更なし。変更は本記録と `docs/ai/tmp/assess-20261106-1056/` のみ。`git status --short`: docs/ai 配下の記録のみ（他項目の変更は別セッション・非触）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
