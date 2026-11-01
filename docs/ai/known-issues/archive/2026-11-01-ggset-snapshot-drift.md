# ggset.go の switch snapshot は live gg.js とずれる

**Date**: 2026-11-01
**Status**: 解決済み（2026-11-01・実行時に正典 gg.js を読み、snapshot は fallback に降格）

## 症状

`app/ggset.go` は生成時点の `gg.js` switch values と bucket を固定している。live の `gg.js` は回転しており、同じ hash でも live の bucket で組んだ shard URL が 404 になる。

## 実測（2026-11-01）

- snapshot: `gg.b = 1793487601/`、`case → 0`（switch にある value で 0）
- live gg.js: `gg.b = 1793491201/`、case 集合が snapshot と異なる（snapshot に無い `2488` が live の switch にあり、snapshot にある `1286` が live に無い）
- hash `2d18a9a8…42b89`（`g = 2488`）:
  - snapshot の subdomain/bucket（`w2` + `1793487601`）→ `200`
  - live の bucket（`1793491201`）+ live の `m`（`w1`）→ `200`
  - live bucket + snapshot の `m`（`w2`）→ `404`
  - snapshot bucket + live の `m`（`w1`）→ `404`
  → subdomain と bucket は同一時点の gg.js 由来でないと 404

## 影響

- shard（full image）URL 構築。TN path は bucket を使わないため TN には影響なし。
- tests は snapshot の値をハードコードしている（mock・live を呼ばない）。live 検証は snapshot 値で行った。

## 回避策

- `ggset.go` を再生成するときは live で URL を検証してから使う（同一時点の bucket と switch を使う）。
- 生成脚本: `Common_Operations\scratch\hitomi-usage\gen_ggset_current.py`（本リポジトリの `docs/ai/tmp/` に写して実行）。

## 解決（2026-11-01）

- 実行時に正典 `gg.js` を 1 実行 1 回読んで URL を組む（`app/gg.go` の `parseGG` / `GG`）。`ggset.go` は offline 時の fallback。生成正典は `tools/gen_ggset.py`。
- tests は rotating な実値をハードコードしない（fixture で規則を検証）。
- 追加実測: bucket は 1 時間刻みで前進する（`1793491201` → `1793494802`）。9:35 に 200 だった URL は 10:05 に 404（4 組合せすべて）。drift は snapshot の問題ではなく、shard URL が live の窓に data が在る時点のみ解ける性質。

決定: `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`
