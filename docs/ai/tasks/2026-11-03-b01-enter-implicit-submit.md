# B-01: 検索ボックス Enter で前ページが実行される

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測・機構 уточ）。`app/ui.html` の `#search` 内 button に `type` 属性がなく 3 つすべて submit（`search-prev`/`search-next`/`search-submit`、tree order 首位 = `search-prev`）。page 1 では `search-prev` が disabled → implicit submission の default button がないため Enter で検索実行されない（実測: Enter 後 network request なし、`#page` 1 のまま）。page ≥2 では `search-prev` が enabled → Enter で `setSearchPage(-1)` が走し、入力した語で「前ページ」へ移動（実測: page 2 で `female:catgirl` 入力 + Enter → `#/q/female%3Acatgirl/page/1`）。スクショ: `docs/ai/tmp/assess-20261103-1220/02-b01-enter-implicit-submit.png`（Drive へコピー済み）。対応: `#search-submit` に `type="submit"`、prev/next に `type="button"`。合格条件: 1280x900 fresh tab で Enter が current page の語で検索を実行し、prev/next/saved-search の既存動作（setSearchPage）が壊れない。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
