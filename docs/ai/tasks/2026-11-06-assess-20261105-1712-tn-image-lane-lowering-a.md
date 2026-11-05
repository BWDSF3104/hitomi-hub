# TN 取得間隔の単純下げ（案A）の検討

**Status**: 進行中（検討完了・値の判断保留）
**Date**: 2026-11-06
**Backlog**: P3 #10 関連（案A）
**Next Action**: 値の承認（推奨 60 ms。63.75 ms で prefetch 経路が UI 上限に当たるため 50 ms は効果頭打ち）。承認後の実装: defaultHitomiConfig 1 値 + config.example.json + decision entry + docs sample。健康な上流窓で bounded 持続実測（block 未発生・gate_wait 60 ms・TN 15〜16 rps）

## 意図 / 背景

ユーザー指示「案A（取得間隔の単純下げ）で検討」。条件は「既にある検索結果先読み（prefetch）の速度が落ちることは許容できない。影響のない設計にすべき」。案A は gate 論理を変えず、正典値（`image_min_interval_ms`）だけを変える形。値の根拠・影響范围・テストアンカー・退化経路をここで確定する。実装は未着手（実装開始指示未受領）。

## 調査結果 / 変更内容

### 正典現値とガード（実測・コード一致）

- image lane 125 ms・data lane 100 ms（`app/hitomi.go` `defaultHitomiConfig()`）
- 下限ガード `const minHitomiRateFloorMS = 50`（`app/main.go:150`、`loadConfig` 175-176 行）: 0/負値は「未指定＝既定値」、1〜49 ms はキー名を含む設定エラーで拒否、50 ms 以上は受理。案A はこのガード内に収まる
- `gateLane`・FIFO・共有 `last` 連鎖・`bg_grace_ms` 300・C-10 test inject 口・metrics kind 分類は非接触（案A は値のみ）

### ボトルネック算術（TN 1本 median 255 ms は実測 2026-11-05。以下は算術＝推定）

- 枠の上限 = 1000 ÷ interval。125 ms → 8.0 rps（実測 8.2 rps と一致）
- UI 側の潜在 = worker ÷ latency: prefetch 4 → 15.7 rps、search card worker 8 → 31.4 rps、card cover は `<img>` 経由（per-origin 上限 ≈6・推定）→ 23.5 rps
- 効く方は min(枠の上限, UI 潜在)。125 ms では枠が bound（8.0 < 15.7）
- crossover: prefetch 経路が UI bound になる interval = 255 ÷ 4 = **63.75 ms**
- → 60 ms なら prefetch 経路は UI bound（15.7 rps）、card cover 経路は枠 bound（16.7 rps）。**60 ms 未満へ下げても prefetch 経路は UI 上限で頭打ち**。追加の効果はなく block exposure だけ増える。だから 50 ms（下限）を推す理由がない
- cold 1 page（25 card・TN）floor: 25 × 125 = 3.13 s → 60 ms 時 ≈ 1.6 s（推定）

### full image 経路（同一 lane）

- TN と同じ lane。full latency 700〜900 ms（実測）→ UI 潜在 8 ÷ 0.8 ≈ 10 rps < 16.7 → 60 ms では UI bound。枠由来の gain は 8 → 10 rps で頭打ち
- bytes/sec（推定）: TN ≈ 15.7 × 8,100 B ≈ 0.13 MB/s、full ≈ 10 × 250 KB ≈ 2.5 MB/s。TN 単独より full 側の負担が大きい

### 既存先読みへの影響（要求の保証）

- `tests/test_search_prefetch.py` が固定する定数（idle 800 ms・concurrency 4・cooldown 5 分）、中断条件、page+1 限定、cool down は非接触。テストは interval を固定していないので re-anchor 不要
- 125 → 60 ms は prefetch を遅くする方向に働かない（算術は単方向の改善）。lane 間の相互待機（TN が full の待ちを継ぐ）を作らない

### 唯一の退化経路

- block signal（403・`cf-mitigated`・503・522）→ 両 lane 5 分間 1000 ms へ退避（`app/hitomi.go:34-35`、`gateLane` 405-406 行）。この間だけ prefetch は現状より遅くなる
- 100 ms 持続（12 並行）は 429・rate ヘッダなしで実測済み。100 ms 未満の持続呼出しは未測（`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md` 未確認点）。上流 TN の 502 不安定（2026-11-05 06:07 以降）で C 系再測定が未確認のまま（`docs/ai/tasks/2026-11-05-phase4-gate-priority-implementation.md`）→ 60 ms は健康な窓での持続実測をもって正典化する

### 実装規模とアンカー（案A）

- `defaultHitomiConfig()` 1 値、`config.example.json` 1 値、decision entry（125 ms 維持の該当行を supersede）、docs（`usage-memo.md` の sample 数値を正典値へ。`p3-10` の実測 fact とポインタは維持）
- `TestConfigExampleHitomiKeysMatchCodeDefaults` は両更新で整合。`TestImageGateUsesLowerImageIntervalSeparately`（cfg 直接設定）と `main_test.go` の明示値アサート（1000）は `loadConfig` 非経路なので非接触。`TestLoadConfigHitomiRateFloorZeroAndNegativeUseDefaults` は `defs` 参照で整合
- lane 分離（案B）は算術上不要（60 ms で枠が bound でない経路があるため effect が頭打ち）。案A で閉じる

### 合格条件（案A）

1. `app` で `go vet . && go test -count=1 -timeout 300s .` 合格。ルートで `python -m pytest tests/ -v` 合格（example 照合含む）
2. 実機（canonical build・scratch instance・健康な上流窓・bounded 呼出し）: metrics の `gate_wait` 分布が 60 ms に一致、TN throughput 期待 15〜16 rps（UI bound 確認）、block signal 0・cool down 未発動、`x-cache` の `tn`/`tn-cache` 意味維持、TN byte を full cache に保存しない
3. search prefetch 固定テスト無変更（定数・中断条件・cool down・page+1 限定）
4. full image 表示源・TN display fallback 禁止ガードテスト合格（非接触確認）

### 判断保留（承認が必要）

- 値: **60 ms（推奨）** / 80 ms（100 ms 実測帯に近く保守的）/ 50 ms（下限・効果頭打ちで推薦根拠なし）
- 上流回復後の持続実測（100 呼出し未満・bounded）で block 未発生を確認してから正典化

## Verification

未実行（検討段階）。実装・実測実施後にコマンド・結果・経過時間を記録する。

## 並行タスク

`docs/ai/tasks/2026-11-05-saved-status-definition.md` が進行中（実装開始指示未受領）。本検討は docs のみで並行継続。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
