# ggset の生成正典と live gg.js 追随（snapshot は fallback）

**Date**: 2026-11-01
**Status**: 決定・実装済み

## 決定

- `app/ggset.go` は**snapshot**（fallback）で、実行時は正典 `gg.js` を 1 実行 1 回読んで URL を組み込む。
- 生成正典スクリプト: `tools/gen_ggset.py`（live `https://ltn.gold-usergeneratedcontent.net/gg.js` から bucket と switch values を読んだ Go ファイルを生成。`--dry-run` 対応）。プロジェクトごとのコピーは作らない。
- 実行時読取: `app/gg.go` の `parseGG()` / `GG`。`ImageURL`・`StorageURL`・`ThumbnailURL` は `GG` を受け取る。`Client.LoadGG()` が失敗時 `snapshotGG()` へフォールバックし、CLI は stderr に `gg: snapshot fallback` を出す。
- config: `hitomi.use_live_gg`（既定 true）。

## gg.js の構造（規則）

- `m(g)`: switch の `case` に含まれる value は `0`（subdomain `w1` / `atn`）、それ以外 `1`（`w2` / `btn`）
- `s(hash)`: 末尾 3 hex を反転して 10 進（`parseInt(m[2]+m[1], 16)`）
- `b`: shard の base path（末尾 `/`）

## 実測（2026-11-01・rotation の証拠）

- 9:35 取得の `gg.js`: `b = 1793491201/`、cases 2112（`2488` を含む）
- 10:0x 取得: `b = 1793494802/`、cases 1984（`2488` を含まない）
- bucket 値は unix 時分（`1793491201` = 2026-11-01T00:00:01Z、`1793494802` = 2026-11-01T01:00:02Z）→ **1 時間刻みで前進**する
- hash `2d18a9a8…42b89`（value 2488）:
  - 9:35 の live（`w1` + `1793491201`）→ 200
  - 10:05 の live（`w2` + `1793494802`）→ 404。同一 hash の `w1`/`w2` × `1793494802`/`1793491201` の 4 組合せはすべて 404（`docs/ai/tmp/gg_probe2.py`）
- shard URL は「その時点で live gg.js が指す窓に data が在るとき」のみ解ける。snapshot 値だけで組んだ URL は時間とともに 404 になる（drift の本質）。
- TN path は bucket を使わないため shard の rotation の影響を受けず、実測 200（`image/webp` 55320 bytes）。subdomain は live gg.js の `m` 由来（この時点では `btn`）。

## tests の方針

- rotating な実値をハードコードしない。fixture（`parseGG`）で規則を検証する:
  - `TestCanonicalGGRuleComposition` — cases 内 → `w1`/`atn`、cases 外 → `w2`/`btn`
  - `TestSnapshotGGIsPopulatedAndUsedAsFallback` — snapshot 非空、fetch 失敗時 snapshot、`use_live_gg: false` で snapshot
  - `TestStorageURLUsesFormatNotNameExt`、`TestThumbnailURLMatchesVerifiedTNPath` — fixture で base バイトと path 構造を検証

関連: `docs/ai/known-issues/archive/2026-11-01-ggset-snapshot-drift.md`
