# P3 #10 検索速度改善の検証

## 現在の実装状況

- elapsed time、metrics、profiling code is absent.
- `time.Now()` is used only in `nowISO()`, `gateForInterval()`, and `vt.go`.
- Search path: `SearchHTML()` calls `termIDs()` for each term. Colon terms call `SearchNozomiParts()` without Range, so full nozomi fetch is used. Plain terms call `IndexIDs()` through `indexVersion()` and `indexNode()`.
- `Client.versions` is an instance map cache only, no disk persistence.
- `Viewer.infoCache` is process-local.
- Viewer search writes `recordDiscovered` for each result, max 25 rows. DB write is serialized by `SetMaxOpenConns(1)`.
- Card metadata is fetched one-by-one by `loadSearchCards()`, no parallelism.
- `x-cache` header and `prefetch-status` are existing cache-hit visibility mechanisms.

## 実現可能性

- Measurement is possible.
- Gate wait time must be separated from network time, otherwise numbers are mixed with intentional politeness waits.
- Existing mock `Client.fetch` tests can validate measurement shape without external API calls.

## 制限事項

- External API calls in tests are allowed for `hitomi-hub` by decision `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`.
- Existing mock tests remain useful, but live API measurement tests are allowed when rate limits and API policy are respected.
- hitomi rate gate `min_interval_ms` applies to nozomi/index/gg calls as project policy, not as a server-side constraint. Values follow the canonical single source `defaultHitomiConfig()` in `app/hitomi.go` (not restated here). 2026-11-05 measurement showed no 429 up to `100 ms` with 12 parallel and no rate headers (`docs/ai/tasks/2026-11-05-assess-20261105-1010-rate-limit-verification.md`); the value change was approved by decision `docs/ai/decisions/2026-11-05-rate-values-and-concurrency-change-approved.md`. Lowering below the canonical floor guard (`minHitomiRateFloorMS` in `app/main.go` / `loadConfig`) is rejected as a config error.
- Multi-term search may make multiple gate-passed calls.
- Index search may make depth-based calls.
- Live CDN/window and retry backoff make wall-clock measurements unstable.
- CLI has no persistent cache across runs, so cold and warm results differ.
- Measurement scripts must be written under `docs/ai/tmp/`.

## 推奨実装案

- Add a measurement layer to `Client`.
  - Record `calls`, `gate_wait`, `network`, `decode`, and `retry_sleep`.
  - Classify URLs by kind: version, index, data, nozomi, gg, gallery, image.
- Add phase timing to `search()`.
  - `SearchHTML()` time
  - `recordDiscovered` loop time
  - JSON encode time
- UI should measure `/api/search` and each `/api/search-card/{id}` using `performance.now()`.
- Keep `search-status` semantics unchanged. Add `#perf-status` only.
- Compare cache-less root and cache-warm root for the same query.
- Confirm `x-cache` distribution does not cause network re-fetch on cache hit.

## その他実装案

- Wrap `handler()` with middleware and return `x-response-time`.
- Add CLI `-profile` flag and attach timings to JSON output.

## 検証方法

- Use mock `Client.fetch`, no external API calls.
- `go test .` and `go vet .` in `app`.
- Screen verify with cache-less runtime root.
- Record command, result, and elapsed time in task Verification.

## 採用決定

Recommended implementation is adopted. Measurement layer, phase timing, `performance.now()`, cache-less/cache-warm comparison, and mock-based tests are the implementation target.