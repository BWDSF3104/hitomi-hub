# 残判断項目の推奨承認（2026-11-04・ユーザー承認）→ 第11実行

**Status**: 判断確定
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220

## 承認内容（ユーザー 2026-11-04「推奨で続行」）

1. **flaky LRU tie**（known-issue `2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`）: 修復実施。方式はテスト側決定化（fixture で tie を排除・insertion order/sizes を確定）。production LRU 意味を変えない。単独 commit。
2. **C-05 残件（unbounded caches）**: 記録のみ維持（観察）。項目化は memory 使用の実測根拠が出た時点で、テストで固定できる bounded case として扱う。issue は open のまま（Status を判断反映へ更新）。
3. **B-10 残件**: スコープ拡大を実施。`URLError`・timeout 等の非 HTTPError を tuple 規約（`(payload, status)`・`api_error:network_error`）へ変換し raise しない。Python 参照実装（`src/scan.py`）のみ。単独 commit。
4. **manual full-image retry**: 「natural production error」の追及は不要。合格基準は control network failure（500・`x-cache` なし）＋ actual UI click（`TestEmbeddedUIManualFullImageRetryPassCriteria`・combined 条件で実装・assert 済み）。issue は基準確定として解決・archive。natural reproduction は optional（デバッガは forced 再現のみで natural 基準を満たさない。既存 network interception が再現手段）。
5. **gg polarity issue**: faithful parse `f76dd7a` 実装済みで premise 解消 → 解決更新・archive。コード作業なし。

## 第11実行（委譲4項目・1件ずつ・単独 commit）

- ① B-06残件（Go・`app/vt.go` empty results 基準小修正・承認済み方針。テスト固定。任意で live 1 回 200 応答基線固定）
- ② B-07残件①（Python・YARA error 表現を Go 表現 `not_evaluated`/`scan_source` へ揃える・テスト固定）
- ③ B-10残件（Python・非 HTTPError tuple 規約・テスト固定）
- ④ flaky LRU tie（Go テスト側決定化）

実行固有 ID: `assess-20261103-1220`（継続）。正典 `task_manager.py` で項目タスクを作成（第11実行）し、項目完了時に end。plan ファイル（close 済み）への progress 追記は親の記録のみ・子は commit しない。
