# viewer の hitomi TN variant 準拠表示（work page full-entry）

**Status**: 進行中
**Date**: 2026-11-01
**Next Action**: commit・push はユーザー承認待ち（実装・テスト・画面確認完了）

## 意図 / 背景

hitomi の work page（作品ページ）は local file だけを見せる実装ではなく、`galleryinfo.files[]` の各 entry を TN thumbnail で表示する。この方針への最終理想形は「元サイトの機能拡張版」なので、viewer の reader strip で cover/local だけを描画する暫定回避を廃止し、hitomi の TN variant に合わせる。

## 調査結果 / 変更内容

実装計画:
1. hitomi の TN variant path を正典化（`bigtn` / `smalltn` / `smallsmalltn` / `smallbigtn`）
2. viewer の `/api/thumb/` を TN-first、local/full fallback に変更
3. UI strip を hitomi work page と同じ full entry 表示に変更し、`local` は app 機能の marker として残す
4. tests、build、画面確認、記録

変更:
- `app/hitomi.go`: `TNVariantSuffix()` / `TNBase()` / `ThumbnailURLVariant()` を追加。`ThumbnailURL()` は既定 `bigtn` variant。TN path は `<sub>tn.<domain>/<format><variant>/<last>/<prev2>/<hash>.<format>`。hitomi TN に格納されるのは webp / avif ののみで、`galleryinfo.files[].name` の ext は TN path に使わない。
- `app/viewer.go`:
  - `/api/thumb/<hash>` は `variant` query を受け、まず TN を fetch。TN が無い場合は local file、さらに full image fallback。
  - `/api/image/<hash>` は local full > remote full > TN bigtn fallback。remote full が CDN 時間窓で失敗しても TN で view を維持。
  - `/api/gallery/<id>` の `files[].local` は app 機能の marker として残す。
  - search result cover は hitomi galleryblock と同じ `variant=bigtn` path を返す。
- `app/ui.html`:
  - search card は cover thumbnail（`bigtn`）だけ。
  - reader strip は `files[]` の全 entry を hitomi work page variant で描画。`devicePixelRatio === 1` は `smallsmalltn`、それ以外 `smalltn`。
  - reader strip は hitomi の work page group と同じ 50 entry unit で描画。
  - `local` entry は filter ではなく `title='local'`、`data-local`、alt marker として app 機能で表示。
  - main image load failure は note に表示。

## Verification

- `go vet ./...` → clean
- `go test -count=1 ./...` → ok（`hitomi-hub/app`）
  - `TestTNVariantPathsMatchHitomiWorkPage`: `webpbigtn` / `webpsmalltn` / `webpsmallsmalltn` / `smallbigtn` path と `name` ext が TN path に混入しないこと。
  - `TestViewerThumbProxyUsesSmalltnVariantWhenRequested`: UI strip の variant path を server が hitomi TN path で fetch。
  - `TestViewerThumbProxyTNFirstWhenGalleryQueryAndLocalExists`: local file があっても hitomi work page の TN-first を維持。
  - `TestViewerThumbProxyFallsBackToLocalWhenTNMissingAndLocalExists`: TN missing 時 local fallback。
  - `TestViewerImageProxyFallsBackToTnBigtnWhenStorageMissing`: full image CDN failure の TN fallback。
  - `TestEmbeddedUIThumbUsesHitomiTNVariants`: UI filter 廃止、DPR variant、group 50、marker 維持。
- `python -m pytest tests/ -q` → 16 passed
- build completion exe → `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe`

Screen verification（2026-11-01）:
- viewer server: `http://127.0.0.1:8731/#/g/2648601/page/1`
- gallery: `Kumo Hebi no Ecchi Hon`・27 files・local 0
- reader strip count `27`、loaded `27`、broken `0`、errors `0`。playwright DPR `2` で hitomi work page の `smalltn` variant を使用。
- sample strip src example: `/api/thumb/30c5ca9585628457f43bdbd7ea6ffbe62a2ff4ce6924a2e1972368d08d99a022?format=webp&gallery=2648601&variant=smalltn` → `naturalWidth 300`。
- main image load ok（naturalWidth `640`）。full shard current window では TN bigtn fallback で表示。
- Screenshot: `scratch\hitomi-usage\screen_hitomi_tn_all_strip.png`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
