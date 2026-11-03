# B-04: acquisition のフォルダ名と確認処理の不一致

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（静的）。`app/acquisition.go:88`（`acquisitionCheck`）は `acquisitionTitleFromDB`（無加工 title）で folder を確認するが、`app/acquisition.go:163`（`acquire`）は `acquisitionTitleName`（`/ \\ : * ? " < > |` と制御文字を `_` へ）で folder を作る。title に `?` などを含む gallery では check が指す path と acquire が書く path が食い違う（check は not_exists/confirm を誤報告）。合格条件: 無効文字を含む title の gallery で check と acquire が同一 folder を指すこと。推定規模: 小。UI 実操作確認は実装段階（DB に該当 title が必要なため、テスト fixture で確認）。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
