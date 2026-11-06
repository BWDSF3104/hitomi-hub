# R-20（低優先・方針除外）: GET の /api/* と /files/* は Host を見ていない

**Status**: 未解決（ローカル専用前提のためセキュリティ項目は扱わない方針・判断要）
**Date**: 2026-11-06
**Source**: 外部指摘（Claude）2026-11-06 補足 → `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 読み取りで確認できた機構

- Host / Origin 検査は POST のみ。`withPOSTSecurity` は `r.Method != POST` の場合そのまま next へ渡す（`app/viewer.go:155-172`）。
- `sameOriginPost` は loopback Host と same-origin Origin を要求（`app/viewer.go:174-194`）。
- GET 系（`GET /api/search`、`GET /api/search-card/{id}`、`GET /api/gallery/{id}`、`GET /files/{gallery}/{name}` など）は route 側で Host を確認しない（`app/viewer.go:132-152`）。
- 理論上、DNS rebinding で外部 origin のブラウザから GET 読み取り（既読・お気に入り・保存検索の list 表示、ファイル配信）が可能。書き込みは POST 検査で防げている。

## 影響

ローカル専用運用では実害は確認されていない。2026-11-03 の評価フェーズは「ローカル専用前提のためセキュリティ項目は扱わない」と記録（`docs/ai/tasks/2026-11-03-claude-review-349778d-assessment.md`）。本項目はその方針の範囲外として記録のみ。

## 次の対応（未実施・承認が必要）

塞ぐなら GET にも loopback Host 検査を入れる（既存 `isLoopbackHost` を流用）。方針判断（扱うか除外のまま置くか）を要る。実装はしていない。
