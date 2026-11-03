# C-10: go test が 35 秒前後かかる

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（情報）。実測（2026-11-03, `app` で `go vet .` clean）: `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 35.352s`。原因は client gate（`MinIntervalMS` 1000ms）を mock fetch の sleep で使う test が複数あること。対応: gate wait を test で短縮（`gateForInterval` を inject 可能にする）または test ごとに interval を 0/1ms に設定（正典 gate 意味は維持）。合格条件: test 時間短縮、gate semantics tests は維持。推定規模: 小（テスト変更は実装段階）。

実施（2026-11-03 実施段階・方針確定）。baseline 実測（修正前）: `app` で `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 66.946s`。内訳（`-v` 実測）: 1s 以上の test は searchindex/searchhtml/viewer/check/hitomi の mock fetch 多呼出 test（1呼出あたり gate sleep 約1s。例: `TestSearchCacheEvictsLRUEntriesWhenOverEntryLimit` 4.00s = 呼出4回）。elapsed 判定を持つ gate semantics test は hitomi_test.go の 2 件のみ（cancel 即 return 証明。いずれも fast）。

確定方針（承認済み 2026-11-03・`docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md` C-10）: 本番既定（`defaultHitomiConfig()` MinIntervalMS 1000ms）は不変。`gateForInterval` を inject 可能にするテスト専用口を実装:
- `Client` へ `testGateIntervalMS int` 追加（0 = 非 inject・cfg 値を使用。テスト専用）
- `gateForInterval` 冒頭で `testGateIntervalMS > 0` のとき interval を inject 値へ置換。gate semantics（interval 記録・sleep・ctx cancel・recordMetric）は維持
- 対象: check_test.go・searchhtml_test.go・searchindex_test.go・hitomi_test.go（非 semantics test）・viewer_test.go（`newTestViewer` helper）で inject 1ms。gate semantics test（`TestRateGatePerRequest`・`TestRateGateReturnsImmediatelyOnContextCancellation`・`TestRetryBackoffReturnsImmediatelyOnContextCancellation`・`TestImageGateUsesLowerImageIntervalSeparately`・GG force/semantics・live test）は変更しない

## Verification

実測（2026-11-03・修正後。UI 未変更のため server 実機確認は該当规则上不要）:
- `app` で `go vet .` clean
- `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 5.094s`（修正前 baseline 66.946s。短縮効果: 約 61.85s）
- gate semantics tests 維持（変更なし・PASS）: `TestRateGatePerRequest`・`TestRateGateReturnsImmediatelyOnContextCancellation`（1000ms 既定維持の elapsed 判定）・`TestRetryBackoffReturnsImmediatelyOnContextCancellation`・`TestImageGateUsesLowerImageIntervalSeparately`・GG force/TTL/singleflight 系。1s 以上の test は baseline の 27 件から 0 件
- ルートで `python -m pytest tests/ -v` → 40 passed（`test_canonical_build_freshness.py` 含む）
- canonical exe 再 build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）→ freshness test 通過
- 本番既定不変確認: `defaultHitomiConfig()` MinIntervalMS 1000（`main_test.go` の既定 1000 判定 PASS）

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end で完了）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）→ 該当なし
- [ ] `backlog.md` の完了項目を削除（該当する場合）→ 該当なし（assess-20261103-1220 は実行計画側で管理）
