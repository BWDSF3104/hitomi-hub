# A-15: isLoopbackHost が [::1]:port を弾く

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/viewer.go:122-135`（`isLoopbackHost`）は `strings.Cut(host, ":")` で `[::1]:8731` を name=`[` と読む。実測: `POST /api/fav`（invalid body）+ `Host: [::1]:8731` → 403、`Host: localhost`（control）→ 400。したがって正当な loopback Host 形式で POST が拒否される。実運用は `[::1]:8731` で bind しないので影響は限定されるが、機能上の不具合として有効。対応: bracket 形式を剥がして name を比較。合格条件: `Host: [::1]:8731` で POST が same-origin 扱い（400/200）。推定規模: 小（テストは変更しない）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
