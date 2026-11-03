# C-08 no_full short-TTL negative cache

Date: 2026-11-03

**Related decisions**:

- `2026-11-01-fast-nonblocking-viewer-policy.md`
- `2026-11-01-ggset-generation-and-drift.md`
- `2026-11-03-a08-tn-only-thumb-proxy.md`

**Decision**:

- `/api/image/` の storage 404 による `no_full` だけを、memory-only short-TTL negative cache として保持する。
- negative cache key は `hash + format + gg`。
- TTL は `5 * time.Minute`。gg rotation の 1 時間より十分短い。
- disk、full cache、persistent cache に `no_full` を保存しない。
- network error, timeout, parse error は negative cache しない。
- manual retry、`force=1` request、`EnsureGG(force=true)` による actual gg change は negative cache を bypass する。
- response の status 404 と `x-cache: no_full` の意味を変えない。UI/DB の full actual pixels 判定を変えない。

## Reason

恒常的に shard 窓外の gallery では、display/prefetch ごとに remote 照会を続けると gate を無駄消費し、表示可能な画像の取得を圧迫する。short-TTL memory-only negative cache は、恒常的な `no_full` の再照会を減らしつつ、gg drift による一時的な 404 を長期固定しない。

## Non-goals

- `no_full` を永続化しない。
- `no_full` を full actual pixels として数えない。
- manual retry の意味を「cache hit の返答」へ変えない。
- display path と prefetch path の x-cache semantics を変えない。
