# page 基準の正典化: files 基準統一（KI-20261106-01）

**Date**: 2026-11-09
**Status**: active（ユーザー同意 2026-11-09 の推奨A。実装・検証合格）

## Decision

- viewer の page 基準は files 基準（`current.count = g.files.length`・1 entry = 1 page）を正典とする。scene_indexes があっても position 分母・movePage 上限・pagenum guard・読み完了判定・strip items（1 entry = 1 card）は files 基準。
- scene badge 表示（scene start card の枚数）は維持。TN・full 判定・generation guard の正典不変条件には触れない。
- 既存 read_state の page 値は files 基準で解釈するのみ。migration・データ書き換えはしない（不可逆のため）。

## Reason

- app は全ファイルを表示・下載する reader model（1 entry = 1 page）で、hitomi の scene-page 模型（scene 内ファイル表示不能）とは機能目的が違う。外部仕様として 1:1（scene = files index 連続）は保証されない（`gallery.js` は badge>1 の scene を明示的に扱う）。1:1でない sparse gallery（実例 733697: files 210・scene 13）で scene ordinal 基準の count と files 基準の `entryForPage` が矛盾し、最終ファイル到達不可・位置表示不一致・pagenum 拒否が served 実測で発生（`docs/ai/known-issues/KI-20261106-01-scene-page-count-basis.md`、2026-11-09 実測）。
- 代替案 B（hitomi 正典 scene 基準準拠）は scene 内ファイルの表示・既読判定が不能になり app の機能と衝突するため不採用。

## Supersede

- supersede 対象なし（既存 decision・正典ルールを変えない。混合経路の解消のみ）。

## 実装

- `app/viewer.html`: `setGalleryMeta`（count）・`stripItems`（files items + badge map）・`readerItems` second（entryForPage）。テスト re-anchor（`app/viewer_test.go`）+ harness（`tests/viewer_page_basis_harness.mjs`・`tests/test_viewer_page_basis.py`）。
