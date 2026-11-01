# Backlog

完了項目は削除（実装・検証は `tasks/` と `decisions/` に記録）。

## P2

- #7 デバイス間同期 — 不要（ユーザー判断 2026-11-01）
- #8 AVIF の pdf embed（pure-Go decoder が無い。webp・png・jpeg は decoder あり）
- #9 `bookmark_queries.notify` の用途（現行実装では未使用）

- #12 Viewer thumbnail strip currently renders only the current 50-item group, not all entries; `stripGroupPage()` selects group by current page.

- #13 Tags display area clamp: search result tags display area should be limited to thumbnail height. Planned implementation: `--search-card-height: 130px`, `.tags` `max-height: var(--search-card-height)`, `-webkit-line-clamp`, `title` full tags. Do not treat status text as actual display pass.

