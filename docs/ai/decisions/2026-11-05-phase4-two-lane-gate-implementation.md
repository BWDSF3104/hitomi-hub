# Phase 4 — サーバー側ゲート優先度化（2レーン化）の実装決定

**日付**: 2026-11-05
**項目**: `2026-11-05-hitomi-hub-search-phase4-gate-priority`（実行固有ID `assess-20261105-0024`）
**supersede**: なし（単一タイムスタンプゲートの既存意味（系統間隔・FIFO・global interval 遵守）は維持し、レーン追加のみ）

## 判定（手順1 実測で閉じる）

正典 server（port 8731・root `p3_canonical_fresh`・canonical build）＋実 Phase 2 scheduler（並行度 2・`priority=background` marker）で測定。

- 基線（gate idle・server-direct 3 反復 median）: A（data search）265ms（baseline-A2）／B（galleryinfo）180ms／C（image lane full）679ms（TN median 653ms）
- Scenario A（有効3反復・run4 fresh tab・prefetch card phase 中・marker 276→284 増分確認）: 2443 / 2992 / 2990ms → median 増加 +2725ms（data 1 枠=1000ms の約 2.7 枠）→ **超**（再現性あり）
- Scenario B（有効3反復・run4・未 cache gallery  fetch）: 3010 / 3000 / 3010ms（ids 4290425 / 2923055 / 1070891）→ median 増加 +2830ms（約 2.8 枠）→ **超**（補助: run3 russian id +2841ms、run1 934/1005/1048ms）
- Scenario C（有効3反復・run9・italian・prefetch cover window 中・marker 339→374・x-cache miss・network）: 1398 / 715 / 702ms → wall median 増加は +36ms（1 枠=250ms 未満）だが、per-sample gate 成分は推定 400–900ms（>1 枠・3/3）。**推定は確認と区別**: wall median 単独では C は基準未超過、gate 成分（in-flight 2 の FIFO 連鎖待ち）で >1 枠。判定は A/B の median 超過で確定（実施基準「いずれかのシナリオの 1 枠超の再現性（反復3回以上・median）」充足）。

実施不要分岐（全シナリオ ≤1 枠）には該当せず → **手順2（2レーン化）を実装**。

## 実装設計（評価記録推奨案を閉じた形）

- `Client` に `lastFg` / `lastBg` / `fgPending` / `bgWaitSince` を追加。共有 `last` 連鎖は維持（系統間隔の global 遵守＝FIFO 算術は基線設計のまま）。
- `gateLane(ctx, intervalMS, last, background)`（`gateForInterval` の置き換え）:
  - foreground（`background=false`）: 現行 FIFO 将来予約のまま（`last` .advance・`lastFg` 記録・`fgPending` 増減）。
  - background: **今この瞬間のみ予約**（未来予約禁止。フォア遅延 ≤1 枠の根拠）。条件: `fgPending == 0` かつ `last + interval <= now` かつ `lastFg + grace <= now`。不公平飢餓防止: bg 待機が 3×interval 以上継続し、かつ interval 条件を満たす時点で公平枠として予約可（将来予約にはしない）。
  - `bgGraceMS` 既定 300ms（config `bg_grace_ms`）。テスト override は config 値直接設定。
  - `testGateIntervalMS`（C-10）inject 口・metrics 記録（kind 別意味・`marker.<priority>`）・A-06 ctx sleep（sleep 中 cancel 即 return）は非接触維持。
- マーカー伝達（正典）: handler の `priority`（query param・実測 no-op）から client 呼出へ明示引数 `background bool` を渡す。
  - `get` / `getImage` / `fetchRemoteWithGate` / `gateLane` に引数追加。
  - `SearchHTML(..., background)` → `searchState.background` フィールドで `termIDs` → `SearchNozomiParts` へ伝搬（`context.Background()` 内部使用の gap 解消）。
  - `IndexIDs(..., background)`、`Gallery(id, background)`、viewer の `info(id, background)` / `searchResult(id, background)` / `fetchFullImageOnce(..., background)` / `thumbProxy` / `imageProxy`。
- 固定テスト（`hitomi_test.go`）: ①フォア待機（`fgPending>0`）中にバックは slot 予約しない（`lastBg` ゼロ・待機）②フォア不在時バックは現行 interval で進む（飢餓なし）③bg の ctx キャンセル即 return（A-06 意味）④grace（lastFg+300ms）尊重。既存 gate テストの `c.last` 直接参照は共有連鎖意味のまま re-anchor 可。

## 非接触（ガードテスト維持）

- `TestStorageURLUsesFormatNotNameExt` / `TestCanonicalGGRuleComposition` / `TestThumbnailURLMatchesVerifiedTNPath`・viewer imageProxy 順序（local full → persistent full cache → remote → no_full）、TN display fallback 禁止、TN バイトを full cache に保存しない。
- metrics kind 分類（`classifyFetchURL`）・marker kind 分離（Phase 2 正典）は変更しない。

## 実測ファイル（スクラッチ・コミット禁止）

`docs/ai/tmp/phase4_results.jsonl`（基線）、`phase4_loadrun_ab.json`（run1）、`phase4_loadrun_ab2.json`（run2）、`phase4_loadrun_ab3.json`（run3）、`phase4_loadrun_ab4.json`（run4 A/B）、`phase4_loadrun_c1.json`（window 後・無効）、`phase4_run5_probe.json`／`phase4_run5_ids.json`、`phase4_loadrun_c2.json`（miss）、`phase4_run8_probe.json`、`phase4_run9_probe.json`、`phase4_loadrun_c3.json`（miss）、`phase4_loadrun_c4.json`（run9 C 有効）、`phase4_b_displaycheck.json`（UI display pass）。UI 確認 screenshot: `docs/ai/tmp/drive/confirmation-A-run5-cardphase.png`（実 UI search 実行中）、`confirmation-B-C-run5-load.png`（load 中 gallery UI）、`confirmation-C-display-pass.png`（display pass・naturalWidth 1059×1500 実 pixels・TN 25,564B ≠ display・objectURL）。
