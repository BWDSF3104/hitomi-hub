# A-08 TN-only thumbProxy

Date: 2026-11-03

**Supersedes**:

- `2026-11-01-operation-priority-placeholder.md` の `thumbProxy()` order にある full cache first と、remote TN failure 時の full fallback を含む部分。
- `2026-11-01-hitomi-tn-variant-render.md` の `/api/thumb/`: TN first -> local file -> full image proxy。

**Decision**:

- `/api/thumb/` は search-card cover 用の TN-only proxy である。
- `thumbProxy()` の合格順序は TN variant cache hit、remote TN、TN failure のみ。
- `thumbProxy()` は full cache path、local full gallery file、remote full、`imageProxy()` へ回らない。
- TN failure は `x-cache: no_tn`、HTTP 502、`cache-control: no-store` で返す。
- `tn`、`tn-cache`、`no_tn` は full actual pixels として数えない。UI display source 判定は `no_tn` を display source から排除する。

## Reason

TN と full の分離は正典。TN failure を full 取得へ回すと TN 側の失敗が隠れ、full image guard の x-cache 意味と actual pixels 判定が混線する。cover の失敗は cover 失敗として見える必要がある。

## Scope

- search-card cover / `/api/thumb/` の server path 変更。
- UI display source 判定と prefetch counts で `no_tn` を TN-related status として扱う。
- reader display、download、full image proxy の取得順序は変更しない。
