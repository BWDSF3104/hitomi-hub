# リモート取得レート制限: 現行設定値の提示と短縮可能性の検証

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 現行値を提示し、code floor と live 実測で短縮可否を検証

## 意図 / 背景

- 大元（hitomi.la / ltn）からの取得レート制限の現行値を提示し、`docs/ai/backlog-items/p3-10-search-speed-measurement.md` の「1000 ms は下げるできない（project policy）」という記載が、サーバー側強制なのかプロジェクト自己方針なのかを切り分けて検証する。
- 実装開始指示は受けていない。値変更は行わず、現行値の提示と短縮可否の証拠取得のみ。

## 現行設定値（提示）

| 項目 | 値 | 定義箇所 |
|----|----|----|
| data lane（nozomi・galleriesindex・gg・galleryinfo） | `min_interval_ms` = 1000 ms | `app/hitomi.go:222`（default）／`config.example.json:15` |
| image lane（full image・TN） | `image_min_interval_ms` = 250 ms | `app/hitomi.go:222`／`config.example.json:16` |
| gg force refresh 間隔 | `gg_force_min_interval_ms` = 60000 ms | `app/hitomi.go:222`（`config.example.json` にキーなし） |
| background lane grace | `bg_grace_ms` = 300 ms | `app/hitomi.go:350`（`config.example.json` にキーなし） |
| retry | max 3・backoff 2000 ms × 試行、429/5xx のみ | `app/hitomi.go:222`・`app/hitomi.go:532` |
| prefetch 並行度 | 4（viewer UI）／4（CLI download） | `docs/ai/decisions/2026-11-01-image-rate-and-prefetch-concurrency.md`・`app/main.go:1062` |

正典 runtime root（`output/hitomi-hub`）に `config.json` は無く、`config.example.json` fallback。よって実効値は上表＝既定値。

## 調査結果 / 変更内容

- **コードに下限はない**（短縮可能なことを確認）:
  - `HitomiConfig.UnmarshalJSON`（`app/hitomi.go:316-348`）と `loadConfig`（`app/main.go:163-172`）は `0` を「未指定＝既定値」として扱うだけで、正の値への下限検査がない。
  - `gateLane`（`app/hitomi.go:382-384`）は `intervalMS <= 0` で即 return（ゲート実質無効）。`1` ms 等の設定はそのまま受け入れられる（テスト `app/hitomi_test.go` で `MinIntervalMS = 1` を多用）。
  - したがって「短くできない」は技術的制約ではなく方針記載。受入条件として `app/hitomi_test.go:864-865`（production gate interval must remain 1000ms）が既定値をハードアサートしている → 変更には受入条件変更（＝承認）が必要。
- **サーバー側強制は観測されない**（live 実測・正典 UA・`docs/ai/tmp/rate_probe_20261105*.py`）:
  - data lane 250 ms（現行の 1/4）で 10 呼出し: 全 206・429 なし・latency 423–438 ms で安定。
  - data lane 100 ms（現行の 1/10）で 10 呼出し: 全 206・429 なし。さらに 6 並行（間隔なし）: 全 206・423–467 ms。
  - image lane 125 ms（現行の 1/2）TN 8 呼出し、および full image 10 呼出しを並行度 4・間隔なし（＝現行ゲート通過量 4 req/s を超える実効 ~5.7 req/s）: 全 200・429 なし。
  - 応答ヘッダにレート関連ヘッダなし（`cf-ray`・`retry-after`・`x-rate-limit` 不在、`cf-mitigated` 不在）。ltn data 経路は `server: nginx/1.31.7`（Cloudflare 通过ではない）。`cache-control: max-age=3600` のみ。
  - 注記: image lane の TN 404 はレート制限ではなく gg drift（live 2112 case・bucket `1793872802/` が snapshot `1793707201/` と不一致）による URL 構成差。rate 判定は full image 200 のみで行った。
- **判定**: 「それ以上短くできない」は検証済みの制約ではない。p3-10 の記載はプロジェクト自己方針（politely wait）であり、server 側 minimum の実測根拠はない。短縮は (a) 方針変更の承認、(b) `app/hitomi_test.go:864` の受入条件 re-anchor を要る。
- 未確認点: 100 ms 未満・長時間持続・同一 IP への累計規模での block 判定は未測（本検証は 20–40 呼出しの bounded probe まで）。adaptive backoff のない現状（429 時に 2000 ms×試行のみ）で低 interval を許容すると block リスクは自己責任側で上がる。

## Verification

- `python docs\ai\tmp\rate_probe_20261105.py`（2026-11-05・bounded・計 20 呼出し）→ `docs/ai/tmp/rate_probe_20261105_results.json`
- `python docs\ai\tmp\rate_probe_20261105_b.py`（live gg 照合＋image lane・計 22 呼出し）→ `docs/ai/tmp/rate_probe_20261105_b_results.json`
- `python docs\ai\tmp\rate_probe_20261105_c.py`（data lane 100 ms＋6 並行・full header dump）→ `docs/ai/tmp/rate_probe_20261105_c_results.json`
- コード変更なし → 本プロジェクトの検証コマンド（`go vet` / `pytest`）は該当なし（docs のみ）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
