# 2026-11-06 — 検索結果サムネイル（TN）の取得間隔を 60 ms へ（案A）

**実行固有ID**: `assess-20261105-1712`
**Status**: 承認済み（ユーザー 2026-11-05 17:16 UTC「承認」。値 60 ms・実装範囲の承認。実測で閉じ済み）
**タスク記録**: `docs/ai/tasks/2026-11-06-assess-20261105-1712-tn-image-lane-lowering-a.md`

## 決定（正典値）

- image lane `image_min_interval_ms`: 125 ms → **60 ms**（正典単一出力元 `app/hitomi.go` `defaultHitomiConfig()`。`config.example.json` も同値へ）
- 変更しない: data lane `min_interval_ms` 100 ms、`gg_force_min_interval_ms` 60000、`bg_grace_ms` 300、`max_retries` 3、`retry_backoff_ms` 2000、下限ガード `minHitomiRateFloorMS` 50、block signal（403・`cf-mitigated`・503・522）→ 5 分クールダウン（両 lane 1000 ms 退避）、`Retry-After` 尊重
- search page 側の動作は無変更（ユーザー条件「既存の検索結果先読みの速度が落ちることは許容できない」）: `SEARCH_CARD_CONCURRENCY` 8、`PREFETCH_CONCURRENCY` 4、`PREFETCH_IDLE_MS` 800、`PREFETCH_COOLDOWN_MS` 300000、中断条件、page+1 限定、eager cover 8 件、`priority=background` マーカー
- 案B（TN 専用 lane 分離）は不採用。算術上、60 ms で枠が bound でない経路があるため効果が頭打ち
- TN display policy・full image policy・TN byte を full cache に保存しない規則は非接触

## 根拠（実測と算術を区別）

- TN 1 本 median 255 ms（実測 2026-11-05）。枠の上限 = 1000 ÷ interval
- 推定（算術）: prefetch 経路の UI 潜在 = 4 ÷ 0.255 s = 15.7 rps → crossover interval = 255 ÷ 4 = **63.75 ms**。60 ms では枠 16.7 rps ≒ UI 潜在。50 ms では枠 20 rps だが UI 潜在で頭打ち（効果増なし・block exposure 増のみ）→ 下限 50 ms は不採用
- card cover 経路は `<img>` 経由（per-origin 上限 ≈6・推定 → 23.5 rps potential）なので 60 ms で枠が bound。cold 1 page（25 card）TN floor 3.13 s → ≈1.6 s（推定）
- full image 経路（同一 lane）は UI 潜在 8 ÷ 0.8 s ≈ 10 rps（実測 latency 700〜900 ms）で UI bound
- server 側 minimum interval は未観測（100 ms・12 並行で 429・rate 関連ヘッダなし）。100 ms 未満の**持続**呼出しの block 挙動は未測のまま（本実測は bounded burst）

## 実測（2026-11-05 17:26 UTC・canonical build・scratch instance 8742・正典 runtime cache 非汚染）

- gate 間隔: 12 本を並列投下。`image` kind `gate_wait_ms` 合計 3973 ms。FIFO 将来予約の算術（60×(0+1+…+11) = 3960 ms）と一致 → **60 ms で動作確認**
- 同 burst: `image` calls 12、network 合計 5188 ms（mean 432 ms）、retry_sleep 0（リトライ発生なし）、wall 1015 ms
- 内訳: TN 200 が 9 本（bytes 17,270〜55,320・`x-cache: tn`）、失敗 3 本（`x-cache: no_tn`・アプリ側 502）。403・`cf-mitigated`・503・522 は 0・クールダウン未発動
- viewer 実機: `/api/image/c05d79ed…e37d8c63?format=webp` → 200・`x-cache: miss`・300,986 B（`no_full` でない）
- CLI download 実 gallery 4301695（scratch root・`--limit 2`）: `attempted=2` == `written=2`・`skipped=0`
- 外部呼出し数: gg 1・nozomi 1・galleryinfo 6・TN 12・shard full 2・viewer full 1 = 23（bounded）

## 合格条件（実測で閉じた）

- `go vet .`（app）clean、`go test -count=1 -timeout 300s .`（app）pass 25.171 s、`python -m pytest tests/ -v` 53 passed 11.94 s（`TestConfigExampleHitomiKeysMatchCodeDefaults` 含む）
- `TestImageGateUsesLowerImageIntervalSeparately` の正典値アサートを 60 ms へ re-anchor（decision 参照は本 entry）。`main_test.go` の明示値アサート（50/125 は fixture 値・下限ガード経路）は非接触
- search prefetch 固定テスト（`tests/test_search_prefetch.py`）無変更・pass

## supersede する決定

- `docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md` の「`image_min_interval_ms`: 125 ms を維持」の点のみ supersede。同 decision の data lane 100 ms・search page 定数・block 系は維持

## 未確認点

- 100 ms 未満の持続呼出し（本実測は 660 ms 分の burst）での block 挙動。次回は長い窓で確認する
- 上流 TN の不安定（`x-cache: no_tn`・502）は appside 表示（proxy は上流 status を透過しない）であり、rate 起因とは分離できない
