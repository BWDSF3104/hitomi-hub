# Backlog

完了項目は削除（実装・検証は `tasks/` と `decisions/` に記録）。

## P2

- #7 デバイス間同期 — 不要（ユーザー判断 2026-11-01）
- #8 AVIF の pdf embed（pure-Go decoder が無い。webp・png・jpeg は decoder あり）
- #9 `bookmark_queries.notify` の用途（現行実装では未使用）

- #13 Tags display area clamp: search result tags display area should be limited to thumbnail height. Recommended implementation: `--search-card-height: 130px`, `.tags { max-height: var(--search-card-height); overflow: hidden; overflow-wrap: anywhere; line-height: 1.25; display: -webkit-box; -webkit-line-clamp: 9; }`, `title` full tags. Do not treat status text as actual display pass.

- #14 Viewer page switch display/status separation: recommended implementation. Keep `status` as backend/source truth and `displayPass` as DOM actual pixel truth. For local full entries, prefer direct `/files/...` img src. `sourceStatus` is from `entry.local` / `entry.local_formats`; `displayPass` is from `img.onload` and `naturalWidth > 0`. Direct `<img>` cannot read `x-cache`. Recommended status line: `actual pixels X/Y | local direct L/Y | objectURL cache C/Y | no full N/Y | TN display fallback disabled`. Background prefetch must not block viewer actual display. Stale status writes must be ignored by current `showGeneration`, `openGeneration`, `renderGeneration`, and `backgroundGalleryId`. Completed backlog `#12` was resolved by strip group controls and removed.

