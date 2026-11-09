# 2026-11-09 — 完成フォルダ（Galleries・acquisition）はアプリ側で削除しない（正典条件）

**Status**: 実測で確認済み。正典条件として記録（ユーザー 2026-11-09 00:39 UTC「acquisition は絶対に削除禁止」）
**タスク記録**: `docs/ai/tasks/2026-11-09-settings-value-centralization.md`

## 決定（正典条件）

- 完成フォルダ内のファイル（`Galleries`・`acquisition`）をアプリ側で削除しない。削除は正典 `kilo-safe-remove`（ユーザー操作）のみ。
- prune（上限超過で捨てる）対象は cache 層のみ（full・thumbs・galleryinfo）。完成フォルダは件数にも捨て対象にも入れない。
- DB 側は `cache_entries` 行の DELETE のみ。`files` 表の DELETE は置かない。欠損は `missing_at_verify` / `scan_status = 'external_removal'` の記録のみ（`app/main.go:748-752`）。

## 実測（HEAD で存在する削除経路をすべて列挙）

- `app/galleryinfocache.go:165`（壊れた galleryinfo ファイル除去）、`:190`（prune）、`:205`（temp 後片付け）
- `app/viewer.go:484`（atomic write の temp 後片付け）、`:641`（cache prune のファイル削除）
- `cacheEntryPath` は cache root 内のみ解く（full → `safeCachePath`、thumbs → `safeThumbnailCachePath`、`app/viewer.go:591-596`）
- `safeAcquisitionPath`（`app/acquisition.go:15-25`）と `safeGalleryPath`（`app/viewer.go:662-670`）に削除呼び出しはない
- `DELETE FROM` は `cache_entries` のみ（`app/viewer.go:644`）

→ `Galleries`・`acquisition` へ向く削除経路は 0 件。この条件は追加の実装なしで成立している。

## 補足

- `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md:49` は LRU 適用範囲を cache 層とし、`Galleries` と `hitomi.db` を削除対象にしないと記録済み。`acquisition` は同列に明記されていなかったため、本 entry で完成フォルダ両方を含む正典条件として明記する。
- `AGENTS.md` の「絶対に壊してはいけない条件」節へこの条件を追加するには明示承認を要る（`AGENTS.md` は write-protected）。本 entry は `decisions/` 側の記録であり、`AGENTS.md` は未変更。
