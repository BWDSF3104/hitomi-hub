# N-03: 稼働中に gg が TTL 更新されず 404 起点と no_full を招く

**実行固有ID**: assess-20261108-1332

**Status**: 進行中
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

稼働中に gg が TTL 更新されない問題の調査。`LoadGG` は起動時のみ（server では `app/viewer.go:258` の `warnGG(loadGG(v.client))`、CLI では `app/main.go:1432-1434` の `loadGG`）、`imageProxy`（`app/viewer.go:1066`）は 404 時のみ `EnsureGG(force=true)` を呼ぶ。bucket 約 1 時間回転（`ggCacheTTL = time.Hour`・`app/hitomi.go:58`）後、最初の取得が stale gg で 404 から始まり、さらに force throttle（`GGForceMinIntervalMS` 既定 60000・`app/hitomi.go:279`）内だと更新されず `sameGG` true → 有効な画像が `no_full`（negative cache TTL 5 分・`app/hitomi.go:22`）と記録される窓がある。対処案（`imageProxy` での `EnsureGG(ctx, false)`）の実測検証と、正典条件・cache hit 意味・fast/non-blocking viewer との衝突確認を行う。UI 項目ではないため画面確認は不要（viewer 表示源の正典を変更する案ではない）。

作業前チェック（フル画像・viewer 関連条）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を Glob で存在確認し読了（3 件とも実在）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `imageProxy`（`app/viewer.go:1066-1138`）の経路: local full → persistent full cache（`serveStoredImage`）→ `v.client.GG()`（`app/gg.go:204-211`。TTL 判定なし、cache 値を返すだけ）→ negative `no_full` 判定 → `fetchFullImageOnce` → 404 時だけ `EnsureGG(ctx, true)`（`app/viewer.go:1109`）→ `source == "live" && !sameGG(initialGG, gg)` のとき 1 回再試行（`app/viewer.go:1112-1131`）→ `no_full`。正典経路順序は維持されている。
- `EnsureGG`（`app/gg.go:120-202`）: force throttle は `GGForceMinIntervalMS`（既定 60000・`app/hitomi.go:279`）。throttle 内（`ggForceAt` から未満）で force のときは network なしで source を返す（`app/gg.go:134-139`）。非 force は `age < ggCacheTTL`（1 時間・`app/hitomi.go:58`）なら memory 判定のみ、超過時のみ data lane gate 経由で network（`app/hitomi.go:526-528`、singleflight `app/gg.go:147-161`）。
- `no_full` negative cache TTL = 5 分: `negativeNoFullCacheTTL = 5 * time.Minute`（`app/hitomi.go:22`）。5 分の根拠は正典 convention 定数（`blockCooldownDuration`・`searchResultCacheTTL` が同値引用、`app/hitomi.go:34,66`）。key は gg を含む（`negativeNoFullKey`・`app/hitomi.go:297`）。

### 有効性: 有効（実測で確定）

実測は mock（既存テスト流儀: `newTestViewer` + `v.client.fetch` inject + `testGateIntervalMS=1`・C-10。外部 API 呼出しなし）。スクラッチコピー `docs/ai/tmp/n03-scratch/`（本番 app は無変更）で実行。

- (a) 起動後 1 時間（TTL 超過）の最初の image 取得は 404 から始まる: 呼出し順 `gg.js#1(startup)→storage(stale windowA) 404 → gg.js#2(404-force) → storage(windowB) 200`。storage 呼出し 2・gg 呼出し 2・最終 status `miss`。200 になるのは 404 後の force 更新 + 1 回再試行が成功したときだけ（`TestN03CurrentFirstFetchAfterTTLStartsFrom404` PASS）。
- (b) throttle 窓: force 初回で throttle 発動後、60 秒内の 404 では `EnsureGG(true)` が network なし（gg 呼出し 2 のまま）、`sameGG` true（`app/gg.go:77`）で再試行なし → `no_full` 記録（記録 status 値 = `no_full`、`x-cache: no_full`）。同じ画像は live window（mock）で 200（= false negative）。negative cache が 5 分有効（4m59s で hit・5m01s で失効）、窓内リクエストは storage 取得なしで `no_full` を返す（`TestN03CurrentThrottleWindowPoisonsValidImageAsNoFull` PASS）。

### 対応の実現可能性と推奨修正方法

推奨（実測済み・正典経路順序維持）: `imageProxy` の **remote path 先頭**（`serveLocalImage`/`serveStoredImage` の cache hit 判定後・`gg := v.client.GG()` 使用前、`app/viewer.go:1082-1083` の位置）で `EnsureGG(r.Context(), false)` を 1 行追加。TTL 内は mutex 判定のみ（network なし）、TTL 超過時だけ data lane gate 経由で 1 回。throttle は force にのみ適用（`app/gg.go:135`）なので、throttle 内でも TTL 判定で gg が更新され、最初の remote fetch が live window で 200 になり、誤 `no_full` 記録が発生しない（実測 PASS）。negative key が gg を含むため、fresh gg の key で記録される `no_full` は正 404 の既存意味どおり。

### 副作用・影響範囲・代替案

- politeness gate・data lane: per-request の追加呼出しは TTL 内は memory 判定のみ（gate 消費なし）。network は TTL 超過直後の 1 リクエストだけ（singleflight 既存）。gset 取得回数は増えない（現行も 404 時 force で 1 回/60 秒上限）。gate・grace（Phase 4 2 レーン）は非接触。
- fast/non-blocking viewer 正典との照合: `imageProxy` はもともと image fetch 待ち（意図された priority wait）で、`showPage()` 非await 条には衝突しない。ただし原文のとおり **handler 最頭**（cache hit 判定前）に置くと、TTL 超過時に cache hit リクエストでも network 呼出しが発生し「cache hit（x-cache: cache|local）で呼ばない」既存意味を弱める（実測: cache hit + TTL 超過で呼出し 1）→ **handler 最頭案は却下**、remote path 先頭（cache hit 判定後）を推奨（実測: cache hit で呼出し 0、TTL 内 remote path memory-only）。
- 正典条件（壊してはいけない）: 経路順序・永続先・shard URL format・live gg 既定（snapshot fallback）・TN 表示源禁止は変更なし（`TestN03PatchedCanonicalOrderIntactLegit404` で正 404 の `no_full`・順序維持を実測）。
- 代替案: ① stale-while-revalidate（async 更新）— UI を待たせないが、最初の取得は stale gg のまま（404 起点が残る）で「TTL 更新されない」の解決にならない。却下（推奨案がより直接的）。② force throttle に TTL 判定も算入 — 正 404 頻発時に throttle 保護が薄くなる。不採用。③ throttle 窓だけ狭める — TTL 更新缺失は残る。不採用。

推定規模: `app/viewer.go` 1 行挿入 + Go test 1 本（mock 流儀、既存 `viewer_test.go` negative cache テストと同型）。優先順位付けに効く要素: 重要度高（フル画像表示の正典条件・false negative）、緊急度中（稼働時に毎時間発生、正 404 頻発で throttle hit 確率上がる）、リスク低（cache hit・gate・singleflight・記録意味は実測で非接触確認）、依存なし（既存 EnsureGG/gate/negative cache の既存意味に接するのみ）。

## Verification

- 実測（scratch コピー `docs/ai/tmp/n03-scratch/`、テストファイル `docs/ai/tmp/n03_current_test.go`・`n03_patched_test.go`、mock・外部 API なし）:
  - `go test -count=1 -timeout 120s -run TestN03Current -v .`: 2/2 PASS（経過 2.8s）
  - `go vet .`（scratch・推奨案適用後）OK、`go test -count=1 -timeout 120s -run TestN03Patched -v .`: 5/5 PASS（経過 5.6s）
- コード検証（本番 app 無変更）: `go vet . && go test -count=1 -timeout 300s .`（app）→ vet OK・`ok hitomi-hub/app 26.700s`（経過 29.9s）
- 実機確認（canonical build・`/api/image/<hash>?format=webp` の 200/x-cache）は未実施（調査のみ・実装未実施のため）。実測は mock で機構・呼出し回数・記録 status 値を確定済み。
- UI 項目でないため画面確認は不要として記録（画面キャプチャなし）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
