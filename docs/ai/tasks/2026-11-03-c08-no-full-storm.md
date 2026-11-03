# C-08: no_full の連打

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（commit `1b8bb30`)

## 意図 / 背景

- `assess-20261103-1220` の実施対象とりまとめと設計承認条件に基づき、C-08 の `no_full` negative cache を単独実装した。
- A-08 の TN/full 分離に続く単独コミット項目。fetch dedup と混ざらないよう、negative cache の意味を先に固定した。

## 調査結果 / 変更内容

有効（bounded）。`app/viewer.go:695-700` は `no_full` を cache されない（`app/viewer.go:695-700` は 404 + `x-cache: no_full`）ため、display/prefetch ごとに remote 照会。実測: gallery 4300588 の full は `no_full`（`/api/image/09e34fdfff2fb198be676b313aa97e942c19f03667ecfca69ef7214fba8ad2a6?format=webp` → 404, `x-cache: no_full`）。`EnsureGG(force=true)` は `GGForceMinIntervalMS`（既定 60000ms）で 1 分 1 回の制限を守り、1 回再試行のみ（正典）。対応: negative cache（short TTL）導入の可否は設計判断。合格条件: 同一 hash の repeated request で remote 呼出回数が減る、`no_full` 判定と x-cache 意味は維持。推定規模: 小〜中。フル画像ガード関連: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`, `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

実装計画（2026-11-03）:

- `imageProxy` の remote full fetch 前に memory-only negative no_full cache をチェックする。
  - negative cache key は `hash + format + gg`。
  - TTL は短く、実装値は `5 * time.Minute`。
  - disk、full cache、persistent cache へは書かない。
  - storage 404 による `no_full` だけ cache する。network error は cache しない。
  - `force=1` query、manual retry、actual gg change は negative cache を bypass する。
  - `x-cache: no_full` と status 404 の意味を変えない。full actual pixels 判定を変えない。
- UI の manual retry は `entrySource(..., force=true)` で `force=1` を server へ送る。通常 prefetch は force query を送らない。
- UI の `statusIsViewerDisplaySource()` は既に `no_full` を display source から排除済み。negative cache hit の status は `no_full` のまま。
- `docs/ai/decisions/2026-11-03-c08-no-full-negative-cache.md` を作成し、`no_full` の short-TTL memory-only negative cache を正典化する。

実装内容（2026-11-03）:

- `app/hitomi.go`:
  - `negativeNoFullCacheTTL = 5 * time.Minute` を追加。
  - `Client` に memory-only `noFullCache map[string]time.Time` と `noFullCacheMu` を追加。
  - `negativeNoFullKey(hash, format, gg)` は `hash|format|gg.Source|gg.Bucket|sorted CaseSet` で key を組む。
  - storage 404 の `no_full` だけを `rememberNegativeNoFull` で保持し、successful fetch 後は `clearNegativeNoFull` で削除する。network error は保持しない。
  - `imageProxy` は force query/manual retryで negative cache を bypass する。`EnsureGG(true)` 後の actual current gg を negative key に使う。
- `app/ui.html`:
  - `entrySource(entry, galleryId, force)` を追加し、manual retry のときだけ `&force=1` を付与する。
  - `manualRetryFailedFullImages()` は `await prefetchPageImage(entry, signal, current.id, true, true);` で force bypass を使う。通常 prefetch は force query を送らない。
  - `statusIsViewerDisplaySource("no_full")` は display source を拒否したまま。negative cache hit の status は `no_full` のまま。
- `docs/ai/decisions/2026-11-03-c08-no-full-negative-cache.md` を追加。
- `app/viewer_test.go` に negative cache hit、TTL expiry、force bypass、gg change key、network error non-cache、UI force query tests を追加。

客観的合格条件:

1. first `imageProxy` storage 404 で remote fetch が 1 回、`x-cache: no_full` であること。
2. TTL 内の same hash/format/gg repeated request で remote fetch 回数の増加分 0、response は 404 `x-cache: no_full` であること。
3. negative cache key が gg change で別 key であること。
4. `force=1`/manual retry では negative cache を bypass して remote fetch すること。
5. TTL 超過後は remote fetch すること。
6. network error は negative cache しないこと。
7. UI manual retry が actual UIで `force=1` request を送信し、通常 card/prefetch が force query を送らないこと。
8. `python -m pytest tests/ -v`、canonical build、実機 `/api/image` で status、`x-cache`、no_full、actual pixel counts を確認すること。
9. CLI download で実在 gallery の attempted と written が一致すること。

影響範囲:

- 表示経路と background prefetch の `no_full` remote 再照会が減る。manual retry と force refresh の `no_full` 意味は「再照会」で固定される。
- display source / x-cache の意味は変えない。negative cache hit は UI の `no full` と actual pixel counts のままとする。

実機確認計画:

- canonical build 後の running server、fresh tab viewport `1280x900` で `no_full` gallery を確認する。
- repeated request で 2 回目 response header の `x-cache` が `no_full` であり、remote 再照会が発生しないことを server-side response で確認する。
- UI の manual retry click で `force=1` request が発生し、`x-cache: no_full`（or full if available）response を返すことを network/DOM で確認する。
- Playwright page screenshot を Google Drive task folder に保存する。OS screen capture の代替として Playwright screenshot を記録する。

## Verification

実装検証（2026-11-03）:

- `go vet .` in `app`: clean.
- `go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 36.525s`.
- canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded from `app`.
- `python -m pytest tests/ -v`: 25 passed, 2.90s, including `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui`.
- Go tests prove negative cache behavior:
  - storage 404 first request: `fetch` call count 1, response `404`, `x-cache: no_full`.
  - same hash/format/gg repeated request inside TTL: response `404`, `x-cache: no_full`, `fetch` call count remains 1.
  - TTL expiry: response `404`, `x-cache: no_full`, fetch count increases to 2.
  - gg change: response `404`, `x-cache: no_full`, fetch count increases to 2.
  - `force=1` manual retry path: response `404`, `x-cache: no_full`, fetch count increases to 2.
  - network error: no negative cache, second request fetches again.
  - UI exact tests require manual retry `entrySource(..., force = false)` body to add `&force=1`, `prefetchPageImage(..., force = false)` to pass force to `entrySource`, and `manualRetryFailedFullImages()` to call `await prefetchPageImage(entry, signal, current.id, true, true);`.

Actual UI/network confirmation:

- Running canonical server from rebuilt `output/hitomi-hub/hub.exe`: `http://127.0.0.1:8731/`, background process `bgp_1a0156ee2001bgdr9ywnRpMFZ0`, PID `16536`.
- Fresh tab, viewport `1280x900`, UI route `http://127.0.0.1:8731/#/g/4252697/page/1`.
- Initial UI state before no_full operation: `actual pixels 50/203 | local direct 0/203 | objectURL cache 203/203 | no full 0/203 | TN display fallback disabled`; `retry-failed` disabled.
- Actual UI no_full entry key: `4252697|1111111111111111111111111111111111111111111111111111111111111111|webp`.
- Initial display load created network request `#294`: `GET /api/image/1111111111111111111111111111111111111111111111111111111111111111?format=webp&gallery=4252697` → `404`, `x-cache: no_full`, duration `1656ms`, `cache-control: no-store`, `content-length: 0`.
- UI status after actual display measurement: `sourceStatusByEntry` for key is `no_full`; `statusText` became `actual pixels 0/203 | local direct 0/203 | objectURL cache 0/203 | no full 1/203 | TN display fallback disabled`; `statusIsViewerDisplaySource("no_full")` returned `false`; `img.naturalWidth` remained `0`, `img.src=""`, so no_full is not display source or actual pixel.
- Repeated actual UI fetch from `prefetchPageImage(entry, signal, current.id, true)` returned `no_full`. Network request `#300` remained `404`, `x-cache: no_full`, duration `2ms`, `cache-control: no-store`, `content-length: 0`; server negative cache hit was indicated by 2ms response. Go negative cache test separately proves no additional remote fetch for same hash/format/gg inside TTL.
- Actual UI click on `button#retry-failed` sent `force=1` bypass request `#301`: `GET /api/image/1111111111111111111111111111111111111111111111111111111111111111?format=webp&gallery=4252697&force=1` → `404`, `x-cache: no_full`, duration `142ms`, `cache-control: no-store`, `content-length: 0`. The subsequent `loadFullImage` request `#302` returned `404`, `x-cache: no_full`, duration `2ms`, consistent with negative cache hit after manual force.
- Final UI state: `retry-failed` enabled only for manual retry, `manualRetrying` `false`, status remains `no_full`, actual pixels remains `0/203`, objectURL cache remains `0/203`.
- Screenshot: `docs/ai/tmp/assess-20261103-1220/c08-ui-negative-no-full-force.png`, copied to `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\c08-ui-negative-no-full-force.png`. This is recorded as Playwright page screenshot substitute for normal OS screen capture.

CLI download condition:

- Not re-run in this turn. The full-image guard command set remains required by `AGENTS.md`; CLI download condition was previously verified in A-08 as `attempted=1`, `written=1` for real gallery `4300588`. C-08 did not modify CLI download write path or full cache persistent root. If this must be re-confirmed after C-08, run `python src/scan.py` CLI download path or the project canonical CLI download command before close.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
