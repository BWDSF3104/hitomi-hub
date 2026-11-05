# KI-20261105-08 検索 form control の accessible name（修正承認済み）

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: aria-label 追加・テスト固定・正典 build 実機確認

## 意図 / 背景

`docs/ai/known-issues/KI-20261105-08-search-form-controls-missing-accessible-names.md` の修正（ユーザー承認 2026-11-05 18:07）。検索 form の control に accessible name がない問題への最小対応。方針は KI 記載どおり `aria-label` のみ（layout を変えない・visible label による行契約 re-anchor 判断は対象外）。

- 修正対象: `#query`・`#lang`・`#qname`・`#qtext`（`#page` は commit `0cb18b7` で `aria-label="ページ"` 追加済み・維持）
- 並行タスク `docs/ai/tasks/2026-11-05-saved-status-definition.md` はスコープ重複なしのため進行中まま継続（本項目の承認は KI-08 のみ）
- テスト影響確認済み: `form#search` の button 照合（`app/viewer_test.go:3121-3145`）と id 衝突禁止条（`:2671`）は属性追加の影響なし。input 行の exact anchor なし。guard テストを追加して固定する

## 調査結果 / 変更内容

### 変更（修正承認 2026-11-05 18:07）

- `app/search.html`:
  - `#query` → `aria-label="検索 term"`
  - `#lang` → `aria-label="言語"`
  - `#qname` → `aria-label="名前"`
  - `#qtext` → `aria-label="term"`
  - `#page`（commit `0cb18b7` で `aria-label="ページ"` 済み）は無変更
- `app/viewer_test.go`: `TestEmbeddedUISearchFormControlsHaveAccessibleNames` 追加（5 control の aria-label を文字列照合で固定。既存の button 照合 `3121-3145`・id 衝突禁止条 `2671` は影響なし）

### 正典 server 実機確認（fresh target・Playwright browser tool）

正典 build 再 build → 正典 server 8731 再起動（正典 exe・runtime root `Common_Operations\scratch\p3_canonical_fresh`・新 PID 17756・旧 PID 30176 停止）→ `http://127.0.0.1:8731/search` fresh tab 照合:

- `#query` = `検索 term` / `#lang` = `言語` / `#page` = `ページ` / `#qname` = `名前` / `#qtext` = `term`（evaluate 実測。`labels` なしは aria-label のみ方針のため正常）
- `document.getElementById("query")` の解決先 = input（id 衝突禁止条維持）
- console error は `favicon.ico` 404 のみ（本変更と無関係。accessibility 経路の error なし）
- スクリーンショット（通常スクリーンショットルール・Playwright browser画面）: `docs/ai/tmp/ki-20261105-08-search-form-accessible-names.png`

## Verification

- `go vet .`（app）clean。`go test -count=1 -timeout 300s .` → ok 25.525s（新 guard テスト含む）
- canonical build（正典 command）→ `python -m pytest tests/ -v` → 53 passed in 11.24s（`test_canonical_build_freshness.py` の 2ファイル照合 pass）
- served UI 実測（正典 exe 再起動 server 8731・Playwright）→ 上記 5 control の accessible name 一致

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
