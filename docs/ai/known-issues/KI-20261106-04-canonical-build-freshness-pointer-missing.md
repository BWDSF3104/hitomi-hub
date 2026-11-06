# 既知問題: `test_canonical_build_freshness_rule_exists` が現 HEAD で失敗（AGENTS.md pointer 集約）

**Status**: 未解決（既存 failure・assess-20261106-1056 の全項目検証で 54 passed / 1 failed）
**Date**: 2026-11-06
**Source**: assess-20261106-1056（R-10 が親へ報告。R-12・R-14・R-15・R-16・R-17・R-18 で踏襲確認）→ `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 機構（実測で確定）

- `python -m pytest tests/ -v`（ルート）で `test_canonical_build_freshness_rule_exists` が失敗。
- 原因は commit `e018c1c`（`docs: 確認テスト系ルールを単一ファイルへ集約`）で AGENTS.md の Rules 条が `docs/ai/rules/confirmation-test-rule.md` へ集約され、テストが照合する literal pointer `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` が AGENTS.md から消えたこと。
- コード・テストの破損ではなく、正典 pointer とテスト期待値の不一致。

## 影響

- 項目単位の検証は通過（`go vet . && go test -count=1 -timeout 300s .` は全項目 ok）。pytest の合格条件を「54 passed」で報告する形になった（本 project の code verification 条では pytest 合格が完了基準）。
- 実装項目の完了判定時に、この failure を既存の別問題として扱うか、テスト側を現正典へ合わせるかの判断が必要。

## 次の対応（未実施・承認が必要）

- AGENTS.md は write-protected（明示承認なしにエージェントが変更不可）。テスト側を `docs/ai/rules/confirmation-test-rule.md` の pointer へ更新する案が最も適切（既存ルール集約の決定と整合）。実装開始指示が必要。
