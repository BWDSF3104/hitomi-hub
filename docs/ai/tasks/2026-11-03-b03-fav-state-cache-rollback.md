# B-03: お気に入り状態がキャッシュで巻き戻る

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`#fav` click（`app/ui.html:1448-1454`）は POST 後 `current.favorite` だけ更新し、`galleryCache`/`searchCardCache` の `read_state.favorite` を更新しない。実測（gallery 4300588, `?newtab=1` tab）: 1 回目 click で `{"gallery_id":4300588,"on":true}` → `current.favorite` 1。同じ card を再 click（cache hit path, `app/ui.html:1147,1157,1173`）で `current.favorite` が 0 へ巻き戻り、card の `read_state.favorite` も 0。続けて click すると unfavorite 期待で `on:true` が送られ（実測 request body `{"gallery_id":4300588,"on":true}`）、DB の favorite は 1 のまま（`/api/lists` の favorites に残る）。対応: toggle 時に `galleryCache`/`searchCardCache` の read_state を更新、または click 前に `!current.favorite` を DB と突き合わせ。合格条件: cache 経由で gallery を再表示した後も favorite 反映と toggle 方向が正しい。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
