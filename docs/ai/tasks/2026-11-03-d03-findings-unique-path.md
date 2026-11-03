# D-03: findings の UNIQUE に path がない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測、2 点）。(1) unique index が `(sha256, rule, detail)` で path を含まない（`schema.sql:81`, `app/main.go:299`）→ 同一内容の別 path は first row のみ。(2) Go が新規作成した DB には unique index が無い：scratch DB `PRAGMA index_list(findings)` = `idx_findings_sha256` のみ、canonical DB には `idx_findings_unique` あり（migrate 済み）。scan を 2 回実行すると findings が 5 → 10 行へ重複（`docs/ai/tmp/assess-20261103-1220/repro_scan.py`）。対応: `ensureSchema` の DDL に unique index を追加（path を key に含めるか別解）。合格条件: 新規 DB で `INSERT OR IGNORE` が重複を skip、2 scan で行数不変。推定規模: 小〜中。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
