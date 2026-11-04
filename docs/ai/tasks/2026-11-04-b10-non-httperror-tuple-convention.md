# B-10残件: 非 HTTPError（URLError・timeout）の tuple 規約拡大

**Status**: 進行中
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 委譲（第11実行③・単独 commit・テスト固定）

## 意図 / 背景

B-10（close `a303a4e`）の残件。`vt_lookup` の `fetch=None` path で非 HTTPError（`URLError`・timeout）は依然 raise し得る。推奨承認（`decisions/2026-11-04-recommendations-approved-execution.md`）でスコープ拡大：非 HTTPError を tuple 規約 `(payload, status)`（`api_error:network_error`）へ変換し crash しない。Python 参照実装のみ・テスト固定・TTL（30日/2時間）意味不変。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
