# A-04: plain term に lang が効かない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

機構は事実。`app/searchhtml.go:123` の plain term 分岐は `c.IndexIDs("galleries", term)` を呼び、`state.language` を使わない（nozomi 分岐のみ language 反映）。影響: `?query=furry&lang=english` で plain term は言語条件なしの集合になる。実測（client 累積メトリクス）plain term `furry` で index calls 7・gate_wait 6043ms。hitomi の galleriesindex が言語キーを持つかの正典確認が必要なので、判定は「hitomi 準拠確認待ち」。対応方針: 確認後に lang 適用（nozomi 併用）または lang 無視を明示。推定規模: 中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
