# R-07: http.Client が default Transport（MaxIdleConnsPerHost 2）で TLS 再接続が繰り返される

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実装: helper newPooledTransport（Clone+MaxIdleConnsPerHost=16）を hitomi.go に追加・5 構築口へ適用・新規テスト追加（既存テスト変更なし）

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

### 実施（2026-11-08・実作業）

- 推奨案どおり実装。helper `newPooledTransport()`（`const pooledMaxIdleConnsPerHost = 16`）を `app/hitomi.go:291` に追加（`http.DefaultTransport` は直接変更せず `Clone()` + 1 値のみ）。
- 5 構築口へ適用（現在行番号: `app/main.go:1168`、`1216`、`1233`、`1347`、`app/viewer.go:128`）。調査時の行番号（1047・1095・1112・1226・86）は R-11b・R-15 等の追加でシフトしたが、内容上同一の 5 箇所（乖離・実装不適用の持ち帰りは不要と判断）。
- 既存テスト変更なし。合格条件テストを新規ファイル `app/transport_test.go` に追加（3 test）。
- gate semantics・fetch 口（テスト inject）・R-13 dedup・R-15 cache_entries 非触。記録の data lane「100 ms」記述は R-11b（`a9f7786`・正典 90 ms）で supersede 済み（`defaultHitomiConfig()` 現値 90）。矛盾規則により新しい正典を優先、本項目では gate に触れない。

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

### 実装検証（2026-11-08・実作業）

- 作業前チェック（フル画像・viewer・download 経路に触れるため正典要求）: 事前読了をタスク記録に明記 — `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。viewer・download 経路は Client 構築口（Transport）のみ追加で、full 順序・TN 表示源禁止・gate・fetch 口・cache write 機構は無変更。
- 新規テスト（mock 計上・8 並行接続Reuse確認）`app/transport_test.go`:
  - `TestPooledTransportSetsIdlePerHost16AndKeepsDefaultValues` PASS: helper `MaxIdleConnsPerHost = 16`、`http.DefaultTransport`（フィールド 0 = 実効 2）を直接変更していない（VT・テスト inject 波及回避）、clone が IdleConnTimeout 90 s・TLSHandshakeTimeout・ForceAttemptHTTP2・ProxyFromEnvironment を保持。
  - `TestPooledTransportReuses8ConnectionsUnder8ParallelBurst` PASS（8 並行・round barrier 同調・40 要求・counting listener）: helper = 確立 8（全 request reuse）、既存 default = 確立 23（round boundary で idle pool 3 本超 → 6 本 close → 再接続）。gate なし burst 条件での reuse を mock 計上で確定。
  - `TestViewerUsesPooledTransportAndFetchInjectPathUntouched` PASS: viewer 構築口（`newViewer`）が pooled transport、`fetch` inject 口 default nil。
  - 修正・再検証 1 回: 初回 run で `DefaultTransport MaxIdleConnsPerHost == 2` assert が失敗（実フィールドは 0 = 実効 2）。0 のまま不動を assert するよう修正し PASS。原因は「正典は実効値 2・フィールド値 0」の区別。
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet clean、`ok hitomi-hub/app 26.066s`（2026-11-08 実測）。
- 接続 reuse 実測（スクラッチ `docs/ai/tmp/assess-20261106-1056/main.go` 再作成・`go run main.go`・counting listener + FIFO gate + dial 注入 handshake 340 ms）:
  - gate 定常（8 並行・120 要求・gate 60 ms・handler 700 ms）: default 8 接続 = helper 8 接続（改善 0・wall 10.931 s vs 10.927 s）。
  - burst（gate なし・同調完結・120 要求・5 試行）: default 51 / 65 / 53 / 39 / 54（中央 ≈ 52・先行記録の 18–53 と同帯）、helper 16 は毎回 8。
  - burst + handshake 注入 340 ms・56 要求: 26 接続 → 8 接続、wall 1385 ms → 1046 ms（−339 ms・−24.5%）。gate 定常条件は両設定差 0。先行実測（burst ≈40・handshake 注入 wall −11.5%）と同じ方向。
- 正典 build 実機確認（CGO_ENABLED=0 `output/hitomi-hub/hub.exe`・外部 API 呼び出し方針 `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`）:
  - CLI download（実在 gallery 4294614・`--limit 3`・scratch root livecheck2・`--no-vt`）: attempted 3 = written 3（skipped 0）。
  - canonical server（port 8731・scratch root livecheck2・data-dir プロジェクト root）: `/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=webp` → 200・`x-cache: miss`（210784 bytes・cache write 後 `cache`）、`/api/image/4d16cd74bbceb4d1ff7bd3ac95ae6586cb06e8b877e2c5b1c205b98d807769ad?format=webp` → 200・`x-cache: cache`（224248 bytes）。`no_full` 出ず。
  - 注記（pwsh 計測アーティファクト）: 初回 `Invoke-WebRequest` の `"$h?format=webp"` 二重引用符内補間が三項演算子として解釈され 400（server の hash 検証通過後、URL 破損が原因）。curl で 200 を確定（server 挙動ではない）。curl 経由の results が実測値。
  - 注記（既存環境・本項目非触）: `output/hitomi-hub/schema.sql` コピーが stale（`resolved_at` 列なし）で download 時 stderr に `scan: SQL logic error: no such column: resolved_at` が出ることがある。プロジェクト root の現 schema.sql（R-17・line 55）を data-dir とすると出ない。既存 staleness（canonical build freshness 検査の趣旨内）で本項目の不合格ではない。スコープ外のため本記録のみ（separate entry なし）。
- `python -m pytest tests/ -v`（本項目 Python 変更なし）: 1 failed・59 passed（2026-11-08）。既存失敗 `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 条検査・本項目と無関係）。
- 照合: gate interval・politeness・FIFO・将来予約・grace・lane 直列化・fetch inject 口・R-13 dedup・R-15 cache_entries 変更なし。VT 経路（`http.DefaultClient`）は helper を使わない。既存テスト無変更。
- 画面確認: UI 変更なし → 該当なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
