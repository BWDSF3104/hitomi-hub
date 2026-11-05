# jump-to-first-status-page: 設計判断の確定（2026-11-05）

**Status**: 確定（ユーザー 2026-11-05 08:05）
**Date**: 2026-11-05
**対象タスク**: `docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`

## 確定した判断

1. ステータス範囲: badge 4状態のいずれか（既定）+ 絞り込み可（単一/複数選択）。UI は「いずれか」+ 各 flag checkbox、CLI は `--status` 値
2. `saved`: 定義は本件で変えない（現行正典 `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項を使用）。定義自体の将来確定は別タスク `docs/ai/tasks/2026-11-05-saved-status-definition.md`
3. `orderbydir=random`: 無効（`docs/ai/decisions/2026-11-05-jump-first-status-page-random-order-disabled.md`）
4. hit card の highlight: **付与**（page 移動時に `.card.highlight` + scrollIntoView）
5. 入口: UI button + CLI `search --first-status`

## default 案として確定扱い

- target page = current page: 移動せず highlight のみ
- hit なし: current page にとどまり status 行へ「該当なし」
- jump endpoint / CLI 計算は read-only。DB 記録は target page 描画時の既存 `recordDiscoveredBatch` のみ
- 判定は `searchCache`（key `area|tag|lang`・TTL 1h）の同一 dataset list を共有（既存 idle prefetch と重複 fetch なし）

## 実装開始指示

未受領（本記録は判断確定のみ）。
