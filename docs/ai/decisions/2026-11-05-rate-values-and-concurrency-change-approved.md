# 2026-11-05 — 取得レート値・並行度の変更（実測に基づく方針変更）

**実行固有ID**: `assess-20261105-1015`
**Status**: 承認済み（実装実行の承認範囲。ユーザー指示は `implement-items.md` 実行の goal objective。承認行「直前の内容について承認する。」はメッセージになく、規則どおり直近の assess ID（本実行）を対象とした）

## 決定（正典値）

- data lane `min_interval_ms`: 1000 ms → **250 ms**
- image lane `image_min_interval_ms`: 250 ms → **125 ms**
- prefetch / CLI download 並行度: 4 → **8**
- `gg_force_min_interval_ms`（60000）と `bg_grace_ms`（300）は変更しない
- 正典単一出力元: `app/hitomi.go` の `defaultHitomiConfig()`（`bg_grace_ms` は `const defaultBgGraceMS` を参照）。docs の数値写しは正典参照（ポインタ）へ。
- `loadConfig` で interval の下限ガード（1〜49 ms は設定エラーで拒否、0 は「未指定＝既定値」維持）。
- block signal（403・`cf-mitigated`・503・522）はクールダウン（両 lane を 5 分間 1000 ms へ退避）＋ `Retry-After` 尊重。429/5xx の既存 backoff（2000 ms×試行）は維持。

## 根拠（実測・2026-11-05）

- server 側の minimum interval は観測されず（100 ms 間隔・6 並行・12 並行で 429 なし・rate 関連ヘッダなし・ltn data 経路 `server: nginx`・`cf-ray` 不在）。記録: `docs/ai/tasks/2026-11-05-assess-20261105-1010-rate-limit-verification.md`・`docs/ai/tmp/rate_probe_20261105_*_results.json`
- latency 支配（data ≈ 425 ms・image full ≈ 700–900 ms）。逐次検索の gate_wait（7 呼出し・1000 ms）6043 ms → 250 ms で約 1750 ms。image 並行 8 実測 8.2 rps（125 ms gate の ceiling と一致）。
- block 検出（項目2・commit `7ee82bf`）を前提として実装済み。403/challenge は単発エラーで終わる現状（429/5xx のみ再試行）を是正。

## supersede する決定

- `docs/ai/decisions/2026-11-01-hitomi-client-endpoints.md`（呼出し間隔 1000 ms）→ data lane の数値部分のみ supersede（server 強制ではないという旨は維持）
- `docs/ai/decisions/2026-11-01-image-rate-and-prefetch-concurrency.md`（image 250 ms・prefetch concurrency 4）→ 数値を supersede
- `docs/ai/decisions/2026-11-03-implementation-judgment-approval.md`・`2026-11-03-remaining-item-recommendations-approved.md`・`2026-11-03-judgment-waiting-reduction-policy.md` の「本番 gate interval 1000ms は変更しない（C-10）」→ 本決定により supersede（C-10 の test inject 口そのものは維持）
- `docs/ai/backlog-items/p3-10-search-speed-measurement.md:24` の「cannot be lowered」→ server 制約ではなく方針である旨へ re-anchor（項目4）

## 未確認点

- 100 ms 未満、持続呼出し、累計規模（250 ms でも 240 呼出し/分）での block 挙動は未測。実装後の実機確認（正典 build・bounded）で 429/403 が観測されないことを追加確認する。
