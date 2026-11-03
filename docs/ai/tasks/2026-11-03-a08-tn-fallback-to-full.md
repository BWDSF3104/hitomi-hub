# A-08: thumbProxy が TN 失敗時に full へフォールバック

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（ガード違反）。`app/viewer.go:620-623` の `thumbProxy` は TN リクエストでも full cache bytes を先に返す。TN fetch 失敗時は `serveLocalImage` → `imageProxy`（full）へフォールバック（`app/viewer.go:639-642`）。TN バイトを full cache に保存しない・TN display fallback 禁止の正典と衝突。対応: full cache 参照順を TN cache/remote TN に限定し、TN 失敗は `no_tn`（502）で返す。実装は方針変更の提案として承認後に実施。合格条件: TN リクエストで full cache が参照されないこと（mocked fetch + x-cache 検証）。フル画像ガード関連: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`, `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`, `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。推定規模: 中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
