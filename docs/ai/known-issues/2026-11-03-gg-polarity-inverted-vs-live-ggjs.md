# gg polarity 固定は成立しない（A-10 を block した原因・窓 rotation）

**Date**: 2026-11-03
**Status**: Open（faithful parse 実装の承認待ち）
**Backlog**: assess-20261103-1220

## 現象

- A-10（download 既存確認）の実装・mock テストは合格済みだが、live CLI 実測が gg polarity の窓依存で block されている。
- 原因の核心は「polarity 反転」ではなく、live `gg.js` の switch 構造（`var o` 初期値と case 代入値）が窓ごとに反転すること。code 側の固定値（case → 1）は `var o = 0` 窓でのみ live と一致し、`var o = 1` 窓では組む subdomain が live 側と逆になり全件 404。

## 実測値（2026-11-03）

- 11:38Z live: `var o = 0`／case → `o = 1`（bucket `1793703601/`・cases 1984）。window data（gallery 4301695・50 files）: in-case hash `w2`=200・`w1`=404／out-of-case hash `w1`=200・`w2`=404（p8）。TN subdomain `atn`/`btn` は双方 200（p4・同一 bytes）。
- 10:0x 窓（A-10): `var o = 1`／case → `o = 0`（bucket `1793700002/`・cases 2112）。in-case hash `e3ee8d44...735fd9`（g=2557）: `w1`=200（`320782 bytes`）／`w2`=404。
- 12:05Z（12:00 窓）: `var o = 1`／case → `o = 0`（cases 2112）。同一 gallery 4301695 の全 50 file が live 側（in-case → `w1`）で 200。
- 現行 code 実機（12:0xZ・canonical exe・8731）: CLI download gallery 4301695 → `attempted=50 written=0`（全 404・code 構築 URL は live 側と逆）。viewer `/api/image/c05d79ed...?format=webp` → `404` `x-cache: no_full`。

## 判定

- decision `2026-11-01-ggset-generation-and-drift.md` line 15 の固定構造主張は supersede（mapping 方向 0→`w1`/`atn`、1→`w2`/`btn` は有効）。正典規則は faithful parse（初期 o・case 代入を live から読む）。
- 本項目の承認 premise（fresh live 実測 = decision 側）は 11:38Z 実測で不成立。polarity の片側固定（どちらへ反転させても）は窓 rotation で同じ失敗を繰り返すため、code 変更は実施しない。実装判断（faithful parse 導入）をユーザーへ持ち帰る。

## 影響

- A-10: live 実測（1 回目 attempted==written）は `var o = 0` 窓でのみ成立。項目は進行中 maintained（close せず）。tsv 参照行は削除しない。
- fixture（`var o = 0; case → o = 1`）は 11-01 10:23 時点の構造基準であり「live 構造の正典」ではない。faithful parse 導入時に fixture assertion を規則基準へ更新が必要。
- TN display policy・imageProxy 順序・`use_live_gg`・B-01〜B-09・A-07b・C-08 は本件と無関係（変更なし）。TN host は interchangeable のため TN 表示源規則には影響しない。

probe: `docs/ai/tmp/assess-20261103-1220/polarity_probe*.py`・`polarity_ggjs_raw*.js`・`polarity_live.py`（実機確認出力 `polarity_live_out.txt`）。

関連: `docs/ai/decisions/2026-11-03-gg-case-polarity-correction.md`、`docs/ai/tasks/2026-11-03-a10-download-existing-check.md`、`docs/ai/known-issues/archive/2026-11-01-parsegg-switch-value-inversion.md`。
