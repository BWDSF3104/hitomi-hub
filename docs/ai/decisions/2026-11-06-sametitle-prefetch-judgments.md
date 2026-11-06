# 2026-11-06 — 同名日本語版 prefetch: 枚数判定・リアルタイム付与・key・foreground

**実行固有ID**: `assess-20261106-sametitle-prefetch-realtime`
**Status**: 承認済み（ユーザー回答 2026-11-06 03:10 UTC）

## 決定

- 先読み枚数 `6` は「実測判定で確定できる上限」として扱う。現在の `3` fixed 実装は変更しない。
- dynamic運用の既定は `3`、下限 `1`、上限 `6`。
- dynamicの判断材料は、実測1ページ分耗时とユーザー待機時間。サーバー応答速度だけでは速くなりません。
- リアルタイム付与: 成功時の card patch と group cache 更新を許可する。失敗は画面へ出さない。schedulerBlock内は失敗処理だけ。成功patch/cache更新は schedulerBlock 外の helper 経由。
- 同名判定 key は現行 `title || japanese_title` を維持。`japanese_title || title` は非採用。union keyは改善が見込めないため非採用。
- foreground影響: `3`までは正典 foreground ≤1 枠保証で許容可能。`4~6`は block 持続窓と foreground影響の実測後に確定する。

## supersede する決定

- `docs/ai/decisions/2026-11-06-tn-image-lane-60ms-approved.md` の「search page 側の動作は無変更」のうち、`page+1 限定` の点のみ。
- `page+1` 単発ではなく、実測判定で最大 `6` まで ahead pages を許可する方針へ supersede。
- data lane `100 ms`、image lane `60 ms`、foreground ≤1 枠保証、block signal、cooldown、TN display policy、full image policy、TN byte を full cache に保存しない規則は維持。

## 実装状況

- 実装済み: `PREFETCH_AHEAD_PAGES = 3` と `runPrefetch()` の ahead-page loop。
- 未実装: dynamic、実測1ページ分耗时に基づく枚数変更、成功時の card patch、group cache 更新、helper re-anchor。
- 未確認: 実測1ページ分耗时、block 持続窓、card height で anchor が見えるか、`japanese_title` populated事例。
