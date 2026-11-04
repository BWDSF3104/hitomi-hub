# Phase 3 gallery 情報永続キャッシュ: file 方式の採用と運用決定（assess-20261105-0024）

**Status**: 決定・実装済み
**Date**: 2026-11-05
**実行固有ID**: `assess-20261105-0024`
**項目ID**: `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`
**個別タスク記録**: `docs/ai/tasks/2026-11-05-phase3-gallery-info-cache-implementation.md`
**リモートタスク（正典・承認範囲）**: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache.md`

## 決定（採用方式）

- 保存方式: **file 方式**。cache root 配下 `cache/galleryinfo/<id>.json`（1 gallery 1 ファイル）、
  内容は parse 済み `GalleryInfo` の JSON + 取得時刻 `fetched_at`。
  - 理由: 既存 cache 配置の流儀（`docs/ai/decisions/2026-11-01-viewer-persistent-image-cache.md` の
    `cache/images`）と同型。DB スキーマ移行を伴わない、アプリの書き込みが完成フォルダ内に閉じる
    （絶対パス・`%APPDATA%`・temp 展開なし）、壊れたファイルの除去が容易。
  - DB 新テーブル案は不採用（スキーマ移行・非破壊性・バックアップ方針を要す。リモートタスクの
    推奨方式から変更しない）。
- 参照順: memory `infoCache` → 永続 disk 層 → remote（`Client.Gallery`）。実装は `viewer.info()`
  本体（共有 helper）に置く。Phase 2（次ページ先読み）の先読み成果もこの helper 経由で自動で
  永続化され、Phase 2 側の追加作業は不要（独立条件满足）。
- disk hit は network（ゲート）を消費しない。永続 hit は memory へ載せ、remote 成功は memory・永続の
  両方へ書く。

## 鮮度・壊れた内容

- TTL: **30 日**（既定）。定義は `app/hitomi.go` の既存 TTL 定数ブロックの流儀に従い
  `galleryInfoCacheTTL = 30 * 24 * time.Hour`。
- TTL 切れは remote 再取得、成功時は永続ファイルの `fetched_at` を更新。
- **stale 暫定表示は採用**: remote 失敗時のみ期限切れ永続 entry を暫定表示に使う。fetch metrics は
  network 呼出し基準のため stale hit は kind `gallery` calls に現れない。判別は `viewer.infoDiskStaleHits`
  カウンターと stderr ログ行（`galleryinfo stale served id=... age=...`）。
- 壊れたファイル（JSON parse 不可・id 不一致）はミス扱い: ファイルを除去して remote 再取得。
  壊れた内容を根拠にエラーを返さない。

## 上限・evict

- 上限: 件数 **5000** または総容量 **256MB** の先に到達した方（`defaultInfoDiskMaxEntries` /
  `defaultInfoDiskMaxBytes`・`app/viewer.go` の C-05/A-14 bounded 流儀と同思想）。
- evict: **取得時刻（`fetched_at`）古い順**。JSON field 正・parse 不可は mtime fallback。
- 実行タイミング: 書き込み時の間引きのみ（directory scan は超過時のみ・初回書き込み時に
  accounting 同期 scan）。disk 層は access order を持たない（memory LRU と層が別で矛盾なし）。
- テストは insert 時刻を固定して決定化（教訓: `docs/ai/known-issues/archive/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md`）。

## 書き込み

- atomic 書き込み: 既存 A-07a 流儀（同一ディレクトリ内 temp 書込 → close → rename、失敗時 temp
  cleanup、OS temp dir・scratch/・output/ 不使用）。異常終了で中途半端なファイルを残さない。

## 範囲・分離

- 対象は `viewer.info()` 経路（検索カード・gallery 表示・pageSource）。CLI（search/download）は
  `client.Gallery` 直呼びで非共有。共有には helper wiring（構造変更）が必要 → 無理に広げない
  （リモートタスクの申し送り条に従い、実装段階でも広げなかった。申し送り: 将来 `loadGalleryInfo`
  程度の共有 helper で CLI も disk 層を利用できる）。
- フル画像ガード（`StorageURL`/`ImageURL`/`ThumbnailURLVariant`/TN 経路/`imageProxy` 順序/gg 生成/
  `downloadFilesParallel`・UI の `x-cache` 判定）は非接触。メタ JSON のみの専用の置き場で分離維持。
- 検索カードの `x-cache`（memory 有無表示）は変更しない（UX 変更回避）。
- 既存テスト 1 件の再アンカー: `TestViewerInfoCacheEvictsLRUEntriesWhenOverEntryLimit` の
  「memory evict された key は refetch」 assertion は disk 層導入により network 消費なし（永続 hit）へ
  変更（`fetches` 5→4）。memory bounded assertion は維持。これは本機能の効果そのもの（gate 消費消去）で、
  C-05/A-14 の memory 層 assertion と矛盾しない。

## 影響

- `docs/ai/decisions/2026-11-01-viewer-persistent-image-cache.md` は変更しない（画像キャッシュの
  decision。本決定はメタ情報 cache の専用置き場追加で、画像バイト経路の decision を supersede しない）。
