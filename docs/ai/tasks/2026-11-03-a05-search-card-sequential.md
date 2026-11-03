# A-05/C-01: 検索カードの逐次取得

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/ui.html:1355-1375` の `loadSearchCards` は ids を 1 件ずつ `await api("/api/search-card/<id>")`。server gate は `MinIntervalMS = 1000ms`（`app/hitomi.go:128-144`）。実測 `#perf-status`: `cards 33 30552ms・cache miss=32 cache=1`（fresh tab, `female:yuri`, 2026-11-03 03:40）。C-02: `app/ui.html:1265-1268` の `loadSearchCardCover` が card ごとに `await currentPriorityDone`。対応: 小規模並列（例 4）+ gate 遵守 + 現 card 優先顺序維持。generation guard は維持（weak にしない）。合格条件: mocked fetch で直列待ち回数が ids 未満、実機の cards 時間が短縮、stale card が DOM に残らない。推定規模: 中。A-05/C-01/C-02 は同一項目（記録は A-05、C-01/C-02 へ相互参照）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
