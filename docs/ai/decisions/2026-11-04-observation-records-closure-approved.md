# 観察記録3件の締方向承認（2026-11-04・ユーザー承認）→ 第12実行

**Status**: 判断確定
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220

## 承認内容

1. **`2026-11-01-viewer-cdn-time-window-404`**: 再観察は実施しない（窓運依存で低価値）。strict full-image actual pixel pass は control evidence（既存 cache hash `86f0dba8…` の `1/1` 表示確認済み・decision 記録済み）で充足と判定し解決・archive。natural window observation は optional。窓依存 404（過去 gallery・DNS 3番目解決失敗）は環境挙動として記録維持。コード作業なし。
2. **`2026-11-03-search-api-response-field-ids`**: 運用メモ（規約確定・code 変更なし）として archive。作業なし。
3. **`2026-11-04-c05-non-target-caches-unbounded`**: mock 実測（外部 API 呼出しなし）を即実施。`indexTopCache` の version rotation key 成長と `noFullCache` の stale key 残留を実測し、有意な成長を示せば **`indexTopCache` のみ**項目化（既存 `boundedCachePut` 適用・テストで固定できる bounded case・単独 commit）。`noFullCache` は C-08 negative cache 規則（TTL hit 時 delete・singleflight・再試行 1 回）接触のため項目化・実装に別承認を要る（本実行では変更しない）。`versions`/`versionAt`/`versionForceAt` は field 数ぶん実質 bounded・非触。

## 第12実行

- 親記録処理: #1・#2 の Status 更新＋archive（本 decision 引用）。
- 委譲⑤: C-05 実測（mock・外部呼出しなし）＋条件付き `indexTopCache` 実装（単独 commit）。実測が有意成長を示さない場合は実装せず報告のみ（コード変更なし・commit なし。記録は親が反映）。
- 実測・実装の合格条件は個別タスク記録 `docs/ai/tasks/2026-11-04-c05-measure-and-indextopcache-bounded.md` に記載。正典 `task_manager.py` で運用。plan ファイルへの progress 追記は親の記録のみ。
