# A-06: time.Sleep が ctx 非対応

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/hitomi.go:139`（gate）と `app/hitomi.go:182,201,215,233`（retry backoff）は `time.Sleep` で ctx を見ていない。UI prefetch の AbortSignal は `fetch` の ctx で効くが、server 側 client の sleep は切断後も最大 backoff（既定 2+4+6=12s）分かかる。対応: `select { case <-ctx.Done(): return; case <-time.After(d): }` へ変更。合格条件: ctx キャンセル直後に呼出が return することを mocked fetch/短 backoff で確認。推定規模: 小（低優先）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
