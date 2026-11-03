# D-08: 未使用コード

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

確認済み（production 未使用）: `fileExt`（`app/hitomi.go:322`、参照は `searchhtml_test.go` のみ）、`imageExts`（`app/scan.go:17`、参照なし）、`Limits.MaxRequestsPerMinute`（parse のみ、gate は per-kind interval を使用）、`Search`/`SearchQuery`（`SearchQuery` の参照はテストのみ、production は `SearchHTML`/`IndexIDs`/nozomi path を使用）、`nozomiURL`（`Search` 経由のみ）。誤り: `pageSource` は未使用ではない（route `app/viewer.go:69` + handler `app/viewer.go:573`）。追加: `app/ui.html:1458` の `current.favorite = current.favorite;` は no-op。対応: 削除 or 使用箇所の明示（削除は実装段階、テスト参照の関数は移動）。推定規模: trivial。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
