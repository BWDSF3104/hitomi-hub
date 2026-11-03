# C-04: language: 併用で同じ tag を 2 回取得

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（静的＋実測補助）。`app/searchhtml.go:115-121`（`termIDs` の `language` 分岐）は `state.language` を set した後、同じ `state.area`/`state.tag` で `SearchNozomiParts` を呼ぶ → 前置 term 已 fetch した同一 tag を新 language で再 fetch。実測: `female:yuri language:english`（lang=all, limit=1）で nozomi calls が 1 → 3（client 累積メトリクス）。対応: `language:` term だけ language を更新し fetch しない（`state.language` 更新後に最初の term fetch へ反映）、または update 後に 1 回だけ再 fetch。合格条件: mocked fetch で同一 tag の language 違い fetch が 1 回。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
