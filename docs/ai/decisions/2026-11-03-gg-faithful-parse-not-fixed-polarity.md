# gg faithful parse（polarity 固定ではなく窓ごとの構造を保持）

**Date**: 2026-11-03
**Status**: 決定・実装済み
**Backlog**: assess-20261103-1220

## 決定（正典規則）

- `parseGG()` / `GG.M(value)` は faithful parse。`GG` が `var o` の初期値（`InitialO`）と switch
  case 代入値（`CaseValue`）を保持し、`M(value)` は「その gg.js が実際に返す値」を返す
  （case 内 → `CaseValue`、case 外 → `InitialO`）。polarity をどちらか片側へ固定しない。
- mapping 方向は固定で有効: `m = 0 → w1`/`atn`、`m = 1 → w2`/`btn`。
- `tools/gen_ggset.py`（正典生成）は生成時点の live から観測した初期 o と case 代入を snapshot
  へ記録する（polarity をスクリプト側でハードコードしない）。`app/ggset.go` を同じ基準で再生成。
- `sameGG()` は bucket・cases・polarity を比較（polarity rotation を gg change として扱う）。

## supersede

- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` line 15 の「case に含まれる value
  は 0」を固定規則とする主張を supersede。構造は窓ごとに rotation するため faithful parse でのみ
  满足される。line 15 の mapping 方向（0→`w1`/`atn`、1→`w2`/`btn`）の記述は有効。
- `docs/ai/decisions/2026-11-03-gg-case-polarity-correction.md`（実装前の判断点）で示した
  faithful parse 方針を実施。当該 decision の結論は有効（本 entry は実装記録として追加）。

## 実装

- `app/gg.go`: `GG` に `InitialO`/`CaseValue` 追加、`parseGG()` が switch body・初期 o・case 代入を読む、
  `M()` faithful 化、`sameGG()` polarity 比較。
- `tools/gen_ggset.py`: 観測 polarity を生成（dry-run にも polarity を表示）。
- `app/ggset.go`: 正典スクリプトで再生成（polarity をファイルへ記録）。
- テスト: `TestCanonicalGGRuleComposition`（両構造・w1/w2 反転固定）、
  `TestSnapshotGGIsPopulatedAndUsedAsFallback`（snapshot polarity・faithful M）、
  `TestStorageURLUsesFormatNotNameExt`・TN 関連テスト（構造差分で subdomain 反転）。EnsureGG/viewer/main
  の fixture を faithful 構造へ更新。

## 実測（2026-11-03・fresh live・現窓）

現窓（12:00 窓・epoch 1793709059・bucket `1793707201/`・cases 2112）: `var o = 1`／case 代入 `o = 0`。
- in-case hash `c05d79ed389022ea...e37d8c63`（g=966）: app 構築 `w1` = 200（300986 bytes）／他 `w2` = 404
- out-of-case hash `4a3c2ca208dc33e3...f40854`（g=1157）: app 構築 `w2` = 200（246926 bytes）／他 `w1` = 404
- 固定 polarity（case→1/out→0）なら in-case は `w2` で 404。faithful parse は現窓の 200 と一致。
- canonical exe 実機（scratch dataDir・8731）: CLI download gallery 4301695（1 回目）`attempted=3 written=3`
  一致。viewer `/api/image/<hash>?format=webp` → in-case 200 `x-cache: miss` 300986 bytes、out-of-case 200
  `x-cache: miss` 246926 bytes（`no_full` なし）。

## tests の方針

- rotating な実値をハードコードしない。fixture で規則を検証し、構造差分で `w1`/`w2`（`atn`/`btn`）が
  反転することを固定。snapshot polarity は生成時点の観測値（テストは値の方向ではなく faithful 挙動と
  反転 invariant を assert）。

関連: `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`、
`docs/ai/decisions/2026-11-03-gg-case-polarity-correction.md`、
`docs/ai/known-issues/2026-11-03-gg-polarity-inverted-vs-live-ggjs.md`、
`docs/ai/tasks/2026-11-03-a10-download-existing-check.md`、
`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、
`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`（作業前チェック読了）。
