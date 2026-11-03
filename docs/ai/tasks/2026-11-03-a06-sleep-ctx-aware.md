# A-06: time.Sleep が ctx 非対応

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

UI prefetch の AbortSignal（fetch の ctx）は server 側 client の gate/backoff sleep には効かない。切断後でも gate 待機（既定 1000ms）と retry backoff（既定 2+4+6=12s）を完了まで待つため、cancel 直後に呼出が return するようにする（承認済み方針 `2026-11-03-remaining-item-recommendations-approved` A-06）。

## 調査結果 / 変更内容

有効。実コード上の該当箇所（タスク記録の行番号 139/182/201/215/233 は旧版参照でシフト、実態は以下）:
- gate sleep: `app/hitomi.go:293`（`gateForInterval`）
- retry backoff sleep: `app/hitomi.go:336,355,369,387`（`fetchRemoteWithGate`）

変更（app/hitomi.go の sleep/gate/backoff 関連のみ、本番 gate interval 1000ms 不変）:
- `sleepCtx(ctx, d)` helper を追加（承認方針の `select { case <-ctx.Done(): return ctx.Err(); case <-time.After(d): }`）
- `gateForInterval` を ctx 対応化（error を返し、待機中に cancel なら ctx.Err() を return）
- `gate()` は `context.Background()` 経由（callers なし・挙動不変）
- `getWithGate`・`fetchRemoteWithGate` で gate/backoff 中の cancel を即 return（`ctx.Err()`）へ。rate gate の interval 記録（`*last` の advance）と recordMetric の gateWait/retrySleep 記録意味は維持
- retry 継続は cancel 後はしない（sleepCtx が ctx.Err() を返して loop を抜ける）

テスト追加（app/hitomi_test.go）:
- `TestRateGateReturnsImmediatelyOnContextCancellation`: mocked fetch・本番 interval 1000ms で、2 呼出目の gate 待機中に cancel 済 ctx → elapsed <250ms で return、fetch 呼出なし、`errors.Is(err, context.Canceled)`
- `TestRetryBackoffReturnsImmediatelyOnContextCancellation`: httptest 500 応答・backoff 5000ms（長めに固定し「待機完了」と区別可能）、1 呼出目の処理中に cancel → elapsed <1s で return、server への request は 1 回のみ（retry 継続なし）、`errors.Is(err, context.Canceled)`

## Verification

- `go vet .`（app）→ clean（2026-11-03）
- 第1回 `go test -count=1 -timeout 300s .`（app）→ FAIL 2件。原因分析: (1) gate test の assertion が cancel 前の正常呼出を fetch call として数えていた（`calls != 0` → `calls != beforeCancel` へ修正）、(2) backoff test で `c.http` 未設定（nil panic）→ `srv.Client()` を設定。いずれも新テスト側の欠陥、実装コードではない。
- 修正後 `go test -count=1 -timeout 300s .`（app）→ `ok hitomi-hub/app 66.944s`（合格）
- 新テストのみ `-run "TestRateGateReturnsImmediatelyOnContextCancellation|TestRetryBackoffReturnsImmediatelyOnContextCancellation" -v` → 両 PASS・各 0.00s・合計 0.165s（1000ms gate / 5000ms backoff に対し即 return の証拠）
- canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 成功
- `python -m pytest tests/ -v`（ルート）→ 40 passed in 8.00s（`test_canonical_build_freshness.py` 含む）
- UI 未変更のため server 実機確認は該当なし（項目手順上 UI 関連ではない）

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end で実施）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（該当なし — plan 表の A-06 行は coordinator 側で管理）
