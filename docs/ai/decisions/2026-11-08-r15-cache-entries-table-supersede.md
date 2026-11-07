# 2026-11-08: R-15 cache_entries 表の新設（2026-11-02 の「image cache state の DB 表追加不可」条の該当点のみ supersede）

**Date**: 2026-11-08
**Status**: 実装（R-15・assess-20261106-1056・実行固有ID assess-20261106-1056）

## 正典矛盾の処理（判断ルール適用）

- 矛盾: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope.md` の Rejected proposals「Do not add DB tables for image cache state」と Accepted facts「image_files / gallery_files table additions are not accepted」は、newer の `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（確定 2026-11-06 10:52）の「件数・作成順・status は filesystem scan ではなく DB の 1 表（例 `cache_entries`）で持つ」と矛盾する。
- 採用: 日付新しい文書を優先。R-15 の 1 表（`cache_entries`）を正典とする。2026-11-02 の当該条は「image cache state の 1 表（cache_entries）追加」の点のみ supersede。atomic write・temp・singleflight・TN・x-cache・cache root・永続先の条は無変更。

## 定義（R-15 実装形）

- 表 `cache_entries`: hash、format、kind（`full` / `thumbs`）、variant（full は `''`）、created_at、source_status。PK は (hash, format, kind, variant)（TN のファイル名 `hash.format.variant` を行 key に写像するため variant を PK に含める）。索引 (kind, created_at)（作成順・件数判定を SQL だけで行う）。
- `gallery_id` 列（実施段階の追加判断・タスク記録の記載どおり）: 「現在閲覧中の gallery は捨ない」を prune で判定するため、書き込みリクエストの gallery param から記録（なければ NULL）。
- 件数・作成順・status の判定は DB 行のみ。filesystem scan で数えない。書き込み時 1 行 INSERT、prune 時にファイル削除 + 1 行 DELETE。
- 表生成以前の実 cache ファイルは backfill しない（backfill は filesystem scan となり正典ルールに反する）。表に記録のない項目は件数に入らず、prune 対象にもならない（実施段階の判断として記録）。
- disk cap: `cache/images` と `cache/images/thumbs` 各 8000 件（件数ベース・byte 計算なし）。削除は作成順（古い順）。現在閲覧中の gallery の項目は捨ない。
- memory cap（client objectURL）: 4000 件・設定 `viewer.max_object_urls`（0 は既定、下限 1000）。page へは server 経由（`GET /api/viewer-config`）。memory 削除は保存済み status（local）優先 → LRU。現在閲覧中・表示中は捨ない（B-02 保護 key 維持）。通知 UI なし、last access 記録なし、sha256 lookup なし。
