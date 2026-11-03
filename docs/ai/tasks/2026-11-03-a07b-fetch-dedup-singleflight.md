# A-07b: fetch の重複排除がない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了

## 意図 / 背景

`imageProxy` の remote full fetch に重複排除がなく、同一 `hash + format + gg` への同時リクエストで remote 呼出が複数回発生しうる。実装目標 `docs/ai/tasks/2026-11-03-assess-20261103-1220-implementation-targets.md` の A-07b 承認条件に従い、`imageProxy` remote full fetch 限定で singleflight を追加した。CLI download、TN、cache write singleflight は対象外。`x-cache` と display source の意味を変えず、単独 request の cancellation を leader fetch へ直結しない。

## 調査結果 / 変更内容

- `app/viewer.go` に `imageFetchCall` と `imageFetchGroup` を追加。remote full fetch 用の join 判定は `imageFetchKey(hash, format, gg)` が行う。
- `imageFetchKey` は `negativeNoFullKey` を再利用し、key に `hash + format + gg` の境界を含める。format/gg が違えば別 group。
- `fetchFullImageOnce` は同一 key の active call があれば joinし、leader fetch の結果を共有する。leader fetch は `context.WithoutCancel(ctx)` を使うため、 waiter の cancellation が remote fetch を cancel しない。waiter 自身の cancellation はその waiter の返却のみ影響する。
- `imageProxy` の remote full fetch 経路は cache write singleflight を維持しつつ、fetch 前に `fetchFullImageOnce` を使う。local first、persistent full cache、negative cache、format/gg、TN path は変更しない。
- `imageProxy` の `x-cache` は remote fetch 成功時 `miss`、disk cache hit 時 `cache`、negative/no storage 時 `no_full` の既存意味を維持。TN path は `tn` / `tn-cache` / `no_tn` を維持。
- deterministic join を保証するため `imageFetchJoinObserver` を viewer に追加し、テストでの concurrent join 確認に使用。production 動作では nil のため影響しない。
- `app/viewer_test.go` に concurrent singleflight remote call count 1、format/gg key boundary、group cleanup、canceled waiter 独立、x-cache `miss` 維持を追加。既存 UI `entrySource` force tests は C-08 完了状態のまま維持。

## Verification

- `go vet .` in `app`: clean。
- `go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 36.672s`。
- Canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded。
- `python -m pytest tests/ -v`: 25 passed, including `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui`。
- CLI download real gallery: `.\hub.exe download --id 4300588 --limit 1` returned `attempted=1`, `written=1`, `scan_summary.not_evaluated=1`。
- Real server UI confirmation from rebuilt canonical exe, `http://127.0.0.1:8731/`, fresh tab, viewport `1280x900`:
  - Actual click on first search card to `#/g/4300588/page/1`.
  - Initial display: `actual pixels 8/8 | local direct 1/8 | objectURL cache 7/8 | no full 0/8 | TN display fallback disabled`。
  - Served UI network singleflight proof:
    - URL: `http://127.0.0.1:8731/api/image/b17f157ac25f3f066f9d0b3707a98040defc90cf8a5847da550b9a0240fb3c53?format=webp`
    - Concurrent browser network requests `#21` and `#22` both returned `200`, `x-cache: miss`, same content bytes `153458`, duration `882ms` each. This confirms served UI/network semantics; Go test proves remote fetch call count 1 for same key.
  - Screenshot: `docs/ai/tmp/assess-20261103-1220/a07b-ui-network-dedup-miss.png`
  - Google Drive copy: `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\a07b-ui-network-dedup-miss.png`

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
