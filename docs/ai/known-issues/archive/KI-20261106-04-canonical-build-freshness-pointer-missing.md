# 既知問題: `test_canonical_build_freshness_rule_exists` が現 HEAD で失敗（AGENTS.md pointer 集約）

**Status**: 解決（2026-11-08 委譲 assess-20261106-1056・テスト照合先を現正典 rule へ更新。`python -m pytest tests/ -v` = 60 passed）
**Date**: 2026-11-06（解決 2026-11-08）
**Source**: assess-20261106-1056（R-10 が親へ報告。R-12・R-14・R-15・R-16・R-17・R-18 で踏襲確認）→ `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 機構（実測で確定）

- `python -m pytest tests/ -v`（ルート）で `test_canonical_build_freshness_rule_exists` が失敗。
- 原因は commit `e018c1c`（`docs: 確認テスト系ルールを単一ファイルへ集約`）で AGENTS.md の Rules 条が `docs/ai/rules/confirmation-test-rule.md` へ集約され、テストが照合する literal pointer `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` が AGENTS.md から消えたこと。
- コード・テストの破損ではなく、正典 pointer とテスト期待値の不一致。

## 影響

- 項目単位の検証は通過（`go vet . && go test -count=1 -timeout 300s .` は全項目 ok）。pytest の合格条件を「54 passed」で報告する形になった（本 project の code verification 条では pytest 合格が完了基準）。
- 実装項目の完了判定時に、この failure を既存の別問題として扱うか、テスト側を現正典へ合わせるかの判断が必要。

## 正典の矛盾と採用源

- 古い decision `docs/ai/decisions/2026-11-04-freshness-rule-test-re-anchor-to-pointer.md`（承認 2026-11-04 11:12）は、AGENTS.md が `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` への参照行を持つことを照合条件として要求する。
- 現行正典 `docs/ai/rules/confirmation-test-rule.md`（Date 2026-11-06・Status active）は「AGENTS.md の Rules ポインタは本ファイル1本のみ」とする。よって AGENTS.md の Rules 条は decision 参照ではなく rule ファイル参照であり、旧 decision の照合条件は状態主張として superseded。
- 採用: 状態主張には新しい正典（rule ファイル）を採用。テストは AGENTS.md Rules ポインタを `docs/ai/rules/confirmation-test-rule.md` へ照合する。旧 decision 2件は歴史的記録として有効（項目4の照合対象2ファイル・失敗メッセージ両ファイル名は `2026-11-04-search-viewer-separation-approach-approved.md` 36行目で有効と明記）なので、存在・内容照合は歴史記録照合として保持する。現行ルールの実質内容（served 画面確認・pytest 合格・build command 正典記載）は rule ファイル側で照合。

## 対応（実施済み・2026-11-08）

- `tests/test_canonical_build_freshness.py` の `test_canonical_build_freshness_rule_exists` を更新:
  - AGENTS.md が `docs/ai/rules/confirmation-test-rule.md` を持つことを照合。不合格時は `AGENTS.md Rules pointer is not aligned to docs/ai/rules/confirmation-test-rule.md`。
  - 旧 literal pointer（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`）の AGENTS.md 照合は外した。
  - `docs/ai/rules/confirmation-test-rule.md` の存在と実質内容（`tests/test_canonical_build_freshness.py` / `served 画面` / build command 正典記載への参照）を照合。
  - canonical build command の正典記載照合（`docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`）は維持。
  - `test_canonical_executable_embeds_current_pages`（byte 照合）は変更なし。

## Verification（2026-11-08・実測）

- 正典 build command（app）: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 出力なし（成功）。現 app からの fresh build として freshness test 合格（stale 化解消）。
- `python -m pytest tests/ -v`（ルート）: **60 passed / 0 failed**（13.00s）。`test_canonical_build_freshness_rule_exists` PASSED、`test_canonical_executable_embeds_current_pages` PASSED。
- `go vet . && go test -count=1 -timeout 300s .`（app）: `go vet` ok。`go test` は既存 failure 2件（`app/viewer_test.go:1818` slice end not found after start、`app/viewer_test.go:2536` initial search restore not found・`app/search.html` の現行構造と test 期待値の不一致）。`git diff HEAD -- app` は空＝app 側は無変更、本件は Python テストのみ変更で Go test を参照しない → 本件の解決条件ではなく、本件変更由来ではない既存 failure として未解決残留（親へ報告）。 ※ stale（現行実測と矛盾・下記「訂正補記」参照・2026-11-09）

## 親の機械確認

- Commit `f6531c6` exists; `git status --short` after the delegation shows only the UFI task-record changes.
- Child report matches this known-issue record: pytest full pass, changed files limited to `tests/test_canonical_build_freshness.py` and this issue record/archive.

## クローズ判定

- 本 issue の「次の対応」案（テスト側を `docs/ai/rules/confirmation-test-rule.md` の pointer へ更新）を実施し、pytest 全件合格で確認。AGENTS.md は無変更（write-protected）。→ 解決。

## 訂正補記（2026-11-09・項目 #17・assess-20261109-1320・記録訂正のみ）

- stale 記述: 上記 Verification（2026-11-08・実測）の「Go 側未解決残留（2 件 failure）」は状態記述として現行実測（2026-11-09）と矛盾し、stale。本補記で訂正（古い状態記述は根拠にしない・日付優先）。
- 機構の訂正: 2 件の失敗は構造不一致ではなく EOL（CRLF）依存。`core.autocrlf=true`（実測）・`.gitattributes` 不在（実測）でワーキングツリーが CRLF であり、`go:embed`（`app/viewer.go:23`）はディスク bytes を埋めるため raw `\n` の完全 adjacency を要求する needle（`viewer_test.go:1818` 相当・`:2536` 相当）が CRLF 内容で不一致になっていた。LF blob（git show）上では構造は照合条件と常に一致（`docs/ai/tmp/assess-20261109-1320/check_literals.py`・`check_eol.py`）。
- 閉機構: commit `b2641df`（2026-11-09 01:08・UFI-1 dedicated Series/Type filter）が 2 assertion を EOL 非依存形へ re-anchor（`sliceBetween(..., "<script>")` 終端、showLists 後の restore 順序照合）。guard 意味維持（results が `.side` に戻らない 2 ファイル分離照合・restore 順序）。現行 CRLF ツリーで全件合格。
- 現行実測（2026-11-09・本実行）: `go vet . && go test -count=1 -timeout 300s .`（app）= vet ok / test ok（26.999s・失敗なし）。`python -m pytest tests/ -v`（ルート・正典 build fresh 後）= 65 passed（21.98s）。該当 2 テスト個別 PASS（`go test -v -run "TestEmbeddedUISearchPageLayoutSeparatesResultsFromSideInputs|TestEmbeddedUIPreservesSearchStateAcrossReload"` → 2 件 PASS・1.560s）。正典 build（app）`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` 成功（fresh）。画面確認: UI 項目でないため該当なし（Drive 保存なし）。
- 採用文書: 日付優先により現行実測（2026-11-09）と `b2641df` を採用。矛盾した古い状態記述（2026-11-08 の未解決残留）は上に変更済み（stale マーカー付与・原文は歴史的記録として保持）。
- 代替案（承認要・実施しない・記録のみ): `.gitattributes` で `app/*.html` を LF 固定し EOL 依存脆性を根源から除去。現行 assertion は EOL 非依存なので追加措置不要（将来 raw `\n` needle 追加時に再発リスク）。
- 正典流儀: 本補記は archive エントリへの状態記述訂正（1 エントリ 1 ファイル・日本語）のみ。テスト実ファイル・既存テスト・AGENTS.md（write-protected）は無変更。個別タスク記録: `docs/ai/tasks/2026-11-08-n17-pytest-freshness-pointer-missing.md`。
