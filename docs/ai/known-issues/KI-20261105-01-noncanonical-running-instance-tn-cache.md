# KI-20261105-01: 正典 server（起動中インスタンス）が `x-cache: tn-cache` を返す観察

**Status**: 未解決（実装前チェック項目として記録）
**Date**: 2026-11-05
**実行固有ID**: `assess-20261105-0024`（P4 調査中の観察。子からの申し受けを受け親が記録。調査実務は再実施していない）

## 観察（P4 記録 `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase4-gate-priority.md` の証拠）

- 正典 server 8731（PID 19316・起動 2026-11-04 23:55 JST）で、`/api/image/<hash>` 応答が `x-cache: tn-cache`（TN バイト 26725）であった。
- 現行 HEAD の `imageProxy` は `x-cache` に `tn-cache` を返せない（local/cache/miss/no_full のみ・`TestImageProxyCacheStateIsNotTN` で固定）。同一 hash の TN は再照会で tn-cache hit せず（no_tn 502・cache ファイルが正典 output ルートに不在）。
- disk ビルドの fresh インスタンス（8732・scratch root）では正しい挙動（imageProxy は miss/no_full・TN は thumbs に永続化）を再現。
- 結論: 起動中インスタンスの in-memory ビルドは正典（HEAD）と保証できない。`TestImageProxyCacheStateIsNotTN` の正典 latest build 条件（canonical build freshness）が実機で担保されていない状態。

## 扱い

- ゲート lane 算術の結論（P4 実測）は HEAD 算術と一致するため影響なし。異常応答自体は TN を full 表示源として扱わないガード違反の内容であり、表示 pass として無効。
- 推奨: Phase 1〜4 の実装・実機確認の前に canonical build を作り直し、正典 server を再起動し、矛盾Observation（C-par2 の `tn-cache`）を再測定すること。
