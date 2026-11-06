# R-11: 複数 term の取得が逐次（gate は間隔のみ直列化）

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 判定済み（逐次維持・FIFO・grace 変更なし）。interval 値の短縮方向は R-11b で追跡。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

- 外部指摘（2026-11-06 項目 11）: 複数検索語の取得が逐次で、体感で数十秒級。並行化の可否を判断する項目。
- 判断の前に「語ごとに分けて 1 語ずつ取得するのが元サイトと違うのか」を確認する。
- 確定済みの承認・前提（変更しない）: 判定は逐次維持。politeness gate の FIFO・grace・FIFO の意味は変更しない。data lane `min_interval_ms` を 90 ms（100 ms の点のみ supersede）。下限 guard 50 ms・image lane 60 ms・politeness 退避先 1000 ms は不変。実測は P3 #10 で gate wait と network を分離して行う。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- 語ごとに分けて 1 語ずつ取得するのは正典準拠。元サイトは 1 リクエストで複数語をまとめて投げる做法ではなく、語ごとの data を取って共通部分を作る（`app/searchhtml.go` の `searchAllIDs` が正典 `results.js` / `search.js` の `get_galleryids_for_query` の集合演算を mirror。`app/hitomi.go:835` に同じ分区規則の記録）。
- 補足（2026-11-06 質問）: 語ごとの分割取得は AND 検索そのもの（共通部分）。`or` は group 内 union、`-term` は exclusion。R-11 の指摘は集合の意味ではなく、取得の逐次・同時の話。
- したがって、集合（出る結果）は元サイトと同じ。並行化は結果を変えず、タイミングだけ変わる。
- ただし同一性が壊れる条件がある:
  1. 順序の起点を変える（最初の positive term の dataset、または plain term で `orderbykey != 'added'` のとき order dataset を使う分岐・D-09）
  2. `language:` term の pre-scan（C-04）の扱いを変える
  3. cache key（`area|tag|language`、order を含む）を変える
- これを満たせば並行化は結果同一。満たさないと順序・集合が変わる。
- 正典 JS で確認したのは集合演算の単位（語ごと）まで。ブラウザ側の取得の並行度（逐次か同時か）は未観測。
- 決定（2026-11-06）: 逐次のまま。politeness gate（1000ms・FIFO・grace）の意味を変えない。体感速度は R-10（同じ検索条件の最終結果を短 TTL で覚えて、page 移動・prefetch・firstStatus で共有）で回収する。
- 実測は P3 #10（`docs/ai/backlog-items/p3-10-search-speed-measurement.md`・採用済み）で gate wait と network を分離して行う。実測前に並行化は採用しない。
- 決定ファイル: `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`

### 有効性

- **一部有効**。機構（語ごとの逐次取得・呼出数=語数ぶん）は有効。指摘の「数十秒級」は 3 語・5 語の cold 検索では実測で支持されない（注入 network 265 ms/呼で 1.59 s / 2.12 s）。数十秒級になるのは呼出数が増える条件（語数・index 降下 depth・retry・warm での再計算）を仮定した場合で、その外挿は未確認。
- gate 直列化の意味は正典どおり（`docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`・`app/hitomi.go:155-166`、`381-393`）。FIFO・将来予約・grace・共有 `last` 連鎖は維持。
- 実測で分かった支配関係: network（呼あたり）が interval 以上のとき gate wait は 0（gate は既に満たされている）。この条件では 90 ms 値変更の効果は 0。interval が支配するのは network が interval 未満のときだけ（warm・高速 CDN・呼が連続する条件）。
- 並行化の_effect_は network の重なりだけ。gate-bound では効果がない（実測で並行の方が悪い）。

### 実測（mock `Client.fetch`・外部 API 呼出しなし）

方法: `docs/ai/tmp/assess-20261106-1056/r11/pkgcopy/` に app パッケージの写し（非 test 源 + embed html）を置いた独立 test で測定。既存テストは変更していない。overlay 追加は `go test` が新ファイルを追加できないため不可（記録）。fixture は cold（client ごと cache なし）で、呼出構成 = version 1 + top node 1 + shared leaf node 1 + term data N（降下 depth 1・共有）。network は注入値 265 ms/呼（基線引用: `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md` の data 基線 A 265 ms）と 0 ms（gate 成分分離）の 2 条件。gate/network 数値は正典 metrics（`app/metrics.go` の kind 別 `gate_wait_ms` / `network_ms`）。

| interval | 語数 | network注入 | 逐次 wall | gate wait | network | calls |
|---|---|---|---|---|---|---|
| 100 ms | 3 | 0 | 500 ms | 498 ms | 0 | 6 |
| 100 ms | 3 | 265 ms | 1.593 s | 0 | 1590 ms | 6 |
| 100 ms | 5 | 0 | 701 ms | 698 ms | 0 | 8 |
| 100 ms | 5 | 265 ms | 2.124 s | 0 | 2120 ms | 8 |
| 90 ms | 3 | 0 | 451 ms | 448 ms | 0 | 6 |
| 90 ms | 3 | 265 ms | 1.592 s | 0 | 1590 ms | 6 |
| 90 ms | 5 | 0 | 631 ms | 627 ms | 0 | 8 |
| 90 ms | 5 | 265 ms | 2.124 s | 0 | 2121 ms | 8 |

- 90 ms の効果（実測）: `(calls − 1) × 10 ms` = 3 語 49 ms、5 語 70 ms。network 支配条件では差 0。
- 並行化の上限（同一 gate 演算・concurrent dispatch・採用しない): network 265 ms 条件で cold-parallel 1.366 s（3 語）/ 1.976 s（5 語）、shared path warm 後 1.263 s（3 語）/ 1.462 s（5 語）→ 90 ms では 1.256 s / 1.422 s。改善幅は約 330–700 ms。集合・順序は逐次と同一（`setEqual=true`・順序の起点は最初の positive term を維持）。
- 予算 1.5 s に対し network 支配の cold 逐次は 3 語 1.59 s、5 語 2.12 s で**超過**。超過するので並行化案は「実測結果が予算を超える場合だけ検討する案」として記録し、採用しない（正典 decision 参照: `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` §R-11）。

### 対応の実現可能性と適切な修正方法（推奨）

- 推奨（確定済みの承認どおり）: **逐次維持**。値変更は `defaultHitomiConfig()`（`app/hitomi.go:237-240`）を 90 ms へ、`config.example.json`（15 行）を 90 ms へ。効果は gate-bound 条件の 50–70 ms だけ。network 支配の体感（1.6–2.1 s）は R-10 の結果 cache と P3 #10 の phase timing で回収する。
- 候補（検討案・採用しない・追加承認要）: interval 演算（共有 `last` 連鎖・FIFO・grace）を変えない形での in-flight 重なり。実測上、interval/FIFO 算術そのものは維持したまま network だけが重なる。ただし実運用では次の追加前提が要る:
  1. index path の dedup（R-13 承認済み・未実装）。cold 並行は cache stampede で呼出が増える（実測 calls 6→10、8→16〜20。5 語・100 ms では逐次 2.124 s に対し並行 2.166 s と悪化）。
  2. 順序の起点（最初の positive term）と cache key を固定する制約（条件 1〜3）。
- gate-bound の短縮は 10 ms/呼 の範囲。これ以上の短縮は下限 guard（50 ms）または interval 意味の変更が必要で、現方針の範囲外。

### 副作用・影響範囲

- 実測（fgPending）: 5 語を concurrent dispatch すると `fgPending` の最大が 4（同一 gate・演算不变）。逐次は 0（完了時）。Phase 4 の「フォア遅延 ≤1 枠」の実際の意味が、foreground burst で ≤N 枠へ変わる（background/prefetch の待ちが増える）。FIFO・grace・interval 算術は不变だが、foreground 優先の体感意味が変わる — この論点の実装は承認を要る。
- metrics の gate 数値は並行時に呼出ごとに加算される（合計が wall を超える）。gate wait の解釈は「呼出の合計」であって「wall 内の待ち」ではない（実測で gate 7993 ms / wall 1.501 s）。報告時に区別が必要。
- 正典の stale 記述（実測ではなく読み取り）: `app/hitomi.go:179-180` と `382` のコメントは data lane 本番既定を「1000 ms」と書く。実体は `defaultHitomiConfig()` = 100 ms（90 ms 変更後は 90）。1000 ms は politeness 退避先。実装時に re-anchor が必要。
- 正典指针の矛盾（記録）: 指示の参照 `docs/ai/decisions/2026-11-03-a05-interactive-search-priority-fifo-and-30s-grace.md` は read File not found・Glob 0 件（実体なし）。実在する FIFO/grace 正典は `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`（grace 既定 300 ms・FIFO 算術維持）で、日付優先ルールによりこちらを採用。

### 90 ms 値変更で更新が必要になる既存テスト（実在名）

- `TestRateGateReturnsImmediatelyOnContextCancellation` — `app/hitomi_test.go:865`（`c.cfg.MinIntervalMS != 100` を assert。866 行の message と 861 行の comment も 100 ms を引用）。885–886 行の elapsed assertion は 90 ms でも通過するが message 文の re-anchor が必要。
- `TestConfigExampleHitomiKeysMatchCodeDefaults` — `app/main_test.go:1304`（`cfg.Hitomi != defaultHitomiConfig()`）。`config.example.json` の `hitomi.min_interval_ms`（15 行）を 90 に更新しないと失敗。
- `TestParseRetryAfterRespectsValueAndCapsAtCanonicalConvention` — `app/hitomi_test.go:1542`（1000 > default）。90 ms でも通過（変更不要）。
- `TestLoadConfigHitomiRateFloorAcceptsFiftyAndAbove` — `app/main_test.go:1251`（floor 50）。通過（変更不要）。
- 訂正: 決定ファイル（`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` §R-11）が更新対象として挙げた `app/hitomi_test.go:1507-1528` は `Test429WithoutRetryAfterKeepsExistingBackoffAndNoCooldown`（1495 行）で、100 ms は `RetryBackoffMS`（1499 行）と `testGateIntervalMS = 1`（1497 行）による。data lane 既定の assert ではないため 90 ms 変更で更新は不要。

### UI

- 対象は取得経路（Go）で UI 項目ではない。画面確認・captcha 保存は対象外（未実施）。

## Verification

- 読み取りのみ。既存テスト・本番コードディレクトリへファイル写入なし。変更は個別タスク記録と `docs/ai/tmp/assess-20261106-1056/` のスクラッチのみ。
- `go vet .`（app）: clean・1.16 s。
- `go test -count=1 -timeout 300s .`（app・既存テスト・overlay なし・100 ms 基準): `ok hitomi-hub/app 25.325s`（コマンド全体 28.3 s）。既存テストは通過（90 ms 変更前の実測）。
- 実測（スクラッチ copy・`docs/ai/tmp/assess-20261106-1056/r11/pkgcopy/`）: `go test -count=1 -timeout 300s -v -run "TestR11FgPendingBurstUnderParallelDispatch|TestR11ColdMultiTermSequentialVsParallelCeiling" .` → `ok hitomi-hub/app 33.851s`（measurement 28.8 s + 4.1 s）。結果は上の表と並行上限の値。外部 API 呼出しなし。
- pytest 側は対象外（search 経路は Go）。
- 未確認点: browser 側 fetch の並行度。実機 network latency（265 ms は基線引用で注入値）。index 降下 depth が 1 より深い場合の呼出数（実測は depth 1・共有 leaf）。90 ms 適用後の実機 wall（実装前）。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（`task_manager end` で実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [ ] `backlog.md` の完了項目を削除（該当なし）
