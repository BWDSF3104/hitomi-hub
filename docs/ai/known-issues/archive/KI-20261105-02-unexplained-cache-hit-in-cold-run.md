# KI-20261105-02 — cold run で 1 件 `x-cache: cache`（未解明・observation）

**Status**: クローズ（ルール採用 2026-11-05 18:23・観察自体は未解明のまま rules で恒久化）
**Date**: 2026-11-05

## 現象

Phase 2 実機確認 run C（canonical build・fresh root `p2_phase2_fresh3`・cold server）で、`female:yuri` p1 cold run で `cache cache=1 miss=24`（1 件 cache hit・当時 first card `4304598` と記録。本 transcript 内では再確認不可）。cold root（`cache/galleryinfo` 空）で `infoCache` membership が発生した経路が特定できない。tab 11 の shared localStorage・startup duplicate fetch・stale tab が候補だが、run E（fresh isolated origin）cold run では再現せず（miss 25）。

## 影響

strict cold UI 確認の「cold state」 claims を 1 件分弱める。run E cold run は 25/25 miss で valid。合格判定は run E の数値で実施（task record 参照）。

## 未確認点

- `4304598` の `infoCache` membership 経路（fetch 記録なし・disk cache 経路も cold root で該当なし）
- shared localStorage 由来の startup duplicate fetch の有無

再発時: network request 全件 + `cacheDistribution` + root disk cache の突合で追跡すること。

## ルール化提案（2026-11-06）

汚染源（startup duplicate fetch・shared origin・server instance 共有）は手順で防げるため、ルール原稿を作成: `docs/ai/rules/cold-confirmation-hygiene-rule.md`。ユーザー承認 2026-11-05 18:23 により active へ採用し、本KIは観察記録として archive へ移動（観察自体は未解明のまま、規則として恒久化。再発時は rules の3条件のどれが破れていたかを突合して分類）。
