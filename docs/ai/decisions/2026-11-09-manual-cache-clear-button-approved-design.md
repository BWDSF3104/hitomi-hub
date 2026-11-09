# 2026-11-09 — 設定画面の手动キャッシュクリアボタン（設計承認）

**Date**: 2026-11-09
**Status**: active（ユーザー承認 2026-11-09 03:23 UTC「推奨を承認する」）
**Task**: `docs/ai/tasks/2026-11-09-settings-manual-cache-clear-button.md`

## Decision（承認済み）

1. 削除対象は disk cache 層（`cache/images` full・`cache/images/thumbs`・`cache/galleryinfo`）＋ `cache_entries` DB 行 ＋ server memory cache 層。
2. 「リスト化前」を含む。定義は `cache_entries` に行がない cache file（表生成以前の実 cache・backfill 禁止分）。削除対象の列挙は file name 照合で行う。
3. 件数判定の filesystem scan 禁止条（R-15）は無変更。削除時のファイル列挙は「件数判定」ではなく「削除対象列挙」条例外として認める。UI の削除前表示は DB 行だけ（walk を UI preview に使わない）。
4. `scan_cache`（VT verdict cache）と `hitomi.db` は対象外。完成フォルダ `Galleries`・`acquisition` は対象外（`docs/ai/decisions/2026-11-09-completion-folders-no-delete-rule.md`）。
5. 確認は2段階。削除前件数（DB 行）を表示 → confirm を満たして実行。不可逆で undo はない。
6. 削除中に書き込み中の path（singleflight group）と cache temp は skip して報告。削除失敗の file は行を保持（prune と同じ流儀）。
7. 設定 allowlist（config key）は増やさない。clear は操作であり設定値ではない。

## Supersedes

`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md:48` の「削除操作は画面にない（正典 `kilo-safe-remove`）」条のうち、cache 層に限定した画面削除ボタンの点のみ supersede。正典 `kilo-safe-remove`（ユーザー操作）・完成フォルダ不削除・LRU 適用範囲条（`hitomi.db` 削除対象外）は維持。

`docs/ai/decisions/2026-11-08-r15-cache-entries-table-supersede.md` の「件数・作成順・status を filesystem scan で数えない」条は無変更。削除列挙の例外のみ追加。

viewer page の cap 通知 UI 禁止、TN 表示源禁止、TN バイトを full cache に数えない、full 判定、永続先、atomic write、singleflight、gate、`x-cache` 意味は無変更。
