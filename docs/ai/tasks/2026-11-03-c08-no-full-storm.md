# C-08: no_full の連打

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（bounded）。`no_full` は cache されない（`app/viewer.go:695-700` は 404 + `x-cache: no_full`）ため、display/prefetch ごとに remote 照会。実測: gallery 4300588 の full は `no_full`（`/api/image/09e34fdfff2fb198be676b313aa97e942c19f03667ecfca69ef7214fba8ad2a6?format=webp` → 404, `x-cache: no_full`）。`EnsureGG(force=true)` は `GGForceMinIntervalMS`（既定 60000ms）で 1 分 1 回の制限を守り、1 回再試行のみ（正典）。対応: negative cache（short TTL）導入の可否は設計判断。合格条件: 同一 hash の repeated request で remote 呼出回数が減る、`no_full` 判定と x-cache 意味は維持。推定規模: 小〜中。フル画像ガード関連: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`, `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
