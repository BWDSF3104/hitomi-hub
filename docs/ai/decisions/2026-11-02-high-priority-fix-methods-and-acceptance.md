# 高優先度修正方法・合格条件（2026-11-02）

## 方針

- 実装開始指示済み。優先度高い順で修正し、各項目で合格条件を満たすまで確認する。
- 実装開始ゲートクリア済みとして扱える。テストは外部 API を実際に呼ばない。
- `docs/ai/tasks/2026-11-02-external-code-review-findings.md` に実施記録を追加する。

## 1. VT `stats` key と未分析 handling

**修正方法**
- `app/vt.go` の status 算出で `attrs["stats"]` ではなく `last_analysis_stats` を読む。
- `last_analysis_stats` が無い、または `last_analysis_results` の分析完了状態を示すフィールドが無い場合は `not_evaluated`。
- `malicious + suspicious` の合算を positives として使う。
- 実呼出し回数を増やさない。既存 rate gate と TTL を維持。

**合格条件**
- `go vet .`, `go test -count=1 .` が `app` で合格。
- v3 形状 fixture で、`result` がある `malicious` engine が `hit` になる。
- `analysis in progress` / `last_analysis_results` 欠如が `not_evaluated` になる。
- `stats` 誤キーだけがある fixture では `not_evaluated`、または修正後の正キーで hit になることをテストで確認。

## 2. VT `ThreatName`

**修正方法**
- `last_analysis_results` entry の `result` を優先。
- `engine_name` を併記できる場合は `engine_name: result`。
- `result` が無い場合は `category` を最終フォールバック。
- テスト fixture を v3 実レスポンス形状へ更新。

**合格条件**
- 実 API 形状 fixture で `result` から検出名が出る。
- `result` が空で `category` だけの例は `category` にフォールバックする。
- 既存 `name` 依存テストを消し、v3 形状テストで置換。

## 3. VT 失敗で構造 `pass` が壊れる

**修正方法**
- 構造検査 status を `structural_status` として保持する。
- `files.status` は構造判定優先。VT 失敗は VT status の finding として記録。
- `scan_error` はファイル構造解析不能の意味に限定。VT 障害は `api_error` として findings detail に記録。
- export skip 条件は構造 status の `hit` / `scan_error` を基準にする。

**合格条件**
- 構造 `pass` の cbz で VT fetch failure, no_key, rate_limited があっても `files.status` が `pass`。
- VT 障害が別 status finding として確認できる。
- `go test` で summary の `pass` 加算を維持。

## 4. `imageProxy` gg.js force refresh throttle

**修正方法**
- `Client` に `ggForceAt` と `ggForceMinInterval` を追加。
- `EnsureGG(force=true)` は mutex 内で、最後の force から N 秒未満なら fetch せず cache を返す。
- `imageProxy` は refresh 前の bucket/CaseSet を保持し、refresh 後も同一なら retry せず `no_full`。
- gg singleflight を無依存の per-key map で追加。既存 `cacheMu` を使うか `Client` に dedicated mutex を追加。
- `IndexIDs` の index version 404 で `ensureIndexVersion(field, true)` を 1 回だけ呼ぶ。同一 minimum-interval guard を共有。

**合格条件**
- 404 連続で同一 client に force 呼出しが 2 回続いても、minimum interval 内では fetch 1 回または skip。
- bucket が同じ refresh では retry しない。
- `go test` で concurrent force の singleflight または mutex protection を確認。
- `TestEnsureGGUsesTTLAndForceRefresh` は新挙動に合わせて更新済み。

## 5. `openGallery` 同 gallery 開き直し

**修正方法**
- `openGallery` で abort した場合、`backgroundGalleryId = null` または `backgroundSignal = null` を設定。
- `ensureBackgroundPrefetch` が new controller を作るようにする。
- `manualRetryFailedFullImages` に `catch` を追加し、aborted signal を unhandled rejection にしない。

**合格条件**
- `go test` で `openGallery` に `backgroundGalleryId = null` または signal 再生成がある。
- Playwright actual UI:
  - fresh tab、fresh root、server baseline と一致する response。
  - 同一 gallery 内 strip item を actual click。
  - `backgroundSignal.signal.aborted` が true のままではない、または fetch が継続する。
  - `fullDone` または `naturalWidth > 0` が増加する。
  - 全物理画面 `3840x2160` で screenshot を保存。

## 6. A-3 `orderbydirection:asc` 二重 reverse

**修正方法**
- `SearchHTML` で `applyOrderForInitial` call と関数を削除、または末尾 `applySearchOrder` だけ残す。
- `or` 経路でも末尾適用が効くことをテストで確認。

**合格条件**
- `go test` で単一 term + `orderbydirection:asc` が reverse される。
- `or` 経路でも `asc` が適用される。
- 既存 `desc` default テストが合格。

## 7. YARA failure status

**修正方法**
- `RunYara` を `(rules []string, err error)` に分離し、実行失敗を finding rule として返さない。
- `processFile` で YARA 失敗時は `scan_error` または warning status を使う。
- summary 集計が YARA 失敗を `hit` にしない。
- Python parity 側は exit code allow-list 方針を記録し、必要なら同じ語彙へ。

**合格条件**
- 存在しない `yr` 実行で `RunYara` が error を返し、`processFile` が `hit` にならない。
- `scan_error` または warning が findings/summary に正しい。
- 実 `yr` をテストで呼ばない。

## 8. search cache / B-tree node cache / record transaction

**修正方法**
- `SearchHTML` のnozomi term result を `(area, tag, lang)` TTL cache に保存。page slice は cache から取る。
- B-tree root/internal node を `field|version|addr` key で TTL cache。
- `recordDiscovered` を prepared statement で batch 1 transaction にまとめる。
- `searchCard` の単独 write も既存行 skip か tx 内へ統一。

**合格条件**
- ページ移動で同 query/lang の cache hit が確認できる。
- B-tree node cache hit が確認できる。
- `recordDiscovered` 1 batch で 1 commit であることをテストまたは count で確認。
- `go vet .`, `go test -count=1 .` が app で合格。

## 9. findings duplicate unique

**修正方法**
- `findings` に `UNIQUE(sha256, rule, detail)`。
- `addFinding` を `INSERT OR IGNORE` にする。
- 既存 DB の列欠如/重複を migration で扱うか、`user_version` 段階 migration を追加。

**合格条件**
- 同じ sha256/rule/detail の重複行が増えない。
- `go test` で addFinding の duplicate insert が idempotent。
- `schema.sql` と migration 経路が一致。

## 10. acquisition run confirm

**修正方法**
- run 開始時の folder check を 1 回だけ行う。
- run 内は確認済み flag を使うか、per-file check だけ残す。
- UI の confirm 文が run 開始判定であることをテスト/実機で確認。

**合格条件**
- page 1 → page 2 の同一 run で 2 ページ目 confirm が出ない。
- UI actual click で confirm が run 開始時の 1 回だけ。
- `go test` で folder check once を確認。

## 11. settings/security

**修正方法**
- `use_live_gg` を sentinel または per-field default で JSON 省略時 true を維持。
- `--config` の記載を usage-memo から削除、または flag 追加。
- POST middleware で Host を localhost 限定、Origin があれば same-origin、Content-Type JSON 要求。
- pragma を DSN `_pragma=` に寄せる。
- schema migration を user_version 段階で実装、または古い DB 再生成の運用明文化。

**合格条件**
- JSON 省略の `use_live_gg` が true のまま snapshot に落ちない。
- `--config` usage と actual behavior が一致。
- `go test` で Host/Origin middleware が POST を localhost で許可し、cross-origin/simple text を拒否。
- pragma settings が接続 reuse で持続する経路または DSN 設定で確認。
- `go vet .`, `go test -count=1 .` が app で合格。