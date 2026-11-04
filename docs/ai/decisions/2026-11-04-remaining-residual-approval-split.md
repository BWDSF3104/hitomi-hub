# 残項目の承認分割（2026-11-04・ユーザー承認）

**Status**: 判断確定  
**Date**: 2026-11-04  
**Backlog**: assess-20261103-1220

## 承認内容

1. **その他の残項目は実施承認として記録**
   - canonical `hitomi.db` の `ensureSchema` 実確認。
     - `user_version=4` → `5`。
     - `scan_cache` の `size` / `mtime` 列を確認。
     - 既存行を削除しない。
     - canonical server 起動時に自然に揃う想定。
   - B-06 の optional live 200 baseline確認。
     - code 変更なし。
     - external API は 1 回のみ。
     - 実施時は API rate policy と `scratch/virustotal-api.key` の記録禁止を守る。

2. **`noFullCache` bounded 化は追加検討**
   - 即時実装は承認しない。
   - 単なる memory cap として見るか、C-08 negative cache の意味変更として見るかを確定してから判断する。
   - 実装する場合は、C-08 の「TTL 内 negative cache hit で remote fetch を避ける」「manual retry / force bypass」「no_full を full actual pixels として数えない」のどれを変えるかを明示する。

## 意味

- 他の項目は実施可能。
- `noFullCache` だけ、実装前に方針確認が必要。
