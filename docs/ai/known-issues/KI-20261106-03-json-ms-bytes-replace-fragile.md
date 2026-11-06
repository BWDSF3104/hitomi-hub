# R-21（低優先）: search ハンドラの json_ms を bytes.Replace で後付けしている

**Status**: 未解決（壊れやすい実装・判定は読み取り）
**Date**: 2026-11-06
**Source**: 外部指摘（Claude）2026-11-06 補足 → `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 読み取りで確認できた機構

- `timing` を encode してから、`buf` 内の `"json_ms":0` を `bytes.Replace(data, ..., 1)` で測定値へ書き換えている（`app/viewer.go:568-595`）。
- 現在の実装は `jsonMS == 0` 時に 1 へ引き上げるため、`payload` の他の箇所（tag・title・detail）に `"json_ms":0` という文字列が現れると、最初の出現が置き換えられる。置き換え対象が timing 以外になる可能性がある。
- 現テストは header と timing の正値のみ照合（`app/viewer_test.go:1375-1398`）。この壊れ方はテストで固定されていない。

## 影響

実データでの誤置き換えは未確認（gallery の tag/title に `"json_ms":0` が出現する条件は実例なし）。計測値を header だけにする、または 2 パス目で組み直す方が安全。

## 次の対応（未実施）

- 実装承認が必要。変更する場合、既存の `x-search-timing` header と `timing.json_ms` 正値の照合テストを維持する。
