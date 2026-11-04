# C-05観察: 非対象 cache 実測（mock）＋条件付き indexTopCache bounded 化

**Status**: 進行中
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 委譲⑤（第12実行・mock 実測・単独 commit・noFullCache/versions 非触）

## 意図 / 背景

C-05（close `52ffde2`）の承認範囲は 4 cache のみ。非対象 cache（`indexTopCache`・`noFullCache`・`versions`）は TTL だけで上限量なし（既知 issue `docs/ai/known-issues/2026-11-04-c05-non-target-caches-unbounded.md`）。項目化条件（memory 使用の実測根拠）を mock 実測で満たせる。ユーザー承認（`decisions/2026-11-04-observation-records-closure-approved.md`）: 有意な成長を示せば `indexTopCache` のみ項目化（既存 `boundedCachePut` 適用・テスト固定・単独 commit）。`noFullCache`（C-08 規則接触）と `versions`（実質 bounded）は非触・別承認。外部 API 呼出しなし。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
