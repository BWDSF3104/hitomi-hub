# noFullCache stale-only bounded memory limit

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 実施開始: stale-only bounded memory limit 実装・テスト固定・単独 commit

## 意図 / 背景

- `C-05` の実測で `noFullCache` の stale key（古くて今後使われない項目）が残留することを確認済み（K=2000 stale key insert → access 10 件のみ delete・残留 1990）。
- `noFullCache` は `C-08` の negative cache（失敗状態の短期記憶）であり、TTL（有効時間） 5 分。
- 単なる active LRU（最近使用順で有効時間内項目も淘汰する方式）にすると、TTL 内 remote storage recheck（外部 storage 再確認）を避ける保証が変わる可能性がある。
- ユーザー承認方針: `noFullCache` は stale key（古くて今後使われない項目）だけを掃除する bounded memory limit（項目数上限）にする。
- active entry（有効時間内の項目）は protected（保護）扱いにする。
- hard cap（厳密な項目数上限）で active entry（有効時間内項目）まで消すなら、`C-08` の意味変更として別途承認が必要。

## 調査結果 / 変更内容

作業前チェック（フル画像関連条・読了確認）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`、承認方針 `docs/ai/decisions/2026-11-04-remaining-residual-approval-split.md` を読了。既存 C-08 意味確認: `canUseNegativeNoFull` は TTL hit で true・TTL 超過 hit で key delete・false（hit 時 delete 意味）。`clearNegativeNoFull`（force/manual retry bypass・成功時 clear）意味。`imageProxy` は 404 時だけ `rememberNegativeNoFull`（network error は入れない）。TN・full cache・UI full 判定は no_full を actual pixels として数えない（既存テスト `TestViewerImageProxyReturnsNoFullWhenStorage404AndDoesNotPersistTN` が固定）。

変更（`app/hitomi.go`）:
- `defaultNoFullCacheMaxEntries = 1024`（entries-only cap）追加。
- `Client.noFullCacheMaxEntries`（テスト inject 口・0 で既定）追加。
- `cleanStaleNoFullCacheLocked()` 追加（`noFullCacheMu` 保持下でのみ呼出）。cap 超過時、TTL 超過済み stale entry だけを delete。active entry（TTL 内）は protected で eviction しない。
- `rememberNegativeNoFull` の insert 後に `cleanStaleNoFullCacheLocked()` を呼ぶ（cap 超過時だけ掃除）。
- `canUseNegativeNoFull` の TTL hit 時 delete 意味・`clearNegativeNoFull` の force/manual retry bypass 意味は変更なし。`imageProxy`、TN rules、full cache、UI full 判定、API rate policy、gate、singleflight、gg、retry-once は非触。

意味: hard cap で active entry まで消す方式（C-08 意味変更）は未承認のまま。stale-only cleaning は hit されなくなった項目の掃除なので C-08 の TTL 内 hit 保証を変えない（テストで固定）。

追加テスト（`app/hitomi_test.go`）: `TestNoFullCacheCleansExpiredEntriesWhenOverEntryLimit`、`TestNoFullCacheDoesNotEvictActiveEntriesWhenOverEntryLimit`（active protected, no active eviction 固定）。

## Verification

- `app` で `go vet .` → clean（無出力）。
- `app` で `go test -count=1 -timeout 300s .` → ok 19.206s（flaky LRU tie 発火なし）。
- 追加テスト単独実行: `TestNoFullCacheCleansExpiredEntriesWhenOverEntryLimit` PASS（0.18s）、`TestNoFullCacheDoesNotEvictActiveEntriesWhenOverEntryLimit` PASS（0.00s）（単独実行 suite 計 0.353s）。
- ルートで `python -m pytest tests/ -v` → 51 passed 8.97s（`tests/test_canonical_build_freshness.py` 含む）。
- canonical exe rebuild 成功（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`・0.843s）。server 起動・UI 実機確認は本項目不要（テストで固定できる内容のみ）。external API 呼出しなし（mock・テスト内 map 操作のみ）。
- 合格条件: stale key が cap 超過で掃除される（テスト1）、active entry は cap 超過でも消えない（テスト2）、`canUseNegativeNoFull` TTL hit 意味不変（テスト1・既存 `TestViewerImageProxyNegativeNoFullCacheReducesRepeatStorageFetches`）、network error / parse error / force / manual retry bypass 意味不変（既存テスト `TestViewerImageProxyDoesNotCacheNetworkErrorAsNegativeNoFull`・`TestViewerImageProxyForceBypassesNegativeNoFullCache`・`TestViewerImageProxyNegativeNoFullKeyChangesWhenGGChanges`）、no_full を full cache / full actual pixels として保存・判定していない（既存 `TestViewerImageProxyReturnsNoFullWhenStorage404AndDoesNotPersistTN`）。全通過。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
