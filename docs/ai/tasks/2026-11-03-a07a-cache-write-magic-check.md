# A-07a: cache 書き込み前の magic 検査がない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/viewer.go:320-376`（`writeCacheFileOnce`。記録の 315-371 は現ファイルの同一関数）は remote bytes を magic/format 検査なしで cache へ書く。scan 側の magic 検査は gallery ファイルに対してのみ（`scan_file`）。

実装方法（確定）: `writeCacheFileOnce` の冒頭（MkdirAll・temp 作成・attempt 計上より前）で `cachePath` の extension から format magic を照合し、不一致は error を返して cache しない（temp も作らない、attempt として数えない）。helper `matchesFormatMagic`: webp = `RIFF`(0-3)+`WEBP`(8-11)、avif = `ftyp`(4-7)+`avif` brand(8-11)。webp/avif 以外の format は本検査の範囲外（現行動作維持・照合なし）。呼び出し側 imageProxy の `_ =`（返り値 ignore）は変更しない → x-cache（miss）・no_full 判定・response body は意味不変。TN cache 書き込みは `thumbProxy` の `os.WriteFile`（`viewer.go:635`）で `writeImageCacheAtomically` を通らないため TN 規則へ影響なし。呼び出しは imageProxy の 2 箇所のみ（full cache）。

影響を受ける既存テスト（mocked fetch）: 記録の `TestViewerImageProxyCachesBytes` は HEAD `22c5953` に存在しない（`git grep` でタスク記録以外に該当なし）。mocked fetch の cache 書き込みテスト実体は `TestViewerImageProxyWritesPersistentCacheAndServesCacheHit` で、この実体を更新する。fixture が不正 magic（"RIFF" 4 bytes 等）の `TestImageProxyCacheMissWritesAtomically`・`TestImageProxyCacheConcurrentWriteSingleflight`・`TestImageProxyCacheSingleflightFailureDoesNotReportSuccess` は合格条件固定のため valid magic fixture へ更新。新規テスト `TestImageProxyCacheMagicCheckSkipsMismatchedBytes`（不正 webp/avif → cache ファイル増えない＋x-cache miss 意味維持、valid webp/avif → cache される）。

合格条件: 不正 bytes（magic 不一致）で cache ファイルが増えない、正しい webp/avif bytes は引き続き cache される。x-cache・no_full・TN 規則（full cache に TN 保存禁止・reader 表示源禁止・display fallback 禁止・full actual pixels 計上禁止）を弱めない。

読了（作業前チェック）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

## Verification

- app（変更後）: `go vet .` exit 0、`go test -count=1 -timeout 300s .` ok `37.686s`（経過 40.06s）。固定テスト個別実測（`-v -run`）6 件 PASS: `TestImageProxyCacheMagicCheckSkipsMismatchedBytes`（新・合格条件固定）/ `TestViewerImageProxyWritesPersistentCacheAndServesCacheHit`（mocked fetch・更新）/ `TestImageProxyCacheMissWritesAtomically` / `TestImageProxyCacheConcurrentWriteSingleflight` / `TestImageProxyCacheSingleflightFailureDoesNotReportSuccess` / `TestImageProxyCacheStateIsNotTN`（TN 規則回帰なし）。経過 `0.962s`。
- mock fetch magic 不一致実測（`TestImageProxyCacheMagicCheckSkipsMismatchedBytes`）: webp 不正 `not webp payload` → cache ファイル作成なし・attempt 0・rename 0、avif 不正 `aaaaftypheicbrandxxx`（ftyp+heic brand）→ 作成なし。正しい webp `RIFFabcdWEBPpayload`・正しい avif `aaaaftypavifbrand`（ftyp+avif brand）→ cache される（attempt 1・rename 1）。x-cache は全ケース `miss`（意味不変）。
- canonical build（修正込み再 build・exit 0・scratch dataDir・8731・製品 output/正典 cache 非汚染）:
  - CLI download 実在 gallery 4301695（scratch root）: fresh root 1 回目 `attempted=1 written=1 skipped=0`（`a07a-download3`）。`limit 3` fresh run1: 3 ファイル written（316356・291158・300986 bytes、run2 で `skipped=3` 一致確認）。経過 3.48s（run1 約 3.7s）。
  - viewer `/api/image/c05d79ed…37d8c63?format=webp`: req1 `200` `x-cache: miss` `300986 bytes`（remote fetch・実測 1.13s）、req2 `200` `x-cache: cache` `300986 bytes`。生成 cache ファイルの magic を `Format-Hex` で実測: `RIFF`(0-3)+`WEBP`(8-11) 正しい bytes が正しく cache される。
  - TN 回帰: `/api/thumb/…` live は 502（fetch 失敗・cache 書込みなし）。TN 規則（full cache 非保存・reader 表示源禁止・display fallback 禁止・full actual pixels 非計上）は `TestImageProxyCacheStateIsNotTN` PASS で固定（実機 mock 不可のため mock fetch 実測はテスト側）。server は実測後停止し開始時状態（未稼働）へ復元。
- pytest ルート: `python -m pytest tests/ -v` 33 passed `2.41s`（`tests/test_canonical_build_freshness.py` 含む）。UI 未変更のため exe 内容の ui.html は同一（freshness 合格、canonical exe は修正込みで再 build 済み）。

判断点（持ち帰り）: 記録の既存テスト名 `TestViewerImageProxyCachesBytes` は HEAD `22c5953` に存在せず（git grep 該当は本記録のみ）、実体の mocked fetch cache テスト `TestViewerImageProxyWritesPersistentCacheAndServesCacheHit` を更新対象として扱った。live server では remote が正しい magic bytes を返すため magic 不一致は mock fetch テストで実測。canonical exe の digest（`tools/VERSIONS.md`）はスコープ外のため未更新。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
