# C-06: strip の再構築

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（軽微）。`app/ui.html:1101-1128`（`renderStrip`）は showPage ごとに strip 全 entry（最大 `MaxStripPages`）を innerHTML で再構築し、`highlightStrip()` を呼ぶ。対応: page 移動時は active class 変更のみ（full rebuild は gallery open 時）。合格条件: page 移動で strip の img が再生成されない（mocked DOM で child 数不変）。推定規模: 小。UI 実操作確認（fresh tab, display load）は実装段階。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
