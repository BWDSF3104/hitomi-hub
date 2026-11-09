# R-21（低優先）: search ハンドラの json_ms を bytes.Replace で後付けしている

**Status**: 解決済み（2026-11-10 実装・検証合格・案1 構造化注入）
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

## 2026-11-10 実装（項目 #4・assess-20261109-1320・案1 構造化注入）

### 採用した判断（段階1・ユーザー同意済み 2026-11-09）

- 案1（構造化注入・2 pass 再 encode）採用。後付け `bytes.Replace` を削除し、encode 実測値を timing map 側で組み立てて再 encode。`json_ms` の意味（encode 実測・正値・header `json` と同一測定値）不変。防御強化のみ（脆さをテスト pin で残す案）は不採用（同意済み）。
- 正典ポインタの食い違い（記録・実測で閉）: 委譲文は `app/hitomi.go:185-187` を指すが、本エントリ正典と実コードは `app/viewer.go` の `search` ハンドラ（現 1244-1313・置換は 1306）。正典＋実コードを正として app/viewer.go を修正。`app/hitomi_test.go` に json_ms pin なし（実測確認・更新なし）。

### 変更

- `app/viewer.go`（search ハンドラ）: 後付け置換 → 構造化注入（timing map 参照・placeholder 0 → 実測 jsonMS → 再 encode）。gate・singleflight・x-cache・TN・full 経路・`validHash`・JSON 出力の意味（key 順・shape・newline）不変。
- `app/viewer_test.go`（`TestSearchHandlerIncludesMeasurementShape`）: 既存 pin（timing 各 field・正値・metrics 非重複・header 相位）維持＋追加 pin「`timing.json_ms` と header `json` 値の一致」（本節 38 行の最小の実効対策・実効対策の同意済み pin 更新）。後付け置換の復活や key 衝突による誤置換は一致崩れとして捕捉。

### 合格条件の証拠

- 決定論確認（scratch・正典流儀・mock・外部 API なし・stdlib）: `docs/ai/tmp/assess-20261109-1320/ki20261106-03-structured.go`（`go run`）。
  - case A（query に raw pattern → escape）: 旧・新出力 bytes 完全一致・header==timing 一致（json_ms=1）。
  - case B（key `marker.json_ms`）: 一致・衝突なし。
  - case C（hypothetical key 衝突・値数値 0）: 旧は metrics 誤置換で timing=0（header==timing 崩れ）、新は正位置（json_ms=1・metrics 値そのまま）。構造化で衝突経路が構造的に消えることを実測。
  - case D（key `json_ms`・値 metric map＝実ハンドラの値型）: 旧・新 bytes 完全一致。既存 pin の意味（json_ms 固定）が保たれることの確認。
- 正典検証: `go vet .` clean・`go test -count=1 -timeout 300s .` ok（約27s・更新 pin 通過・`-count=5` 反復 5/5 安定）。`python -m pytest tests/ -v` 65 passed（canonical build freshness 含む・正典 hub.exe を現 app から fresh build 後）。
- 画面確認: 該当なし（UI 項目でない）→ 実施なし・Drive 保存なし（未検証扱いではない）。

### 作業前チェック適用記録（実装分）

- 読了（本エントリ 54 行で明記済みの正典を維持参照）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。本改修は full 画像経路・TN・gate・singleflight に非接触。
