# KI-20261105-02 — cold run で 1 件 `x-cache: cache`（未解明・observation）

**Status**: 未解明（observation）
**Date**: 2026-11-05

## 現象

Phase 2 実機確認 run C（canonical build・fresh root `p2_phase2_fresh3`・cold server）で、`female:yuri` p1 cold run で `cache cache=1 miss=24`（1 件 cache hit・当時 first card `4304598` と記録。本 transcript 内では再確認不可）。cold root（`cache/galleryinfo` 空）で `infoCache` membership が発生した経路が特定できない。tab 11 の shared localStorage・startup duplicate fetch・stale tab が候補だが、run E（fresh isolated origin）cold run では再現せず（miss 25）。

## 影響

strict cold UI 確認の「cold state」 claims を 1 件分弱める。run E cold run は 25/25 miss で valid。合格判定は run E の数値で実施（task record 参照）。

## 未確認点

- `4304598` の `infoCache` membership 経路（fetch 記録なし・disk cache 経路も cold root で該当なし）
- shared localStorage 由来の startup duplicate fetch の有無

再発時: network request 全件 + `cacheDistribution` + root disk cache の突合で追跡すること。
