# 検索結果カードの status frame と summary reset（追加要件 2026-11-05）

**Status**: 承認済み（ユーザー追加要件 2026-11-05 07:20）
**Date**: 2026-11-05
**対象タスク**: `docs/ai/tasks/2026-11-05-search-result-state-badges-implementation.md`

## Supersede

この decision は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` の「badge 行 inline に `data-reset="viewed|read"`」という個別 button 配置設計を supersede する。既存の `reset-read` / `reset-viewed` の状態意味は維持し、その上位として `target=all` のまとめリセットを追加する。

## 追加設計

1. 何かしらの status が設定されている card は見た目上はっきり区別できる枠を持つ。
   - 実装は `.card.has-status { outline: 2px solid #8ab4f8; border-radius: 6px; }` を使う。
   - card height contract（130px）を壊さないため、layout に影響する `border` や `padding` ではなく `outline` を使う。
2. リセットは flag ごとの個別 button にしない。badge 行に同一行のまとめ reset button を1つだけ置く。
   - button は `data-reset='all'`。
   - click handler は `POST /api/reset` の `target: "all"` へ送る。
3. `target=all` は persisted status flags をまとめてクリアする。
   - `last_page = 0`
   - `fully_read = 0`
   - `viewed = 0`
   - `favorite = 0`
4. `saved` はファイル由来の判定なので、ファイル削除を伴わない summary reset ではクリアしない。
   - `saved` status は card の visual frame の対象として表示する。
   - reset button は persisted user status（viewed / read / favorite）があるときだけ表示する。
