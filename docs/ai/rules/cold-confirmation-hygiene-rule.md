# 本プロジェクトルール原稿: strict cold 確認の hygiene（KI-20261105-02 由来）

**Date**: 2026-11-06
**Status**: active（ユーザー承認 2026-11-05 18:23）
**Scope**: hitomi-hub
**Source**: `docs/ai/known-issues/archive/KI-20261105-02-unexplained-cache-hit-in-cold-run.md`（未解明 observation。手順で防げるためルール化を提案）

## Rule

strict cold 確認（cold state を合格条件として claims する UI/network 実測）では、測定時点で次の3条件をすべて満たすこと。

1. **Canonical build freshness**: 現行 `app` からの fresh canonical `hub.exe`（`tests/test_canonical_build_freshness.py` 合格）から起動した server で測定する。
2. **Fresh server instance**: in-memory `infoCache` は server インスタンスごとに閉じる。同じ实例が測定前に他の tab・run に応答済みなら、disk cache が cold root でも cold claims は無効（warm として記録、または server を再起動して再測定）。
3. **Isolated origin**: 測定用 tab は state のない isolated origin（clean profile）とする。共有 localStorage・stale tab と同一 origin を共有しない。

## 既知の汚染源（根拠）

- `restoreSearchState()` は新規 tab 起動時 localStorage の state から `loadSearchCards()` を実行する（`app/search.html` L350）。cold 測定開始前 already network fetch が発火し、同一 server instance の `infoCache` に membership を作りうる。
- 条件3は clear storage（state を消す）または isolated origin のいずれかで満たす。run E（fresh isolated origin・cold run 25/25 miss）がこの手順で valid だった実績（KI-20261105-02）。

## 再発時手順（KI-02 追跡条の維持）

再発時は network request 全件 + `cacheDistribution` + root disk cache の突合で追跡し、3条件のどれが破れていたかを記録する。

## 採用効果

- 過去の実測で「cold state」 claims を 1 件分弱めた事件（KI-02）を、future measurement で排除できる
- 既存正典（fresh target・canonical build・window size 基準）と矛盾しない。追加条件は origin/storage と server instance の freshness のみ
