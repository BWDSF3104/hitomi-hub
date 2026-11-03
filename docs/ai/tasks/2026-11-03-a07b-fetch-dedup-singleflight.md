# A-07b: fetch の重複排除がない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

機構は事実。singleflight は cache 書き込みのみ（`v.cacheWriteGroup`, `app/viewer.go:300-313`）。fetch への重複排除はない。過去の設計決定で singleflight の scope が「cache write のみ」と合意済みなので、fetch dedup の追加は方針変更の提案として承認が必要（未承認の実装は不可）。TN cache write path（`app/viewer.go:633-636`）は singleflight を使わない。合格条件: 同一 hash への同時リクエストで remote 呼出が 1 回（設計が fetch dedup を認めた場合）。推定規模: 中（設計判断込み）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
