# Viewer reader strip が hitomi work page の TN variant を使っていなかった

- Date: 2026-11-01
- Status: resolved
- Project: hitomi-hub

## Problem

Viewer の reader strip は非 local entry を filter し、cover（`i=0`）と local file の image だけを描画していた。これは hitomi の work page 実装（`galleryinfo.files[]` の各 entry を TN small TN variant で描画する）と違い、remote entry を閲覧できない問題になっていた。

## Resolution

- `app/hitomi.go`: TN variant path（`bigtn` / `smalltn` / `smallsmalltn` / `smallbigtn`）を追加。
- `app/viewer.go`: `/api/thumb/` を TN-first、local/full fallback に変更。`variant` query を受け付ける。
- `app/ui.html`: reader strip で `files[]` の全 entry を hitomi work page variant（DPR 1 -> `smallsmalltn`、DPR > 1 -> `smalltn`）で描画。`local` は filter ではなく marker。
- Screen check gallery `2648601`: strip `27` entries, loaded `27`, broken `0`。
- Screenshot: `scratch/hitomi-usage/screen_hitomi_tn_all_strip.png`
