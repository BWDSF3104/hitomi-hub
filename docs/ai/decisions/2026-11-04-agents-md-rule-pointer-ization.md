# AGENTS.md の Canonical rule 条を直接記載からポインタ化

**Status**: 承認済み（ユーザー 2026-11-04 09:32）
**Date**: 2026-11-04
**実行固有ID**: `assess-20261104-0749`（残件の解消）
**関連**: `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`（正典ルール項目の残件）

## 決定

- `AGENTS.md`（hitomi-hub）の `## Canonical Latest Build / UI Confirmation Test Rule` 節の直接記載を廃止し、正典 decision への参照（ポインタ）のみへ置き換える。
- 参照先: `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`
- AGENTS.md 側には条件本文・手順を置かない。条の実質内容（照合対象 `app/viewer.html`・`app/search.html`、失敗メッセージに両ファイル名、画面確認は canonical server の served UI・viewer ページ基準＋search ページの served 画面確認）は decision が持つ。
- `AGENTS.md` は write-protected 領域。本編集はユーザーの明示承認（2026-11-04 09:32「AGENTS.mdに直接の記載は廃止しポインタ化」）により実施。以後の条内容変更は decision/ルール側で行い、AGENTS.md は参照行だけを変える。

## supersede の扱い

`2026-11-04-search-viewer-separation-approach-approved.md` のルール内容（2ファイル照合・画面確認基準）は有効のまま。置き換えるのはその実装方法だけ（AGENTS.md の文章を書き換える → 参照行へ）。よって旧 decision の内容 supersede ではなく、方法の supersede として本エントリに記録する（旧ファイルは不変）。

## 状態

- テストコードは項目A1の実装（commit `727d9fb`）で2ファイル照合へ変更済み・合格（`python -m pytest tests/ -v` 51 passed）。
- AGENTS.md の文章と decision の内容のずれ（残件）は本決定で解消。
- 項目A1のタスク記録（`docs/ai/tasks/2026-11-04-assess-20261104-0749-search-viewer-separation-minimal.md`）は完了済みのため Next Action は書き換えない。
- AGENTS.md の編集は未コミット（コミットは承認後に実施）。
