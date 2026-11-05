# レート値の集約状況確認と短縮可能域の実測（floor 探索）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 項目2 合格（結果セクション記録済み)・項目3の前提充足・残項目3〜4は親の統括で次委譲

## 意図 / 背景

- 前項目（`2026-11-05-assess-20261105-1010-rate-limit-verification.md`）の続柄。レート値が単一の箇所に集約されているか、そして実測上どこまで短縮できるか（floor）を確定する。
- 実装開始指示は受けていない。値変更・コード変更はしない（記録のみ）。

## 調査結果 / 変更内容

### 集約状況（部分集約・1箇所で完結していない）

- 実行時の単一正典は `app/hitomi.go:222`（`defaultHitomiConfig()`）だが、`bg_grace_ms` は `app/hitomi.go:350`（`const defaultBgGraceMS`）が別途保持し、`defaultHitomiConfig()` 内にもリテラル `300` が重複（`app/hitomi.go:223`）。
- `config.example.json` には `min_interval_ms`・`image_min_interval_ms` のみ。`gg_force_min_interval_ms` と `bg_grace_ms` のキーが無い（コード既定と不一致）→ `known-issues/KI-20261105-07-config-example-missing-rate-keys.md`。
- 同一数値の写し（計測・受入条件・方針として）: `docs/ai/api-rate-limits.md`、`docs/ai/usage-memo.md`（2箇所）、`decisions/2026-11-01-hitomi-client-endpoints.md`、`decisions/2026-11-01-image-rate-and-prefetch-concurrency.md`、`decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`、`decisions/2026-11-03-implementation-judgment-approval.md`、`decisions/2026-11-03-remaining-item-recommendations-approved.md`、`decisions/2026-11-03-judgment-waiting-reduction-policy.md`、`backlog-items/p3-10`、`p3-12`、`p3-13`、`p3-17`、テストリテラル（`app/hitomi_test.go:865`、`app/hitomi_test.go:948`、`app/main_test.go:895,907,983`）。
- したがって「値を変えれば全部追随する」構造ではない。変更時に対象は (1) `defaultHitomiConfig()`、(2) `config.example.json`、(3) `api-rate-limits.md`、(4) `usage-memo.md`、(5) 方針系 decisions、(6) 受入条件テスト（`app/hitomi_test.go:865`）の 6 系統。

### 短縮可能域（実測・bounded probe）

- 逐次（1 呼出しずつ）の実測 latency: data ≈ 425 ms/呼出し（handshake 322–360 ms・server 処理 60–100 ms）、image full ≈ 700–900 ms/呼出し。
  → 逐次経路では interval が支配。gate 1000 ms の 7 呼出し検索で gate_wait 6043 ms（`docs/ai/tmp/baseline_search.json`・`p3_baseline_metrics.json`）に対し、250 ms なら gate_wait ≈ 1750 ms（wall 8.48 s → 約 3.0 s）。100 ms なら ≈ 700 ms（wall 約 2.2 s）。
- 並行を上げると latency が支配（throughput = min(concurrency/latency, 1/interval)):
  - data: 並行 4 → 6.6 rps、並行 8 → 12.4 rps、並行 12 → 12.4 rps（全 206・429 なし）
  - image: 並行 4 → 4.5 rps、並行 8 → 8.2 rps、並行 12 → 5.2 rps（全 200・429 なし）
  - 現行 image gate 250 ms（=4 rps）は並行 4 の latency 上限 4.5 rps とほぼ同値 → **image lane は interval ではなく並行度が支配点**。250 ms を 125 ms へ下げても並行 4 のままでは +10% 程度。
- server 側 block の兆候は 100 ms（data・10 呼出し）・25 ms（data・8 呼出し）・並行 12（image・data）まで観測なし。100 ms 未満・長時間持続・累計規模（例: data 25 ms は 2400 呼出し/分）での挙動は未測。

### 実務上の推奨域（判定ではなく案）

- data lane: 250 ms が妥当（逐次検索の実効改善が大きい。100 ms までは実測で通過confirmed、ただし block リスクは未検証域）。
- image lane: 250 ms のまま、あるいは 125–150 ms に短縮し並行度を 8 へ（並行度を上げないなら短縮の効果が薄い）。
- 429 以外の block 形態（403/Cloudflare challenge）は現状リトライ対象外（`app/hitomi.go:532` は 429/5xx のみ）→ interval を下げるなら block 検出の追加が必要。

## 実装実行（assess-20261105-1015・2026-11-05 10:16 UTC）

- goal 開始失敗（2回）→ goal 未使用。コマンド正典 `C:\Users\fujim\.config\kilo\command\implement-items.md` 本文に従い、親は統括のみ・実作業は子へフォアグラウンド委譲（逐次1件）。
- 承認行「直前の内容について承認する。」はメッセージに含まれず。規則どおり直近の assess ID を対象とした（`assess-20261105-1015`＝本タスク）。承認範囲＝本タスク文書に記録済みの「実務上の推奨域」（下記項目1〜4）のみ。
- 項目と優先順位（依存は本記録の記載どおり: 項目3は項目2が前提）:
  1. 値の一元化＋下限ガード（`app/hitomi.go:222` 単一出力元化・`config.example.json` 欠落2キー・`gateLane` 下限ガード）— 対応: `known-issues/KI-20261105-07-config-example-missing-rate-keys.md`
  2. block 検出（403/Cloudflare challenge のクールダウン・`Retry-After` 尊重）— 項目3の前提
  3. 値変更（data 250 ms・image 125 ms・prefetch/download 並行度 8）＋テストリテラル re-anchor（`app/hitomi_test.go:865`・`:948`）
  4. docs ポインタ化（`api-rate-limits.md`・`usage-memo.md` の数値写しを正典参照へ）

## 委譲サイクル記録

- 項目1 委譲前状態: HEAD `73507d1`、`git status` clean（2026-11-05 10:16 UTC）。
- 項目1 着手時（子の記録・復元）: HEAD `0b00053`（上記チェックポイントコミット）、`git status` clean（2026-11-05 10:19 UTC 実測）。
- 項目1 機械的確認（親・2026-11-05 10:26 UTC）: `git log` に項目1コミット `0d5dbdf` 存在（push 済み）。`git status` は clean（委譲前 clean から悪化なし・項目1分のみ）。個別タスク記録の項目1結果（合格・テスト5件実測）は子の報告と一致。→ 項目1 合格。

- 項目2（block 検出）委譲前状態: HEAD `3885ee1`、`git status` clean（2026-11-05 10:27 UTC）。

### 項目1 実装確定（子・実装前・2026-11-05 10:20 UTC）

- 記録との差異（要確認点だったが実測で閉じた）: 項目1行は「`gateLane` 下限ガード」と記載。実際のコードと正典照合すると、設定値が確定する箇所は `loadConfig`（`app/main.go` 163-172 行）で、承認範囲（子への指示）も `loadConfig` ガード。実際のコードは承認範囲と一致しているため、ガードは `loadConfig` に実装する（`gateLane` は変更しない）。この差異は記録に残す。
- 修正方法:
  1. `app/hitomi.go` `defaultHitomiConfig()`（223 行）: `BgGraceMS: 300` リテラルを廃止し正典 `const defaultBgGraceMS`（350 行）を参照。単一出力元化（`UnmarshalJSON` は既に正典 const 参照・変更なし）。
  2. `config.example.json`: `hitomi` へ `"gg_force_min_interval_ms": 60000` と `"bg_grace_ms": 300` をコード既定値どおり追加（KI-20261105-07 対応。example 側数値写しのポインタ化は項目4・本項目外）。
  3. 下限ガード（`app/main.go` `loadConfig`）: `min_interval_ms` / `image_min_interval_ms` に対し、0 は従来どおり「0＝未指定＝既定値」フォールバック（変更なし）、負値は未指定として既定値フォールバック（「0＝未指定＝既定値」の挙動を負値にも適用）、1〜49 ms はキー名を含む明示的な設定エラーで拒否、50 ms 以上は受理。下限は `const minHitomiRateFloorMS = 50`。
- 影響範囲: `app/main.go`（loadConfig と下限 const）、`app/hitomi.go`（1 行）、`config.example.json`、`app/main_test.go`（新規テストのみ。既存テストリテラル `hitomi_test.go:865`・`:948`、`main_test.go` の 1000/250 アサートは変更しない）。テストの直接 cfg override（`MinIntervalMS = 1` 等）は `loadConfig` を経由しないためガードの影響なし。`test_scan.py` は example の `limits` のみ使用（hitomi 追加キーは影響なし）。
- 客観的合格条件:
  - `app` で `go vet . && go test -count=1 -timeout 300s .` すべて合格（Commit Checklist により `python -m pytest tests/ -v` も合格）
  - 新規テストで実測証明: (a) 1〜49 ms 拒否（エラーメッセージにキー名）、(b) 0→既定フォールバック維持（負値→既定）、(c) 50 ms 以上受理、(d) example の新 2 キー存在と 60000/300 一致、(e) `defaultHitomiConfig().BgGraceMS == defaultBgGraceMS`（単一出力元）
  - 本項目分だけを単独コミットし push まで
- 検証方法: 上記コマンド実行。コマンド・結果・経過時間を Verification に記録。UI 変更なし（画面確認該当なし）。
- 注意: 本タスクは closed 後に reopen したため `current-task.tsv` の参照行が無い（正典 `update` は行を再作成しない・edit 直接編集は不可）。close 時に正典 `end` で行削除相当の確認を行う。

（機械的確認結果は委譲後に追記）

### 項目2 実装確定（子・実装前・2026-11-05 10:31 UTC）

- 作業前チェック（AGENTS.md・viewer/download 接触タスク必須条）として以下を事前に読了（ファイル名明記）:
  - `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
  - `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
  - `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`
  - 加えて正典 build ルールのポインタ先 `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` と build command 正典 `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`（canonical build・server コマンドの正典）を参照
- 記録とコードの照合: §3「429 以外の block 形態（403/Cloudflare challenge）は現状リトライ対象外（`app/hitomi.go:532` は 429/5xx のみ）」→ 実コード 530-544 行と一致。不一致なし（実装可能）。gateLane は FIFO・共有 `last` 連鎖・A-06（sleep 中 ctx cancel 即 return）意味を維持したまま、クールダウン区間の effective interval 適用のみ追加。
- 修正方法:
  1. `app/hitomi.go` `fetchRemoteWithGate`: block signal 判定（`isBlockSignal`）追加 = 403 / `cf-mitigated` ヘッダ（Cloudflare challenge）/ 503 / 522。block をリトライ対象化（backoff は既存 `RetryBackoffMS×(attempt+1)` 式を基盤）。403 は従来即時 return → block として backoff 付きリトライへ変更（承認範囲内。403 の backoff 式は既存 429/5xx と同一式を採用）。
  2. `Retry-After`（429・block 双方）: 存在したら backoff として尊重。秒数（RFC 9110 delay-seconds）と HTTP-date の両形式。負値・解析不能は ok=false → 既存静的 backoff へフォールバック（挙動変更なし）。上限 `maxRetryAfterMS = blockCooldownDuration`（5 分・下記根拠）。
  3. クールダウン: block signal 検出時 `blockCooldownUntil = now + blockCooldownDuration`（Client 領域・`mu` 保護）。クールダウン中は `gateLane` の effective interval を両 lane（data・image）で `max(現行 interval, politenessDefaultIntervalMS)` へ退避。
  4. 数値の根拠（推測数値を作らない・段階1 採用判断）:
     - `blockCooldownDuration = negativeNoFullCacheTTL`（既存正典 convention 5 分・`app/hitomi.go:22`）。UI 側の既存 convention `PREFETCH_COOLDOWN_MS = 300000`（`app/search.html`・`tests/test_search_prefetch.py` が「429 sets a 5 minute cooldown in memory」で assert）と同一値。新たな数値の捏造なし。
     - `politenessDefaultIntervalMS = 1000`（正典 politeness 既定。`defaultHitomiConfig()` data lane 既定・`api-rate-limits.md`/`usage-memo.md` の data 1000 ms 正典値と一致）。
     - `Retry-After` 上限 = `blockCooldownDuration`（5 分）。根拠: 正典 convention の penalty window（5 分）を超える server 提示は「不合理に大きい値」として clamp。既存 convention 以外の数字は使わない。
     - クールダウン退避は「effective interval への max 適用」で実装（既定値の変更ではない → 項目3 の領域外・既存テストリテラル `hitomi_test.go:865`・`:948`（1000/250 アサート）は変更しない）。
- 影響範囲: `app/hitomi.go`（`fetchRemoteWithGate` 末尾 status 分岐、`gateLane` 冒頭の effective interval、Client へ `blockCooldownUntil` 1 フィールド、const 3・helper 2）、`app/hitomi_test.go`（新規テストのみ）。metrics（kind 分類・`marker.<priority>` 分離）・FIFO・共有 last 連鎖・A-06・既定値・docs は変更しない。UI 変更なし（画面確認該当なし）。
- 客観的合格条件:
  1. `app` で `go vet . && go vet && go test -count=1 -timeout 300s .` 全合格
  2. `python -m pytest tests/ -v` 合格
  3. 新規テストで実測証明: (a) 403/`cf-mitigated`/503/522 の block 判定とリトライ、(b) `Retry-After`（429・block）尊重（記録 retrySleep の実測・静的 backoff との区別）、(c) 上限 clamp（`parseRetryAfter` 600 s → 5 分）、(d) クールダウン有効中に data・image 両 lane の gate wait が 1000 ms へ退避（実測 elapsed）、クールダウン満了後は現行 interval 復帰、(e) 429 のみ（block なし）はクールダウン不发動（意味維持）
  4. 正典 build（canonical exe 再 build）で実機確認: CLI download 実在 gallery で attempted == written。viewer `/api/image/<hash>?format=webp` が 200 / `x-cache: miss|cache|local`・`no_full` でないこと（バイト数記録）
- 検証方法: 上記コマンド実行。外部 API 実呼出しは実機確認のみ（正典 UA・`referer`・現行 rate 1000/250 ms 遵守）。テストは httptest/mock（外部 API 呼出しなし）。コマンド・結果・経過時間を Verification に記録。
- 項目2 終了判定は本記録の「項目2 結果」セクションで行い、タスク全体 Status は進行中のまま（親が全項目完了後に end）。

## 項目1 結果（子・実装と検証完了 2026-11-05 10:25 UTC）

- 合否: 合格（項目1の合格条件をすべて実測で確認）
- 変更内容（承認範囲どおり・項目2〜4は未変更）:
  1. `app/hitomi.go` `defaultHitomiConfig()`: `BgGraceMS: 300` リテラル → `BgGraceMS: defaultBgGraceMS`（単一出力元化）
  2. `config.example.json`: `hitomi` へ `"gg_force_min_interval_ms": 60000`・`"bg_grace_ms": 300` 追加（コード既定値と一致）
  3. `app/main.go`: `const minHitomiRateFloorMS = 50` 追加・`loadConfig` で `min_interval_ms`/`image_min_interval_ms` の下限ガード（0＝従来どおり既定フォールバック・負値は未指定として既定フォールバック・1〜49 ms はキー名を含む明示エラーで拒否・50 ms 以上受理）
  4. `app/main_test.go`: 回帰テスト5件追加（新規のみ。既存テストリテラル `hitomi_test.go:865`・`:948`・`main_test.go` の 1000/250 アサートは未変更。`encoding/json` import 追加）
- 採用した判断（段階1）: 下限ガードの配置は「gateLane」記載に対し承認範囲（loadConfig）へ実装。実測で `loadConfig`（163-172 行）が設定値確定箇所であり、gateLane を変更せずとも config 経由値が guard されることをテストで確認。記録の記載と実コードは一致（不一致による持ちら帰なし）。
- 未解決: KI-20261105-07 は本項目で解決（Status 更新済み）。`known-issues/archive/` への移動・`task_manager.py end` はタスク全体の close 操作（親・全項目完了後）に委譲。`current-task.tsv` の参照行は欠落（edit 直接編集禁止のため作成せず・close 時確認）。

## 項目2 結果（子・実装と検証完了 2026-11-05 10:41 UTC）

- 合否: 合格（項目2の合格条件をすべて実測で確認）
- 変更内容（承認範囲どおり・項目3〜4は未変更。metrics・FIFO・共有 last 連鎖・A-06・既定値・既存テストリテラルは未変更）:
  1. `app/hitomi.go`: `isBlockSignal`（403 / `cf-mitigated` ヘッダ / 503 / 522）追加。`fetchRemoteWithGate` の status 分岐で block をリトライ対象化（既存 429/5xx の 2000 ms×試行式を基盤として維持。403 は同一 backoff 式で block 扱い）
  2. `Retry-After`（429・block）: `parseRetryAfter`（RFC 9110 delay-seconds・HTTP-date 両形式）。存在時は backoff として尊重。負値・解析不能は静的 backoff フォールバック（挙動不変）。上限 `blockCooldownDuration`（5 分・正典 convention `negativeNoFullCacheTTL`。数字の新規捏造なし）
  3. クールダウン: block 検出時 `Client.blockCooldownUntil`（`mu` 保護）= 5 分。`gateLane` で有効中は両 lane の effective interval を `politenessDefaultIntervalMS = 1000` ms へ退避（max 適用・既定値変更なし）
- 採用した判断（段階1・根拠済み）: クールダウン期間 5 分 = 既存正典 convention `negativeNoFullCacheTTL`（`app/hitomi.go:22`）と UI 側 `PREFETCH_COOLDOWN_MS = 300000`（`tests/test_search_prefetch.py` の「429 sets a 5 minute cooldown in memory」assert）の同値。上限の根拠も同一 penalty window convention。politeness 退避は「既定値変更」ではなく effective interval への max 適用として実装（項目3 の領域外を侵犯しない）
- 実測での修正（反復なし・原因分析済み）:
  - `http.ParseDate` 不存在（正典は `http.ParseTime`）→ vet が検出・修正（1回）
  - 新規テスト1件で `c.http` 代入漏れ（nil panic）→ 原因分析・修正（1回）
  - クールダウンテストの FIFO 意味（lane 初呼出しは即時・last 記録）を反映せず誤アサート → warmup 追加で修正（1回）。FIFO 実装は変更していない（テスト側の測り方の修正）

## Verification

- `python docs\ai\tmp\rate_probe_20261105_floor.py` → `docs/ai/tmp/rate_probe_20261105_floor_results.json`（data 100/50/25 ms 逐次・image 404 は gg snapshot drift で rate 判定外）
- `python docs\ai\tmp\rate_probe_20261105_image.py` → `docs/ai/tmp/rate_probe_20261105_image_results.json`（live gg・submit().result() で並行化が実質逐次だったため下項目で再測）
- `python docs\ai\tmp\rate_probe_20261105_conc.py` → `docs/ai/tmp/rate_probe_20261105_conc_results.json`（真の並行 4/8/12・handshake 分離計測）
- コード変更なし → 本プロジェクトの検証コマンド該当なし（docs のみ）

### 項目1（実装・2026-11-05 10:22-10:25 UTC・実測）

- `app` で `go vet . && go test -count=1 -timeout 300s .` → 合格: `ok hitomi-hub/app 17.728s`（vet+test 合計経過 17.7 s）
  - 最初の実行で新規テストの型アサート誤り（`raw["hitomi"]` が既に `map[string]any`）を vet が検出 → 原因分析して修正（`raw` を `map[string]any` に）→ 再実行で合格（単発・反復なし）
- 新規テスト個別実測（`go test -count=1 -v -run` 5件・全 PASS）: 1〜49 ms 拒否（25 ms・49 ms・エラーメッセージにキー名）、負値→既定フォールバック（-1→1000・-5→250）、0→既定フォールバック維持、50・125 ms 受理、`config.example.json` の新2キー存在と 60000/300 一致＋loadConfig 経由で example の hitomi 全体が `defaultHitomiConfig()` と一致、`defaultHitomiConfig().BgGraceMS == defaultBgGraceMS`（単一出力元）
- Commit Checklist: `python -m pytest tests/ -v` → 53 passed（10.27 s・`test_canonical_build_freshness.py` 含む）
- UI 変更なし → 画面確認該当なし（保存先・ファイル名なし）

### 項目2（実装・2026-11-05 10:33-10:41 UTC・実測）

- `app` で `go vet . && go vet && go test -count=1 -timeout 300s .` → 合格: `ok hitomi-hub/app 22.971s`（vet+test 合計経過 23.0 s。プロジェクト検証条の `go vet .` と `go vet` の両方実行）
- 新規テスト5件個別実測（`go test -count=1 -v -run`・全 PASS・合計 6.268 s）:
  - `TestBlockSignal403CFMitigatedRetriedWithRetryAfterRespected` (1.00 s): `Retry-After: 1` の試行間 gap 実測 ~1000 ms（静的 2000 ms ではない）／metrics `retry_sleep_ms=1000`／requests=2／クールダウン期限 = `blockCooldownDuration`（5 分±100 ms）
  - `TestBlockStatusesRetriedWithExistingBackoffAndEngageCooldown` (3.00 s): 403・503・522 各 requests=2（block リトライ）・`retry_sleep_ms=20`（既存 backoff 式・Retry-After なし）・クールダウン発動。403 は項目2前では即時 return（リトライ対象外）だった意味変更を実測
  - `Test429WithoutRetryAfterKeepsExistingBackoffAndNoCooldown` (0.10 s): 429 単独の静的 backoff（100 ms 式）維持・gap 実測 ~100 ms・クールダウン不发動（block signal 定義外）を実測
  - `TestParseRetryAfterRespectsValueAndCapsAtCanonicalConvention` (0.00 s): `600`→上限 clamp（=5 分 convention）、`300`→300 s 尊重、負値/解析不能→フォールバック、HTTP-date 両方向、`blockCooldownDuration == negativeNoFullCacheTTL`（推測数値なし）
  - `TestBlockCooldownRetreatsBothLanesToPolitenessDefaultAndRestoresOnExpiry` (2.00 s): block 後 data lane gate wait 実測 ~1000 ms（inject 1 ms ではない）、image lane（既定 250 ms）gate wait 実測 ~1000 ms（退避）、満了後 inject 値へ即時復帰
- Commit Checklist: `python -m pytest tests/ -v` → 53 passed（10.00 s・`test_canonical_build_freshness.py` 含む）
- 正典 build（canonical exe 再 build・正典 build command）後、実機確認:
  - CLI download（正典 exe・実在 gallery 4294614・`--limit 1`）: `attempted=1`・`written=1` 一致（skipped=0）。正典 UA・`referer`・data rate 1000 ms 遵守
  - viewer（正典 server・正典 server コマンド・port 8731）: `GET /api/image/098fa2e4529e651af3fe594fa57a57053598502d5ead2533edc03726d1f3f2ce?format=webp` → `HTTP/1.1 200`・`X-Cache: miss`・`no_full` でない・response body 20476 bytes（`docs/ai/tmp/item2_viewer_headers.txt`・`item2_viewer_image.webp`）。外部 API 実呼出しは本実機確認の 1 呼出しのみ（image rate 250 ms・gate 内）
- UI 関連変更なし → 画面操作の確認は該当なし（未検証而非該当。テストは mock/httptest・外部 API 呼出しなし）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
