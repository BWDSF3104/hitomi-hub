# 検索結果画面の3状態 badge と手動リセット：設計判断の承認（2026-11-05）

**Status**: 承認済み（ユーザー 2026-11-05 03:29）
**Date**: 2026-11-05
**対象タスク**: `docs/ai/tasks/2026-11-05-search-result-state-badges.md`
**実装開始指示**: 未受領（本記録は設計判断の確定のみ・ソースコード無変更）

## 承認された設計

1. **表示済みの記録時点**: `openGallery()` の route 入力時点（`app/viewer.html:1113`、await 前）。新規タブ開封でも viewer document 側の事件として記録される。描画到達基準は採用しない（galleryinfo fetch 失敗・`page out of range`（`app/viewer.html:1087-1090`）で取り逃すため）。
   - `viewed=1`・`viewed_at` は初回のみ（reopen は idempotent）。
   - 書き込み口は `POST /api/viewed`。display path を await しない（fast-nonblocking・generation guard 維持）。
   - **リンククリック済みフラグ（`clicked`）は新設しない**。新タブ開封は既に viewer document の load 事件で捕まるため優位性がない。新設する場合は `viewed` と分離し、false positive 源（同名日本語版 anchor `app/search.html:175`・sidebar link・右クリック/中ボタン・search page が viewer 定義の状態を書くことによる ownership の混ざり）を明示する条件付き。
2. **既読の判定**: 最終 page 到達で auto 記録（scene_indexes の最後、または `files[]` 最後）。手動「読み完了」button は override として維持。到達のみで既読（その page が `no_full` でも既読）。auto post は既存の debounced read helper 内で行い、`showPage()` 表示経路には post を入れない。
3. **保存済みの定義**: 指定フォルダ（config `gallery_dir`、既定 `Galleries`）内に全 entry の local ファイルがあること。部分保存は badge に出さず meta 側の `local N/M` で表示。判定は card API の既存 `entryLocalFormats` ループで計算（追加 I/O なし）。
4. **手動リセット**:
   - cascade: `reset-viewed` → `viewed=0`・`fully_read=0`・`last_page=0`（`fully_read` は `viewed` を含意するため）。`reset-read` → `fully_read=0` のみ（`viewed`・`last_page` 保持）。
   - 配置: 検索 card の badge 行 inline（同一行・`data-reset="viewed|read"`）。viewer nav には追加しない（一覧の所有ページは検索ページ）。リセット後の表示反映は既存の手動 refresh（`#lists-refresh`）で許容。
5. **badge の状態データ源**: card API（`searchResult`）が `viewed` / `saved` を返す（案A2）。`readStateRow` は filter なし（`app/library.go:74-85`）で既読を正しく返せる。現行 badge gap（`listRows("unread")` が `fully_read = 0` filter・`app/main.go:1226-1236`）は card 由来 badge で解消。sidebar 一覧の maps は維持し、card 応答で補完する。

## 維持する制約

- `schema.sql` の `read_state` へ `viewed INTEGER NOT NULL DEFAULT 0`・`viewed_at TEXT` を追加、`schemaVersion` 5→6。迁移は列追加のみ・既存行削除しない（`ensureScanCacheSizeMtime` パターン、D-03 非破壊規約）。
- `setFavorite` は viewed を立てない（`last_page=0` 行と表示済みの混同を防ぐ）。
- card height contract（`--search-card-height: 130px`・`overflow: hidden`）を越えない。badge 行へ行を追加しない。
- 検索/閲覧のページ分離（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`）と TN display policy、full image 経路（`AGENTS.md`「絶対に壊してはいけない条件」）は無変更。
