# 2026-11-09 — 設定値の集約範囲を確定（名前の既定値のみ単一出力元、cap は定数正典）

**Status**: 承認済み（ユーザー 2026-11-09 00:45 UTC「推奨を承認する」）。実装未着手（実装開始指示待ち）
**タスク記録**: `docs/ai/tasks/2026-11-09-settings-value-centralization.md`

## 決定（正典）

- 集約の範囲: **名前の既定値（`db`・`gallery_dir`・`cache_dir`・`acquisition_dir`）だけ**を単一出力元（`defaultConfig()`）へ寄せる。call site ごとの inline literal（`app/main.go:1108`、`912`、`1121`、`app/export.go:95`、`app/pdf.go:288`、`app/main.go:200-202`、`app/viewer.go:126,130`）と `galleryDir()` の `"Galleries"` 直書き（`app/library.go:153-155`）を廃止し、cfg 経由へ統一。
- 件数・秒・MB の cap、floor、TTL、`schemaVersion`、DB pragmas、page 側定数、localStorage keys、表示ラベルは**定数正典としてコードに残す**（設定化しない）。設定化済みは `viewer.max_object_urls` 4000 のまま（例外）。
- `config.example.json` に加える key は `cache_dir`・`acquisition_dir` の 2 件のみ（名前の既定値＝設定化可能と宣言した値）。cap 系は example に key を置かない。
- Python 参照実装の自前既定表 `DEFAULT_LIMITS`（`src/scan.py:22`）を廃止し、同一 `config.example.json` を読む形へ。完成フォルダに同梱 data file として example が存在するため参照実装の単体実行性は保つ。

## 副決定点の確定

1. disk cache cap（4000）は **層（kind）ごとに 4000 件**。full 4000 + thumbs 4000 = 最大 8000 行。既存実装（`app/viewer.go:605-611`）どおり。
2. 捨て順は **作成順（`created_at ASC`）のまま**。last access 列は置かない（現在閲覧中の gallery の項目は捨てない規則を維持、`app/viewer.go:618`）。
3. galleryinfo disk cap は **件数 + 256 MiB を維持**（`app/viewer.go:141-142`）。full/thumbs の件数だけとは単位が混在することを記録済み。
4. page 側定数（concurrency・idle・ahead・cooldown・eager cover）は **page 正典として残す**。config へ移さない。

## 実装時に更新が必要（未実施）

- `app/main_test.go:1584-1585`（disk cap 8000 固定）→ 4000 へ re-anchor。
- `app/viewer.go:598` のコメント（既定 8000 件）→ 4000。
- `tests/test_scan.py:417`（`scan.DEFAULT_LIMITS` 参照）→ 廃止後の形へ更新。
- `app/main_test.go:1546` の一致テストの形を limits・viewer・dir 節へ拡張（example と `defaultConfig()` の自動照合）。
- `config.example.json` へ `cache_dir`・`acquisition_dir` 追加。
- docs pointer（`docs/ai/usage-memo.md:152`、`docs/ai/api-rate-limits.md`）は正典参照として維持。

## 関連決定

- 単位・値: `docs/ai/decisions/2026-11-09-disk-cache-cap-count-4000.md`（件数・4000）
- 削除禁止: `docs/ai/decisions/2026-11-09-completion-folders-no-delete-rule.md`（完成フォルダは prune 対象外。`config.example.json` に `cache_dir` を書く場合でも、永続先を `scratch/`・`output/` へ移さない条は維持）
