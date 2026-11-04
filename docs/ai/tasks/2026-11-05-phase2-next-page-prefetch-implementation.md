# Phase 2 実装 — 次ページのアイドル先読み（優先2・未開始）

**Status**: 未開始（実装開始指示待ち）
**Date**: 2026-11-05
**Next Action**: 実装未開始・明示的な実装開始指示待ち。Phase 1（優先1・app/search.html 共有）→本実装の順。推奨=client側スケジューラ（800msアイドル・並行度2・中断・5分クールダウン・マーカー query param）

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase2-next-page-prefetch`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase2-next-page-prefetch.md`）の実装タスク。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 2（全体順 Phase 1→P3→**P2**→P4。根拠は評価記録の記載のみ）。
- 評価記録（有効性「有効」・実測で閉じ・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase2-next-page-prefetch.md`。
- 検討結果の反映（推奨）: client 側スケジューラ（`app/search.html` に閉じる）。発火=現ページ card 完了＋800ms アイドル（「次」hover/focus で即時）。対象=page+1 のみ。取得順 ID→`/api/search-card/<id>`（並行度 2）→TN cover。中断=新規検索/ページ移動/gallery 開/query 入力/`visibilitychange` hidden／429・5xx→全体停止＋5分クールダウン（メモリ保持）。マーカー=query param `priority=background`（実測で応答・cache 挙動不変確定済み。サーバー側は metrics 区分記録のみ・ゲート制御は P4 範囲）。世代は `searchGeneration` に紐づけ。
- 順序依存: Phase 1（優先1・未開始）と `app/search.html` を共有 — 既存実施計画の順 P1→P2。P3 とは file 競合なし（P3 先行なら P2 成果が永続化され P2 側追加作業不要）。P4 の要否判定は本実装の並行度パラメータに依存（1 固定なら P4 不要）。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
