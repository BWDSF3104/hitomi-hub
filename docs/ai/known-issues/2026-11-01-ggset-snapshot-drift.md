# ggset.go の switch snapshot は live gg.js とずれる

**Date**: 2026-11-01
**Status**: 未修復（運用で回避）

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
