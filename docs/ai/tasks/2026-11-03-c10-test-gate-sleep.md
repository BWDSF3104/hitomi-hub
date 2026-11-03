# C-10: go test が 35 秒前後かかる

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（情報）。実測（2026-11-03, `app` で `go vet .` clean）: `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 35.352s`。原因は client gate（`MinIntervalMS` 1000ms）を mock fetch の sleep で使う test が複数あること。対応: gate wait を test で短縮（`gateForInterval` を inject 可能にする）または test ごとに interval を 0/1ms に設定（正典 gate 意味は維持）。合格条件: test 時間短縮、gate semantics tests は維持。推定規模: 小（テスト変更は実装段階）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
