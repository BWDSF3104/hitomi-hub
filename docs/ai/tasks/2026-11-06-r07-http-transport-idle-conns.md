# R-07: http.Client が default Transport（MaxIdleConnsPerHost 2）で TLS 再接続が繰り返される

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: 8 並行 prefetch/download で接続Reuseが確認できる（MaxIdleConnsPerHost を 16 程度へ）。gate semantics・fetch 口（テスト inject）非触。記録: R-07

## 意図 / 背景

- 指摘: `http.Client` は `Timeout` だけ設定し Transport を指定していない。Go の default Transport の `MaxIdleConnsPerHost` は 2（`DefaultMaxIdleConnsPerHost`）で、8 並行の prefetch・download では並行数が idle 上限を超えたぶん再接続（TLS ハンドシェイク反復）になる。16 へ上げる案。
- 既存読み取り記録（`docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` R-07 欄）は「機構確定・改善幅は推定」。今回の実測で改善幅を数値で確定し、機構が現行条件で実際に発火するかを切り分ける。
- 正典条件: gate semantics（`docs/ai/decisions/2026-11-05-rate-values-and-concurrency-change-approved.md` の lane interval・politeness 退避）と fetch 口（テスト inject）は変更対象外。既存テストは変更しない。

## 調査結果 / 変更内容

### 現在の実装

- Client 構築口は 5 箇所。いずれも `Transport` 未指定（`app/main.go:1047`、`app/main.go:1095`、`app/main.go:1112`、`app/main.go:1226`、`app/viewer.go:86`）→ `http.DefaultTransport` を共有する。
- 呼出しは `app/hitomi.go:521`（`fetchRemoteWithGate` 内の `c.http.Do(req)`）。gate は `app/hitomi.go:495`（`gateLane`）で `Do` の前。interval 現値: data 100 ms・image 60 ms（`app/hitomi.go:238`）。R-11 の承認値（data 90 ms）は未実施（本項目では扱わない）。
- reuse 可能条件は満たしている: response body を `readAll` で読み切った直後に `resp.Body.Close()`（`app/hitomi.go:536-538`）しており、status 判定より前。よって 2xx/非2xx かわらず connection は idle pool に戻される。
- 並行数: viewer prefetch `PREFETCH_CONCURRENCY = 8`（`app/viewer.html:207`）、download worker pool（`app/main.go:712-748`）既定 8（`app/main.go:1134`）。テスト call site は 1・4（`app/hitomi_test.go:1101`、`1163`、`1175`、`1222`、`1287`、`1314`）。
- VT 照会は `http.DefaultClient`（`app/vt.go:40`）を使っており、この Client の Transport とは別。単発呼出しで並行なし。global `http.DefaultTransport` を書き換える実装は VT と既存テスト（`srv.Client()`）へ波及するため、その方式は採用しない。

### 有効性: 一部有効（機構は実在・現行 gate 条件での改善幅は 0）

- `MaxIdleConnsPerHost` は「同時に idle である接続数」に対する上限であり、in-flight（要求実行中）の接続数には効かない。idle 上限が効くのは、同一 host の idle 接続が 3 本以上同時に揃ったときだけ（超えた分の close）。
- よって「並行数 8 > idle 2 → 毎回再接続」という指摘の因果は成立しない。要求遅延（image ≈ 700 ms、data ≈ 425 ms）が interval（image 60 ms・data 90–100 ms）より大きい現行条件では、worker は接続を持続的に使用中で、idle pool が 3 本を超える瞬間がほぼ生じない。
- 実測（local mock・`httptest` なし equivalent: `net.Listen` + `http.Server`、TLS なし、`docs/ai/tmp/assess-20261106-1056/main.go`）:

| 条件（8 並行・120 要求） | default（idle 0 = 実効 2） | 明示 16 |
|---|---|---|
| image lane（gate 60 ms・latency 700 ms） | 8 接続・120/120 reuse | 8 接続・同一 |
| data lane（gate 90 ms・latency 425 ms） | 5 接続 | 5 接続（in-flight 5） |
| 12 並行 + gate 60 ms | 12 接続 | 12 接続 |
| gate が支配（interval 200 ms > latency 60 ms） | 1 接続（in-flight 1） | 1 接続 |
| burst（gate なし・worker 同時完結） | 18・30・41・47・49・53（試行により変動・中央 ≈ 40） | 8（全試行） |

- gate がある定常経路では改善 0。改善は「完結が位相同調する」「一時停止で idle が同時に 3 本以上揃う」条件でのみ出た（burst: 40 本 → 8 本、120 要求あたり 32 本分の確立回避）。
- 遅延差（TLS 相当の注入遅延 340 ms・gate なし burst・56 要求）: 18 接続 → 8 接続、wall 5925 ms → 5244 ms（-681 ms・-11.5%）。gate 定常条件（60 ms）では両設定で wall 5668 ms・5668 ms（差 0）。
- local mock には TLS がないため、実害（ハンドシェイク遅延）は機構（接続確立回数）と外部環境での期待値を分けて記録する。外部ハンドシェイク実測値は 322–360 ms/接続（`docs/ai/tasks/2026-11-05-assess-20261105-1015-rate-value-centralization-floor.md:23`、handshake 分離計測）。burst 条件での回避期待値: 32 本 × 322–360 ms ≈ 10.3–11.5 s（アグリゲート。wall への反映は並行 overlap で縮小し、mock 実測 -11.5% 相当）。
- 限定条件: `IdleConnTimeout` 90 s（`http.DefaultTransport`）で、90 s 超の停止は idle 上限に関係なく切断される。改善は「短い隙間・同調完結」の範囲に限られる。
- h2 注記: remote URL は https（`app/hitomi.go:655`、`707`、`799`）。server が h2 を_negotiate_ すれば 1 接続で多重化され、`MaxIdleConnsPerHost` の意味は stream 単位では消える（h1 の shard 経路に限って本項目の機構が効く）。mock 実測は h1 セマンティクス。

### 対応の実現可能性と適切な修正方法（推奨）

- 推奨案: helper `newPooledTransport()` を 1 本置き、`http.DefaultTransport.Clone()` に `MaxIdleConnsPerHost = 16` だけ設定し、5 構築口（`app/main.go:1047`、`1095`、`1112`、`1226`、`app/viewer.go:86`）で `Transport: newPooledTransport()` を指定する。
  - `Clone()` を使う理由: `IdleConnTimeout` 90 s・`TLSHandshakeTimeout`・`ForceAttemptHTTP2`・`ProxyFromEnvironment` などの既定値を保持し、変更は 1 値だけ。global `http.DefaultTransport` を直接書き換えると VT（`app/vt.go:40`）と既存テスト（`srv.Client()`）へ波及するため不可。
  - gate semantics・`fetch` inject 口（`app/hitomi.go:474`）は経路が別で、テスト inject（`c.http = srv.Client()`）にも影響しない。既存テストはこの挙動を固定していない（照合済み）。
- 16 の根拠: 現行並行 8 に対して headroom。意味は「並行数を下限として保持する」であり、16 であること自体は機構上不要（8 で同等）。ただし現値 2 との比較で改善幅が確定するのは burst 条件のみ。
- 実装規模: helper 1 本 + call site 5 箇所の追加（約 12 行）。UI・DB・schema・gate・fetch 口は変更なし。合格条件テスト（例: helper の `MaxIdleConnsPerHost >= 16`）は新規追加で既存テスト変更なし。

### 副作用・影響範囲・代替案

- 副作用: 保持 socket 数が増える（host ごとに最大 16）。Windows の fd・メモリへの影響は実務上無視できる程度。アイドル接続保持時間が延びる（server 側 keepalive 切断との競争）程度で、ユーザーに見える変化はない。
- 影響範囲: `app/main.go`・`app/viewer.go` の Client 構築のみ。gate interval・politeness 退避・block cooldown・retry は無変更。`config.example.json`・docs の数値写し（`docs/ai/api-rate-limits.md`）は項目意味上無関係（追加しない）。
- 代替案 A（採用しない）: `http.DefaultTransport` を直接変更。global波及・テスト inject 口影響・VT 経路波及。
- 代替案 B: 16 ではなく並行度（8）へ。意味は同じで保持 socket が小さい。値の単一出力元化（`defaultHitomiConfig()` 流儀）に寄せるなら config キー化する案もあるが、gate とちがい性能値なので既定固定で十分。
- 代替案 C（現行条件では妥当）: 変更しない。実測で定常経路の改善幅 0。「8 並行 → 毎回再接続」の指摘は gate 条件下で成立せず、優先順位は下げるべき（findings の優先実装候補 4 位は実測で降格）。実施するなら「burst・pause 条件での確立回数削減」として位置づける。

## Verification

- 実測コマンド（スクラッチ・本番コード変更なし）: `docs/ai/tmp/assess-20261106-1056/main.go`（`httptest` ではなく `net.Listen` + `http.Server` で accept 回数を数える counting listener、gate モデルは FIFO 予約、TLS 相当の遅延は dial 注入）。実行: `go run main.go`（workdir `docs/ai/tmp/assess-20261106-1056`）
  - gate 定常（8 並行・120 要求）: default 8 接続 = 明示 16 8 接続（改善 0）。data lane 5 = 5。12 並行 12 = 12。gate 支配条件 1 = 1。
  - burst（gate なし・同調完結）: default 18–53（6 試行・中央 ≈ 40）→ 明示 16 は 8（全試行）。
  - burst + handshake 注入 340 ms（56 要求）: 18 → 8 接続、wall 5925 ms → 5244 ms（-11.5%）。gate 定常 + 注入: wall 5668 ms = 5668 ms（差 0）。
  - burst を 5 回試行した結果（120 要求）: 49 / 41 / 30 / 47 / 53。明示 16 は毎回 8。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet 出力なし（clean）、`ok hitomi-hub/app 26.161s`（2026-11-06 実測）。コード変更なしで既存テスト全 55 件の Go 側は影響なし。
- `python -m pytest tests/ -v`: 54 passed・10.06 s。1 件失敗 `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md の pointer 条検査）。これは本項目と無関係の既存失敗（本項目でファイル変更なし）。スコープ外なので `known-issues/` への記録はこの記録のみ（書き込み許可範囲が個別タスク記録に限られるため separate entry は作っていない）。
- 照合: gate semantics・`fetch` inject 口を変更していない。既存テストが default Transport の挙動を固定していないことを grep で確認（テストは `c.http = srv.Client()`（`app/hitomi_test.go:913`、`1429`、`1470`、`1515`、`1583`）または Transport 未指定 client（`app/hitomi_test.go:581`、`1270`））。既存テストは変更していない。
- 画面確認: UI 変更なし → 該当なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
