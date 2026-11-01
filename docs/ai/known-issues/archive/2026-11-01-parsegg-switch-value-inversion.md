# parseGG が live gg.js の switch 値を逆に解釈する

**Date**: 2026-11-01
**Status**: 解決済み（2026-11-01・`parseGG()` / `gen_ggset.py` を live `gg.js` の case→1 / case外→0 構造に修正）

## 症状

- Cache-less actual screen で `4252697` の full image が `x-cache: no_full` / `naturalWidth=0` になる。
- 同じ hash の direct live full URL は `Referer: https://hitomi.la/` を付けると `200` で取れる。
- app が生成する shard URL は 404 になる。

## 実測

- Fresh cache-less server で `/api/image/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=avif` は `404`, `x-cache: no_full`。
- 同一 hash の `gg.s(hash)` は `2335`。
- Playwright direct request（app UA + `Referer: https://hitomi.la/`）:
  - `https://a1.gold-usergeneratedcontent.net/1793527201/2335/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.avif` → `200`
  - `https://w1.gold-usergeneratedcontent.net/1793527201/2335/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.webp` → `200`
  - `https://a2.gold-usergeneratedcontent.net/1793527201/2335/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.avif` → `404`
  - `https://w2.gold-usergeneratedcontent.net/1793527201/2335/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.webp` → `404`
- 2026-11-01 10:23Z の live `gg.js` 構造:
  - `var gg={ m: function(g){ var o=0; switch(g){ case ...; o=1; break; } return o; }, s: ..., b:'1793527201/' }`
  - `case 2335` は live の switch にない。
  - この live 構造では `case` に無い value は `m(g)=0`。
  - したがって `m(2335)=0` → `a1` / `w1` の URLが正しく、`a1` / `w1` は `200`。

## 原因

- `app/gg.go` と `tools/gen_ggset.py` は `gg.m(g)` を「switch にある value で 0、それ以外で 1」と解釈している。
- 現在の live `gg.js` は「`var o=0`、switch の case を通ると `o=1`」で、switch にある value は `1`、switch に無い value は `0` を返す。
- `parseGG()` は `case` 値を `ZeroSet` に入れ、`M(value)` は `ZeroSet` にある value を `0`、それ以外を `1` と返す。
- `value=2335` は live `gg.js` の switch に無いので live `m(2335)=0` だが、app は `M(2335)=1` と返し、`a2` / `w2` を生成して 404 になる。

## 影響

- Full image URL の subdomain が逆になる。
- Cache-less viewer display の actual pixels 検証で、remote full data が存在しても `no_full` として失敗する。
- 既存テストは fixture で「cases → w1 / m=0」を仮定しているため、live の実際の switch 構造との矛盾を検出不できない。
- `gen_ggset.py` も同じ誤った構造前提で生成するため、snapshot も live の `m()` 規則と逆になる可能性がある。

## 関連

- `app/gg.go`
- `app/hitomi.go` の `ImageURL` / `StorageURL`
- `tools/gen_ggset.py`
- `app/hitomi_test.go` の existing fixture assertion
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

## 解決（2026-11-01）

- `app/gg.go` の `GG` を `CaseSet` に変更し、`parseGG()` は live `gg.js` の `case` 値を `CaseSet` へ入れた。
- `GG.M(value)` を live 構造に合わせて `case` 内は `1`、`case` 外は `0` を返すよう修正。
- `tools/gen_ggset.py` の生成規則を live 構造（`var o = 0; switch (g) { ... o = 1; break; }`）に合わせ、`ggCaseSet` と `ggM` を cases→1 / case外→0 で生成するよう修正。
- `app/ggset.go` を正典 `tools/gen_ggset.py` で再生成。
- tests を live 構造 fixture に更新し、viewer tests は fixture GG を使うよう変更。
- cache-less actual screen で remote full `200` / actual full pixels `naturalWidth > 0` を確認。

### Verification

- `go vet ./...` pass.
- `go test .` pass `22.533s`.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` pass.
- Canonical server restarted after rebuild:
  - ID: `bgp_196b07014001jT62QJx8infSQG`
  - URL: `http://127.0.0.1:8731`
  - Command: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root scratch/live`
- Cache-less fixed server actual verification:
  - root: `scratch/hitomi-usage-fixed`
  - URL: `http://127.0.0.1:8735/?x=cp12fixed#/g/4252697/page/1-2`
  - `/api/image/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=avif` returned `200`, `x-cache: miss`, `content-type: image/avif`.
  - Reader `#reader img`: blob `src`, `naturalWidth=3746`, `naturalHeight=2525`.
  - Status at first actual display: `full display cache 1/203 | no full 0/203 | TN display fallback disabled`.
  - Screenshot: `scratch/hitomi-usage-fixed/cp12_fixed_cacheless_actual_full_pixels.png`
