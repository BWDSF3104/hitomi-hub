# R-11b: data lane interval の短縮（方向承認・値未確定）

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 値確定 90 ms（100 ms の点のみ supersede）。下限 guard 50 ms・image lane 60 ms・politeness 1000 ms は不変。実装時に defaultHitomiConfig()（hitomi.go:237-240）と 100 ms を期待する test（hitomi_test.go:1507-1528）の更新が必要。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

data lane interval を承認済み正典値 90 ms（100 ms の点のみ supersede）へ実装。値のみ更新。下限 guard 50 ms・image lane 60 ms・politeness 1000 ms・gate semantics（FIFO・将来予約・grace・lane 直列化）・fetch 口（テスト inject）は非触。

## 調査結果 / 変更内容

- 正典: `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（確定 2026-11-06 10:52）。`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md` の data lane 100 ms の点のみ supersede（同 decision の image lane 125 ms は既に 60 ms 決定で有効範囲外・他 points 維持）。正典 pointer `2026-11-03-a05-...` は実体なし（採用: `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`）。
- 記録の参照行は HEAD で旧番号（実測で訂正）:
  - `defaultHitomiConfig()` は `app/hitomi.go:278-281`（記録記載 237-240 は cache フィールド）。変更: `MinIntervalMS: 100 → 90`。
  - 記録記載 `hitomi_test.go:1507-1528` は `Test429WithoutRetryAfterKeepsExistingBackoffAndNoCooldown`（1495 行）で、100 ms は `RetryBackoffMS`（backoff 公式）。data lane interval ではないため **変更しない**。findings 訂正（優先順位記録・`docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`）で確定した更新対象と一致。
  - 更新対象①: `TestRateGateReturnsImmediatelyOnContextCancellation`（`app/hitomi_test.go:863-894`、guard は 865 行・elapsed 判定 885 行・コメント 861 行）を 90 ms（decision 2026-11-06）へ re-anchor。
  - 更新対象②: `TestConfigExampleHitomiKeysMatchCodeDefaults`（`app/main_test.go` 1371 行付近。記録記載 1304 行は HEAD で `TestLoadConfigLimitsPreservesExplicitZero...`。テスト本体は `defaultHitomiConfig()` と動的比較のためコード変更不要）→ `config.example.json:15` を 100 → 90。
  - `hitomi_test.go:1406` の固定リテラル説明コメント（data `100`）を data `90` へ更新（同項目の正典値記述）。
  - `TestParseRetryAfterRespectsValueAndCapsAtCanonicalConvention`（1542 行）の politeness > data default 判定は動的（1000 > 90）で変更不要。`testGateIntervalMS` inject 使用テスト（976・1007・1074 行）は本番既定値非依存で非触。
- 合格条件: ①`defaultHitomiConfig().MinIntervalMS == 90` ②下限 guard 50・image 60・politeness 1000 不変（既存テストで固定）③config.example.json と code default 一致 ④gate semantics 固定テスト（①〜④）合格 ⑤検証コマンド合格（known-issue KI-20261106-04 の pytest 既存 failure は区別して報告）。

## Verification

- `go vet .`（app）: 合格（出力なし）。`go test -count=1 -timeout 300s .`（app）: 合格 `ok hitomi-hub/app 23.363s`（gate semantics 固定テスト①〜④含む全テスト合格）。試行 1 回で合格。
- 個別根拠: `go test -run "TestRateGateReturnsImmediatelyOnContextCancellation|TestConfigExampleHitomiKeysMatchCodeDefaults|TestParseRetryAfterRespectsValueAndCapsAtCanonicalConvention|TestImageGateUsesLowerImageIntervalSeparately"` → 4 PASS（0.941s）。90 guard・config.example 一致・politeness 1000 > data 90・image 60 固定を確認。
- `python -m pytest tests/ -v`: 1 failed / 57 passed（12.64s）。failure は `test_canonical_build_freshness_rule_exists` = known-issue KI-20261106-04（全項目共通の検証 blocker・既存 failure）。本項目の不合格ではない。AGENTS.md は write-protected のため修復しない。
- UI・画面確認: 適用なし（UI 変更なし・gate 値のみの更新）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
