# Canonical build freshness test のポインタ実態へ re-anchor（assess-20261104-1004 残件）

**Status**: 承認済み（ユーザー 2026-11-04 11:12・案1）
**Date**: 2026-11-04
**実行固有ID**: `assess-20261104-1004`（残件処理）
**関連**: `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`（AGENTS.md 条はポインタのみ）、`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`（項目4）、`docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`

## 変更（何照合を何へ）

`tests/test_canonical_build_freshness.py` の `test_canonical_build_freshness_rule_exists` を、AGENTS.md 本文の直接記載3 literal（`tests/test_canonical_build_freshness.py` / build command literal / `UI confirmation`）照合から、次の照合へ re-anchor した（ユーザー承認 2026-11-04 11:12「推奨で承認」）:

1. `AGENTS.md` が正典 decision への参照行を持つこと（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`）
2. 参照先 decision が条の実質内容を持つこと（項目4の3点: `app/viewer.html`・`app/search.html` の2本照合、失敗メッセージに両ファイル名、画面確認は viewer 基準＋search served 画面）
3. canonical build command（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）の正典記載を decision 側へ統一: `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md` に正典 build command を追加し、テストはその decision を照合する

- `docs/ai/usage-memo.md` は照合対象にしない（運用メモ）
- 失敗メッセージは対象ファイル名を明示（「AGENTS.md 条はポインタのみ」の正典と矛盾しない文言）
- `AGENTS.md` は write-protected・変更禁止。テストの目的（条が正典として参照され、実質内容が decision にあることを担保）は維持
- byte 照合の目的と `test_canonical_executable_embeds_current_pages` は変更しない

## build command の正典置き場所

decision `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`（AGENTS.md ではない）。AGENTS.md 条はポインタのままとする。

## supersede の扱い

既存 decision の内容の supersede ではない。方法追随の記録: AGENTS.md 条のポインタ化（`2026-11-04-agents-md-rule-pointer-ization.md`・ユーザー承認 2026-11-04 09:32）により、条の実質内容を持つのは decision 側であり、テスト照合先を AGENTS.md 本文から decision へ移すだけ。`2026-11-04-search-viewer-separation-approach-approved.md` 項目4（2ファイル照合・失敗メッセージ両ファイル名・served 画面確認）と `2026-11-02-canonical-latest-build-confirmation-rule.md` の条内容（byte 照合・UI confirmation）は有効のまま。

## 検証

- `python -m pytest tests/ -v`: re-anchor 後の該当 test 含む全件合格（結果は `docs/ai/tasks/2026-11-04-ui-vestige-review-search-page.md` の残件処理記録へ追記）
