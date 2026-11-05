# レート値の集約状況確認と短縮可能域の実測（floor 探索）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 項目1（値の一元化＋下限ガード）を子へフォアグラウンド委譲

## 意図 / 背景

- 前項目（`2026-11-05-assess-20261105-1010-rate-limit-verification.md`）の続柄。レート値が単一の箇所に集約されているか、そして実測上どこまで短縮できるか（floor）を確定する。
- 実装開始指示は受けていない。値変更・コード変更はしない（記録のみ）。

## 調査結果 / 変更内容

### 集約状況（部分集約・1箇所で完結していない）

- 実行時の単一正典は `app/hitomi.go:222`（`defaultHitomiConfig()`）だが、`bg_grace_ms` は `app/hitomi.go:350`（`const defaultBgGraceMS`）が別途保持し、`defaultHitomiConfig()` 内にもリテラル `300` が重複（`app/hitomi.go:223`）。
- `config.example.json` には `min_interval_ms`・`image_min_interval_ms` のみ。`gg_force_min_interval_ms` と `bg_grace_ms` のキーが無い（コード既定と不一致）→ `known-issues/KI-20261105-07-config-example-missing-rate-keys.md`。
- 同一数値の写し（計測・受入条件・方針として）: `docs/ai/api-rate-limits.md`、`docs/ai/usage-memo.md`（2箇所）、`decisions/2026-11-01-hitomi-client-endpoints.md`、`decisions/2026-11-01-image-rate-and-prefetch-concurrency.md`、`decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`、`decisions/2026-11-03-implementation-judgment-approval.md`、`decisions/2026-11-03-remaining-item-recommendations-approved.md`、`decisions/2026-11-03-judgment-waiting-reduction-policy.md`、`backlog-items/p3-10`、`p3-12`、`p3-13`、`p3-17`、テストリテラル（`app/hitomi_test.go:865`、`app/hitomi_test.go:948`、`app/main_test.go:895,907,983`）。
- したがって「値を変えれば全部追随する」構造ではない。変更時に対象は (1) `defaultHitomiConfig()`、(2) `config.example.json`、(3) `api-rate-limits.md`、(4) `usage-memo.md`、(5) 方針系 decisions、(6) 受入条件テスト（`app/hitomi_test.go:865`）の 6 系統。

### 短縮可能域（実測・bounded probe）

- 逐次（1 呼出しずつ）の実測 latency: data ≈ 425 ms/呼出し（handshake 322–360 ms・server 処理 60–100 ms）、image full ≈ 700–900 ms/呼出し。
  → 逐次経路では interval が支配。gate 1000 ms の 7 呼出し検索で gate_wait 6043 ms（`docs/ai/tmp/baseline_search.json`・`p3_baseline_metrics.json`）に対し、250 ms なら gate_wait ≈ 1750 ms（wall 8.48 s → 約 3.0 s）。100 ms なら ≈ 700 ms（wall 約 2.2 s）。
- 並行を上げると latency が支配（throughput = min(concurrency/latency, 1/interval)):
  - data: 並行 4 → 6.6 rps、並行 8 → 12.4 rps、並行 12 → 12.4 rps（全 206・429 なし）
  - image: 並行 4 → 4.5 rps、並行 8 → 8.2 rps、並行 12 → 5.2 rps（全 200・429 なし）
  - 現行 image gate 250 ms（=4 rps）は並行 4 の latency 上限 4.5 rps とほぼ同値 → **image lane は interval ではなく並行度が支配点**。250 ms を 125 ms へ下げても並行 4 のままでは +10% 程度。
- server 側 block の兆候は 100 ms（data・10 呼出し）・25 ms（data・8 呼出し）・並行 12（image・data）まで観測なし。100 ms 未満・長時間持続・累計規模（例: data 25 ms は 2400 呼出し/分）での挙動は未測。

### 実務上の推奨域（判定ではなく案）

- data lane: 250 ms が妥当（逐次検索の実効改善が大きい。100 ms までは実測で通過confirmed、ただし block リスクは未検証域）。
- image lane: 250 ms のまま、あるいは 125–150 ms に短縮し並行度を 8 へ（並行度を上げないなら短縮の効果が薄い）。
- 429 以外の block 形態（403/Cloudflare challenge）は現状リトライ対象外（`app/hitomi.go:532` は 429/5xx のみ）→ interval を下げるなら block 検出の追加が必要。

## 実装実行（assess-20261105-1015・2026-11-05 10:16 UTC）

- goal 開始失敗（2回）→ goal 未使用。コマンド正典 `C:\Users\fujim\.config\kilo\command\implement-items.md` 本文に従い、親は統括のみ・実作業は子へフォアグラウンド委譲（逐次1件）。
- 承認行「直前の内容について承認する。」はメッセージに含まれず。規則どおり直近の assess ID を対象とした（`assess-20261105-1015`＝本タスク）。承認範囲＝本タスク文書に記録済みの「実務上の推奨域」（下記項目1〜4）のみ。
- 項目と優先順位（依存は本記録の記載どおり: 項目3は項目2が前提）:
  1. 値の一元化＋下限ガード（`app/hitomi.go:222` 単一出力元化・`config.example.json` 欠落2キー・`gateLane` 下限ガード）— 対応: `known-issues/KI-20261105-07-config-example-missing-rate-keys.md`
  2. block 検出（403/Cloudflare challenge のクールダウン・`Retry-After` 尊重）— 項目3の前提
  3. 値変更（data 250 ms・image 125 ms・prefetch/download 並行度 8）＋テストリテラル re-anchor（`app/hitomi_test.go:865`・`:948`）
  4. docs ポインタ化（`api-rate-limits.md`・`usage-memo.md` の数値写しを正典参照へ）

## 委譲サイクル記録

- 項目1 委譲前状態: HEAD `73507d1`、`git status` clean（2026-11-05 10:16 UTC）。
- 注意: 本タスクは closed 後に reopen したため `current-task.tsv` の参照行が無い（正典 `update` は行を再作成しない・edit 直接編集は不可）。close 時に正典 `end` で行削除相当の確認を行う。

（機械的確認結果は委譲後に追記）

## Verification

- `python docs\ai\tmp\rate_probe_20261105_floor.py` → `docs/ai/tmp/rate_probe_20261105_floor_results.json`（data 100/50/25 ms 逐次・image 404 は gg snapshot drift で rate 判定外）
- `python docs\ai\tmp\rate_probe_20261105_image.py` → `docs/ai/tmp/rate_probe_20261105_image_results.json`（live gg・submit().result() で並行化が実質逐次だったため下項目で再測）
- `python docs\ai\tmp\rate_probe_20261105_conc.py` → `docs/ai/tmp/rate_probe_20261105_conc_results.json`（真の並行 4/8/12・handshake 分離計測）
- コード変更なし → 本プロジェクトの検証コマンド該当なし（docs のみ）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
