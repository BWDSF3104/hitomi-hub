# R-19（未検証）: scene_indexes のある gallery でページ番号の基準が食い違う可能性

**Status**: 未解決（未検証・読み取りのみ）
**Date**: 2026-11-06
**Source**: 外部指摘（Claude）2026-11-06 項目 17 → `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 読み取りで確認できた機構

- strip は `scene_indexes` の先頭から末尾-1 までを `sceneIndex + 1` を page として items にする（`app/viewer.html:444-453`）。
- `current.count` は `sceneIndexes.length ? sceneIndexes.length - 1 : g.files.length`（`app/viewer.html:256-263`）。
- `movePage` の上限と pagenum input の上限は `current.count`（`app/viewer.html:527-537`、`1426-1429`）。
- `badge` は `sceneIndexes[idx+1] - sceneIndex`（scene ごとの枚数）なので、scene 内の枚数分だけファイル数が scene 数より多くなり得る。

## 影響（未確認）

ファイル数 > scene 数（scene 境界間の枚数）の場合、`current.count`（scene 数基準）が最終ファイルの page 番号より小さく、`次` で末尾のファイルへ到達できない可能性がある。実 gallery での再現は未確認。

## 次の対応（未実施）

- 実 gallery fixture または live galleryinfo で「scene 数 < ファイル数」の実例を確認してから判定。実例があれば page 基準（scene 基準か files 基準か）の設計判断を要る。読み取りのみで実装はしていない。
