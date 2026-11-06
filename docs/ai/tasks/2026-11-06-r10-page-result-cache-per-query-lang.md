# R-10: ページ送りのたびに searchAllIDs の全件 intersection を再計算

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。案: (query, lang) 単位で最終結果を短 TTL キャッシュし page移動・prefetch・firstStatus で共有。既存 term 単位 searchCache は維持。記録: R-10

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
