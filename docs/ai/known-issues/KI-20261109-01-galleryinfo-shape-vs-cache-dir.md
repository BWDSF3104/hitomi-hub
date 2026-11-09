# KI-20261109-01 — galleryinfo disk 層の置き場が `cache_dir` 既定形とズレる（手动 clear の実測で確認）

**Date**: 2026-11-09
**Status**: 記録（blocker なし・shape 決定待ち点は `app/main.go:80` の正典コメントに既記）
**Task**: `docs/ai/tasks/2026-11-09-settings-manual-cache-clear-button.md`

## 事実

- app 自身の galleryinfo 置き場は `<root>/<cache_dir>/galleryinfo`（`app/viewer.go:133`・`InfoCacheDirName()` は同一 config key `cache_dir` を既定 `cache` で使う。`app/main.go:80`・`:87`）。
- `config.json` に `cache_dir` を `cache/images` と明示すると、app の galleryinfo 経路は `cache/images/galleryinfo` になる。
- 実測（canonical build served・measurement root `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh`・port 8731）:
  - 手动 clear 前の file 数: `cache\galleryinfo` 1345・app 経路 `cache\images\galleryinfo` 0
  - clear 後: app 経路 0（削除 0 件と報告）、`cache\galleryinfo` 1345 のまま
- fresh root（`scratch\clear_confirm2`・port 8743）で app 経路を app 自身の書き込み（`GET /api/gallery/4304794`）で生成して再測定 → clear が `cache\images\galleryinfo\4304794.json` を 1 件削除。動作は正典どおり。

## 解釈

- 手动 clear は app 自身の cache 経路だけを消す（正しい）。`cache\galleryinfo` に残る分は、`cache_dir` 既定形（`cache`）で書かれた過去の file（historical shape）で、現 config の app 経路ではない。
- 件数判定・eviction も app 経路（`infoCacheRoot`）が源。件数表示（`cache_counts`）は `cache_entries`（image cache 表）だけで、galleryinfo を含まない。

## 影響

- 画面の「キャッシュクリア」で消えない galleryinfo file が残ることがある（過去の shape 由来）。データ損失・誤削除はない。
- shape 決定（`defaultInfoCacheDirName` と `defaultCacheDirName` の形）が確定すれば解消する。解消作業は本タスクの範囲外。
