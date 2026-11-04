# 2026-11-04 残件判断のユーザー承認

第7実行後の残件に対する推奨をユーザーが承認（2026-11-04 02:31）。正典 `docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md`（実施順の推奨）を補完する承認。

## 確定した判断

1. **AGENTS.md Code Scope（D-06 持ち帰り）**: 正典を実態へ更新。プロジェクト `AGENTS.md`「Code Scope」の「検索・ダウンロード・UI・既読/お気に入りの書き込み口は未実装」は事実と異なる（viewer/UI・read/fav・download・search は実装済み）。ユーザー承認により該当行を更新し、未実装は P2 残項目のみとする表示へ変更。
2. **D-07 未検証点**: clean clone での skip 実発火を scratch 内 clone 実測で1回確認する（追加確認。低コストで未検証点を解消）。
3. **P2 #8（AVIF の pdf embed）**: pure-Go decoder は追加しない（移植性方針）。**PDF embed の対象外**と記録に明示。embed 対応は png/jpeg 再エンコード経路のみ。
4. **P2 #9（`bookmark_queries.notify`）**: 用途を**保存検索の新着通知源**として定義（`check`/saved query 系の new 判定と通知の持久化）。実装は今後の項目として `backlog.md` に残す。不要と判断された場合は列削除で閉じる選択肢は保留。

## 実施順（承認済み・第7実行で①完了）

- ① 判定不要項目（実施済み）: A-13・A-06・C-10・D-07・D-06 ✓
- ② A-11 → A-09
- ③ hitomi 準拠確認（A-04・D-09・D-10）→ parity（B-06 残件・B-07 残件①）→ 低優先（B-04・A-12）→ P2 #8/#9 処理

## Verification

- AGENTS.md 更新は本 decision の承認に基づく（write-protected 正典の明示承認）。D-07 clean clone 実測結果は `docs/ai/tasks/2026-11-03-d07-canonical-build-test-premise.md` の Verification に記録。
