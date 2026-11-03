# gg switch polarity は固定できない（窓ごとに構造が rotation・faithful parse が必要）

**Date**: 2026-11-03
**Status**: 決定（実装未実施・承認待ちの判断点）
**Backlog**: assess-20261103-1220

## 決定

- `m(g)` の固定 polarity（「case 内 → 0」でも「case 内 → 1」でも）正典にできない。正典規則は faithful parse:
  live `gg.js` から `var o` の初期値と switch の case 代入値を読み、`M(value)` = case 内なら代入値、case 外なら初期値。
- mapping 方向は固定で正しい: `m = 0 → w1` / `m = 1 → w2`、TN subdomain `('a'+m)tn`（`atn`/`btn`）。
  TN は両 host が同一物を返す（interchangeable・実測 both 200）。
- `tools/gen_ggset.py`（正典生成）は snapshot に観測 polarity（初期 o と case 代入）を記録するよう更新が必要。
- 実装は本項目の承認範囲（polarity 反転）と異なる設計変更のため、ユーザー判断を要する（未実施）。

## 実測（2026-11-03・fresh live）

1. 11:38Z 取得 live `gg.js`（bucket `1793703601/`・cases 1984）:
   `m: function(g) { var o = 0; switch (g) { case 1284: ... case 1736: o = 1; break; } return o; }`
   → cases 内 value は `1`、case 外は `0`。現行 `app/gg.go` `GG.M()`（case → 1）と現行 fixture はこの窓と一致。
   decision `2026-11-01-ggset-generation-and-drift.md` line 15（cases 内 → 0 固定）とは矛盾。
2. 窓 data 実測（gallery 4301695・50 files・11:00窓）: in-case hash は `w2`=200（`w1`=404）、out-of-case hash は `w1`=200（`w2`=404）
   （`docs/ai/tmp/assess-20261103-1220/polarity_probe8_out.txt`）。TN は in/out 問わず `atn`・`btn` 双方 200（p4）。
3. 同一 hash `e3ee8d44474c...735fd9`（g=2557）: 10:0x 窓（bucket `1793700002/`・cases 2112・`var o = 1`／case → `o = 0`）で `w1`=200／`w2`=404（A-10 実測）。12:05Z 窓（epoch 1793707505・bucket `1793707201/`・`var o = 1`／case → `o = 0`）では live 側 `w1` で 200。
   → 窓ごとに構造（初期 o と case 代入）が反転。mapping 方向は固定。
4. `var o = 1` 窓での現行 code 実測（12:0xZ・canonical exe 実機）: CLI download gallery 4301695 → `attempted=50 written=0`（全 404。code が組む host は live 側と逆）。viewer `/api/image/<hash>?format=webp` → `404` `x-cache: no_full`。
   → 現行 code は `var o = 0` 窓でのみ正しく、`var o = 1` 窓では live と食い違う。

## 正・誤の判定

- 正: mapping 方向（0 → `w1`/`atn`、1 → `w2`/`btn`）と「live 規則への追随」。規則そのものは faithful parse でのみ满足される。
- 誤（supersede 対象）: `2026-11-01-ggset-generation-and-drift.md` line 15 の「case に含まれる value は 0」を固定規則とする主張。これは 11-01 10:0x 頃の観測（`var o = 1` 窓）に由来し、11-01 10:23 観測（`var o = 0`・archive KI `2026-11-01-parsegg-switch-value-inversion.md`）と 11-03 実測で構造が窓ごとに rotation することが確認された。line 15 の mapping 方向の記述は有効。
- 誤（現行 code）: `app/gg.go` `GG.M()` と `app/ggset.go` `ggM()` の case→1 固定、`tools/gen_ggset.py` の固定生成、fixture の「live 構造」単一固定（11-01 10:23 時点の構造を正典のように記述）。ただし窓 rotation のため「反転是正（case→0 固定）」でも同じ失敗を半分の窓で繰り返す。本項目の承認 premise（fresh live 実測が decision 側と一致）は 11:38Z 実測で不成立。

## 次の対応（承認待ち）

- `parseGG()` に初期 o と case 代入の保持、`M()` の faithful 化、`gen_ggset.py` の観測 polarity 生成、fixture を規則基準（fixture 構造に従う assertion）へ更新。
- A-10 の live 実測（attempted==written）は `var o = 0` 窓で成立し `var o = 1` 窓で 404。faithful parse 導入まで A-10 は窓依存で不安定（A-10 側で再測）。

関連: `docs/ai/known-issues/2026-11-03-gg-polarity-inverted-vs-live-ggjs.md`、`docs/ai/tasks/2026-11-03-a10-download-existing-check.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`、`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`（作業前チェック読了済み）。
