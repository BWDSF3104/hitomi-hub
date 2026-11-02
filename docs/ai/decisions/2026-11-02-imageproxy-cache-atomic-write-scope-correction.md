# imageProxy cache atomic write scope correction

Date: 2026-11-02
Supersedes: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope.md`

## Command correction

Verification commands must use the project canonical form:

- In `app`:
  ```sh
  go vet . && go test -count=1 -timeout 300s .
  ```
- Do not shorten to `./...`.
- Do not omit `-count=1`.
- Repo root has no `go.mod`.
- The commit checklist command `python -m pytest tests/ -v`, including canonical build freshness, is also a completion condition.
- Run `python -m pytest tests/ -v` after canonical build.

## Write sites

Both `imageProxy` full-image cache write sites must use the same atomic write helper:

- `app/viewer.go:557`
- `app/viewer.go:570`

Replacing only one site is not allowed.

## Temp file rule

Temp files must be created in the same directory as `cachePath`, inside `cacheRoot`.

- Do not use OS temp directory, `scratch/`, or `output/` for atomic cache write temp files.
- `os.CreateTemp(v.cacheRoot, ...)` is acceptable as an example because it creates the temp file inside `cacheRoot`.
- Cleanup is required for write failure, close failure, and rename failure.
- Tests must confirm no `.tmp` residue on failure, not only on success.

## Singleflight spec

Singleflight scope is **write only**, not fetch plus write.

- Same-hash concurrent requests may still duplicate remote fetch.
- Singleflight key is `cachePath`.
- Singleflight must not count duplicate actual write as more than one.
- Waiting callers must not report a failed first write as already written.
- `helper` returns success only when:
  - target already exists before the attempt, or
  - an actual attempt completed through successful rename.
- If an actual write, close, or rename fails, the helper result is error.

Test seam for counting actual duplicate writes:

- `cacheWriteAttemptCount` counts actual write attempts.
- `cacheRenameSuccessCount` counts actual rename successes.
- Existing-target skip is not counted as an actual attempt.
- Failure injection hooks should be available through test seams for write, close, and rename failures.
- Test must show:
  - two helper calls,
  - one actual write attempt,
  - one rename success,
  - no duplicate actual write,
  - no `.tmp` residue.

## Actual confirmation prerequisites

The actual server confirmation must use a hash that is both:

- absent from the product root cache at confirmation start, and
- inside the live shard window.

A hash already in cache will not produce first `x-cache: miss`. A hash outside the live shard window will produce `no_full`, so the procedure cannot be satisfied.

The procedure itself remains:

1. first request: `200`, `x-cache: miss`
2. restart
3. same hash second request: `200`, `x-cache: cache`

The product root, hash, format, cache path, command, result, and elapsed time must be recorded. `scratch/` must not be used as the persistent confirmation root.
