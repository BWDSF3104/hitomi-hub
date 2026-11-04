# Phase 4 実装 — サーバー側ゲート優先度化（優先3・条件付き・未開始）

**Status**: 未開始（実装開始指示待ち）
**Date**: 2026-11-05
**Next Action**: 実装未開始。Phase 2 実装後の再判定で実施可（並行度1固定なら実施不要＝最安、並行度2なら必要）。実施時は2レーン（猶予300ms・飢餓防止）＋マーカー明示引数。実装前に canonical rebuild・正典 server 再起動（known-issues KI-20261105-01）

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase4-gate-priority`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase4-gate-priority.md`）の実装タスク（設計上「任意」・要否測定つき）。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 3（根拠は評価記録の記載のみ。条件付き・Phase 2 実装後に再判定）。
- 評価記録（有効性「有効（条件付き）」・実測で閉じ・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase4-gate-priority.md`。
- 検討結果の反映: 前提（単一タイムスタンプゲート・FIFO 将来予約連鎖）は実コード一致。実測: in-flight ≤1（Phase 2「リクエスト単位で譲る」）なら全シナリオのゲート起因遅延 ≤1 枠（→実施不要）。Phase 2 推奨の並行度 2 では data 約 1.9 枠・image 約 1.8 枠を再現性ありで超過（→並行度 2 を採るなら Phase 4 必要）。**決定点: Phase 2 実装で並行度 1 を固定すれば本項目は実施不要（最安）**。
- 実施する場合の推奨案: 系統ごと `lastFg`/`lastBg`＋フォア待機フラグ、バックは「フォア待機なしかつ lastFg+猶予 300ms<=now かつ lastBg+間隔<=now」でのみ予約（飢餓防止）。マーカー=query param（実測 no-op 確認済み）で handler→`Client` 明示引数（data 系統の `SearchNozomiParts`/`Gallery` が `context.Background()` 内部使用＝gap）。metrics 後方互換・ctx sleep（A-06）・gate inject（C-10）・ガードテスト非接触維持。
- 実装前条件: canonical rebuild・正典 server 再起動（known-issues `KI-20261105-01`）＋C-par2 再測定。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
