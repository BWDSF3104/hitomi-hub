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

## 2026-11-09 再調査（assess-20261109-1320）

### 現行実装と判断根拠

- 機構は現行コードそのままに残っている（修正されていない）。現位置は `app/viewer.go:1239-1308`（`search` ハンドラ）、後付け置換は `app/viewer.go:1301`。本エントリの行番号 568-595 は古い（ファイル行数増加によるずれ・記録上の outdated）。
- 現行 `search` payload の文字列値は `query`・`lang` のみ（`results` は常に空配列、`ids` は int）。`metrics` の key は `classifyFetchURL` の固定 kind（version・index・nozomi・gg・data・gallery・image）＋ `marker.<priority>`。

### 有効性（実測で確定）

- 機構：有効（bytes.Replace 後付けは現行使用、`app/viewer.go:1301`）。
- 2026-11-06 原文の前提「tag・title・detail の文字列に `"json_ms":0` が現れると誤置換」は**無効**。JSON string encode は `"` を必ず `\"` に escape するため、string value 内には raw 文字列 `"json_ms":0` が成立しない（実測 case A：query に該当文字列を投入しても escape され、置換は timing 正位置に正しく当たった）。この条項を本再調査で supersede する。
- 誤置換が成立しうる境界は「key 名が正確に `json_ms` で timing より前（alphabetical で metrics が timing より先）に出る」場合のみ。実測 case C（ hypothetical key collision）では確かに metrics 側が先に置換され timing が 0 のまま残った。ただし**現行ハンドラにはその key を生む経路がない**（kind は固定集合、`priority=json_ms` でも key は `marker.json_ms` となり pattern に当たらない — 実測 case B）。よって現状の実データで誤置換は成立不能。
- 総合：脆さの機構は有効、発現条件は現行経路に存在せず、原文の発現条件（string 内容）は JSON escape で否定。残るリスクは将来の payload 構造変更（json_ms key の衝突）への防衛だけ。

### 対応の実現可能性・推奨修正

- 推奨：後付け文字列置換をやめ構造化する。合格条件の維持要件（`timing.json_ms` 正値）があるため header のみ案は除外。最小実装は 2 pass 再 encode（payload → encode 実測 → `jsonMS` を実値として encode し直す）で、json_ms の意味（encode 実測）を保つ。または timing を encode 後に組み立てる構造（json key 順は JSON 消費側で問題にならない）。
- テスト固定：現テスト（`app/viewer_test.go:1436` 以降）は正値のみで、置換位置の正しさ（header の `json` と `timing.json_ms` の一致）を固定していない。一致 assertion を追加すれば将来の key 衝突をテストで捕捉できる。これが最小の実効対策。

### 副作用・影響範囲・代替案

- UI 依存は `x-search-timing` header 側（`#perf-status`）。JSON の `timing` はテスト・観測形で、A-03 で二重 metrics 除去済みの JSON shape 変更を要求する UI 依存はない（2 pass 再 encode で意味変化なし）。
- 代替案（header のみ）は JSON shape 変更で合格条件と衝突するため非推奨。

### 実測（スクラッチ・正典流儀）

- 再現プログラム: `docs/ai/tmp/assess-20261109-1320/ki20261106-03.go`（`go run`・stdlib のみ）。結果:
  - case A（query = `json_ms":0`）: encode 後 raw 出現は timing 直下のみ（index 183）、置換正位置・timing.json_ms=1。string 内容誤置換は成立せず。
  - case B（metrics key `marker.json_ms`）: 衝突なし、置換正位置。
  - case C（metrics key が exactly `json_ms`）: 最初の出現が index 36（metrics 内）→ metrics だけ置換され timing.json_ms=0 のまま。誤置換成立の最小ケース（現行ハンドラでは生成不能な key）。

### 作業前チェック適用記録

- 読了（文件名明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。
- 本再調査は read-only（コード変更なし・変更は本記録のみ）。正典の「変更後」検証条（`go vet . && go test -count=1 -timeout 300s .`）は適用対象外（docs のみの変更は検証不要の共通条）。実測は上記スクラッチ go run の出力で固定。
- UI 画面確認は該当なし（UI 項目ではない）。
