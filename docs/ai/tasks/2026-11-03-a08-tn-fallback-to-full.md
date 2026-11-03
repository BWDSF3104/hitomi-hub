# A-08: thumbProxy が TN 失敗時に full へフォールバック

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了（commit `683ff47`)

## 意図 / 背景

- `assess-20261103-1220` の実施対象とりまとめに基づき、A-08 を TN/full 経路分離として実装する。
- A-08 は単独コミットとし、TN-only status と full actual pixels 判定の混線をテストで固定する。

## 調査結果 / 変更内容

有効（ガード違反）。`app/viewer.go:620-623` の `thumbProxy` は TN リクエストでも full cache bytes を先に返す。TN fetch 失敗時は `serveLocalImage` → `imageProxy`（full）へフォールバック（`app/viewer.go:639-642`）。TN バイトを full cache に保存しない・TN display fallback 禁止の正典と衝突。対応: full cache 参照順を TN cache/remote TN に限定し、TN 失敗は `no_tn`（502）で返す。実装は方針変更の提案として承認後に実施。合格条件: TN リクエストで full cache が参照されないこと（mocked fetch + x-cache 検証）。フル画像ガード関連: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`, `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`, `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。推定規模: 中。

実装計画（2026-11-03）:

- `app/viewer.go` の `thumbProxy` を TN-only 経路にする。
  - full cache path を参照しない。
  - local full gallery file、remote full、`imageProxy` へフォールバックしない。
  - TN cache hit は `x-cache: tn-cache`、remote TN success は `x-cache: tn`。
  - TN fetch failure は `x-cache: no_tn`、HTTP 502、no-store。
- UI の display source 判定と prefetch count で `no_tn` を TN-related status として扱い、full actual pixels として数えない。
- `docs/ai/decisions/2026-11-03-a08-tn-only-thumb-proxy.md` を作成し、古い TN order / TN failure full fallback 記述を supersede する。

客観的合格条件:

1. mocked fetch test で TN request が full cache path を参照しないこと。
2. TN failure test で response code 502、`x-cache: no_tn`、full cache path が参照されないこと。
3. TN cache hit / remote TN success の `x-cache` が `tn-cache` / `tn` であること。
4. UI の `statusIsViewerDisplaySource()` が `tn`、`tn-cache`、`no_tn`、`no_full`、`error` を display source から排除すること。
5. canonical build と実機 `/api/thumb` で TN response が full actual pixels として扱われないこと。
6. `go vet .`、`go test -count=1 -timeout 300s .`、`python -m pytest tests/ -v`、canonical build、実機画面操作確認が通ること。

影響範囲:

- search-card cover の TN failure では full bytes を表示できない。reader の full display には影響しない。
- TN cache hit で速くなるが、TN failure の search-card cover は placeholder / failure のままになる。

実機確認計画:

- canonical build 後の running server の fresh tab viewport `1280x900` で search-card cover 表示を確認する。
- TN hit response の `x-cache` を network response で確認する。
- actual UIで cover が TN source から表示されることを確認し、Playwright page screenshot を Google Drive task folder に保存する。
- actual TN failure は外部 TN を意図的に失敗させるfixtureがないため、mock testで合格条件を満たし、実機 failure screenshot は取得できない場合は未確認として記録する。

## Verification

実施済み（2026-11-03）:

- `app/viewer.go` の `thumbProxy` を TN-only 経路へ変更: TN cache hit / remote TN だけ。full cache path、local full、`imageProxy` full fallback を排除。TN failure は `writeNoTN` で `502`、`x-cache: no_tn`、`cache-control: no-store`。
- `app/ui.html` の `statusIsViewerDisplaySource()` に `no_tn` を追加し、prefetch count の decrement/increment で `tn` と `no_tn` を TN-related status として扱う。`no_tn` を full actual pixels として数えない。
- `app/viewer_test.go` で TN-only path、TN cache hit、TN failure の `no_tn`、full/local fallback なし、UI display source の `no_tn` 排除を確認。

テスト:

- `go vet .` clean.
- `go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 35.604s`.
- canonical build rebuild: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` succeeded.
- `python -m pytest tests/ -v`: 25 passed, including `tests/test_canonical_build_freshness.py`.

実機確認:

- canonical server from rebuilt `output/hitomi-hub/hub.exe`, fresh tab, viewport `1280x900`.
- actual UI operation: `female:catgirl` search, search-card covers load through `/api/thumb/...`.
- first TN network response for `09e34fdfff2fb198be676b313aa97e942c19f03667ecfca69ef7214fba8ad2a6` gallery `4300588`: `200`, `x-cache: tn-cache`, `cache-control: private, max-age=604800`, `content-type: image/webp`. Search card cover screenshot: `docs/ai/tmp/assess-20261103-1220/a08-search-card-tn-only.png`, copied to `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\a08-search-card-tn-only.png`.
- served UI `statusIsViewerDisplaySource("no_tn")` returned `false`.
- actual `/api/thumb` TN failure for a valid random hash returned `502`, `x-cache: no_tn`, `cache-control: no-store`, `content-type: image/webp`; no full fallback was observed.
- actual `/api/image/<hash>?format=webp` for `09e34fdfff2fb198be676b313aa97e942c19f03667ecfca69ef7214fba8ad2a6` returned `200`, `x-cache: miss`, `content-type: image/webp`.
- CLI download real gallery `4300588 -limit 1`: `attempted=1`, `written=1`.

画面キャプチャの代替: strict criteria の「normal OS screen capture」は Playwright page screenshot で代替済み。TN failure の actual UI screen failure screenshot は、external TN を意図的に失敗させるfixtureがないため実機 screenshot として取得していない。合格は mocked test と actual `/api/thumb` TN failure response で確定した。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
