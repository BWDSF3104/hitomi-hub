# assess-20261103-1220 残項目の実装実行計画（サブエージェント1件ずつ・優先順）

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 実施開始指示待ち。指示後は優先4 B-08から1項目ずつサブエージェントで実施し、結果を本タスクと個別タスクへ逐次反映

## 意図 / 背景

- `2026-11-03-assess-20261103-1220-implementation-targets.md` のとりまとめ（優先順位・設計承認3件の条件）を実行計画として取りまとめる。
- 実施方式: 1項目 = 1回のサブエージェント（general）。サブエージェントは同時に最大1つ（global 正典）。1項目完了後に次へ、優先順どおり。
- 1項目ごとに: 個別タスクファイル・単独検証・単独 commit（commit はユーザー承認前提）。
- フル画像ガード関連項目は AGENTS.md「絶対に壊してはいけない条件」優先。作業前に 2026-11-01 の 2 decision（root-and-tn-display-correction-policy、fast-nonblocking-viewer-policy）と `2026-11-01-ggset-generation-and-drift.md` を読み、タスク記録にファイル名で明記する。
- 完了判定は実施確認に基づく。確認できない項目は「未確認」として記録し、完了と書かない。

## 実施方式（各項目共通）

1. サブエージェントは個別タスクファイルと関連コードを読ってから実施する。
2. 合格条件は個別タスクの判定記録（実測根拠）に基づく。テスト追加・更新を含む。
3. 検証: `app` で `go vet . && go test -count=1 -timeout 300s .`、ルートで `python -m pytest tests/ -v`（`test_canonical_build_freshness.py` 含む）。UI・Go 変更後は canonical exe 再 build、canonical server（既定 `http://127.0.0.1:8731`）からの実機確認（Playwright）。
4. 実機 UI 確認（B-01・B-03・B-09 など実操作必須項目）はブラウザツールを持たないサブエージェントでは私が実施する。
5. 個別タスクファイルの「調査結果 / 変更内容」と Verification を即更新し、項目完了時に正典 `task_manager.py end` で閉じ、`current-task.tsv` 参照行を削除する（plan タスクの実施 progress に1行追記）。
- 実施順の取りまとめ完了（本タスクの範囲はここまで）。**実施開始はユーザーの明示的な指示後**（2026-11-03 指示: 開始してはならない。B-08 のサブエージェント起動は取消、code change なし）。commit は項目ごとに単独。global 正典どおり、変更の詳細についてユーザーの事前承認を前提とする。
7. 使い捨てスクリプト・スクショは `docs/ai/tmp/assess-20261103-1220/`。インラインコード実行禁止、`Get-Content`/`Set-Content` パイプ禁止。

## 実施順

### 実施済み（優先1–3・解消済み）

- 実装順 1–3（設計承認3件・単独 commit）: A-08 `683ff47` → C-08 `1b8bb30` → A-07b `fa10536`
- 優先1 A-05/C-01/C-02 `c77f92a`、優先2 B-02 `622fadc`、優先3 A-01/B-11 `55b9f83`
- D-05 解消済み close `621056f`

### 残項目（1項目ずつ順次）

| # | ID | 実施対象 | 備考 | 状況 |
|---|---|---|---|---|
| 4 | B-08 | zip path traversal を path segment `..` 判定へ修正。YARA規則と構造判定を揃える | Go（scan.go）・YARA規則（Python 参照実装に同款があれば揃える） | 未実施（サブエージェント起動は取消。開始指示待ち） |
| 5 | D-03 | findings unique に `path` 追加。Go作成DBとcanonical DBで unique index を揃える | 既存DB重複確認・マイグレーション検証 | 未実施 |
| 6 | D-04 | `IsPE` を `e_lfanew` 参照へ修正 | Go | 未実施 |
| 7 | B-01 | `form#search`/`form#nav` の prev・next・complete・fav に `type="button"`。Enter 暗黙 submit を検索固定 | UI実操作必須 | 未実施 |
| 8 | B-03 | fav/complete cache state を toggle ごとに更新。cache 巻き戻し防止 | UI実操作必須 | 未実施 |
| 9 | B-09 | `newTabMode`・修飾キー click で `preventDefault` しない | UI実操作必須・方針確認必要 | 未実施 |
| 10 | B-05 | `hitomi` キーなし config で `defaultHitomiConfig()`。snapshot fallback を黙って起こさない | フル画像ガード関連 | 未実施 |
| 11 | A-15 | `isLoopbackHost` の `[::1]:port` 判定修正 | Go | 未実施 |
| 12 | A-10 | `downloadFilesParallel` 既存ファイル確認。skip と remote fetch 回数修正 | CLI download 実在 gallery 確認必須 | 未実施 |
| 13 | A-07a | cache write 前に image magic/format 検証 | フル画像ガード関連 | 未実施 |
| 14 | B-06 | VT 空 `last_analysis_results` + positives 0 を `not_evaluated` | 実API確認は key 未保有で未実施（テストで固定） | 未実施 |
| 15 | B-07 | YARA failure を構造 pass に優先させず別 finding/status 分離。export/pdf skip 条件維持 | Python | 未実施 |
| 16 | B-10 | Python 参照実装 VT の `HTTPError` handling、404 `unknown_to_vt` 実経路 | Python | 未実施 |
| 17 | A-02/C-03 | `recordDiscovered` per-call commit を batch/transaction 化。実測根拠をテスト固定 | Go | 未実施 |
| 18 | A-03 | `json_ms` 計測修正、metrics 二重返却修正。テスト更新 | Go | 未実施 |
| 19 | A-14/C-05 | `searchCache`・`indexDataCache`・`indexNodeCache`・`infoCache` に上限・削除・TTL | Go | 未実施 |
| 20 | C-04 | `language:` 併用時の同一 tag 二重 fetch 排除 | Go | 未実施 |
| 21 | C-06/C-07/C-09 | strip rebuild・resize/fullscreen・read/lists の debounce と generation guard 整理 | フル画像ガードに触れる可能性 | 未実施 |
| 22 | A-11 | PDF `rawRGB` 全ページ分保持の解消（上限 or streaming 評価） | Go | 未実施 |
| 23 | A-04/D-09/D-10 | hitomi index key / sort convention 確認後に判定（判定保留） | hitomi 準拠確認（調査） | 未実施 |
| 24 | A-06 | `time.Sleep` を ctx-aware 化 | Go | 未実施 |
| 25 | A-09 | `serveStoredImage` 全読み・Range・304・Content-Range 評価（必要なら部分的 stream/conditional） | Go | 未実施 |
| 26 | A-13 | `ScanPDFKeys` の `string(data)` key allocation 回避 | Go | 未実施 |
| 27 | C-10 | `go test` gate sleep をテスト専用 config で 0 にする案の評価のみ（本番ゲート間隔を変えない） | 案評価のみ | 未実施 |
| 28 | D-01/D-02/D-08 | `.gitignore`、config.example 秘密扱い、production 未使用コード整理 | trivial | 未実施 |
| 29 | D-06 | gg case 方向、TN fallback 古記述、check 説明、README UI 記述修正 | docs 修正は承認後 | 未実施 |
| 30 | D-07 | `test_canonical_build_freshness.py` の clean clone / CI 前提固定 | tests | 未実施 |
| 31 | A-12 | SHA256 全件毎回計算（原優先表外・個別タスクあり） | 低優先 | 未実施 |
| 32 | B-04 | acquisition フォルダ名と確認処理の不一致（原優先表外・D-02 関連） | 低優先 | 未実施 |

## 実施 progress

（1項目実施ごとに1行追記: ID・commit・検証結果・未確認点）

- B-08 合格 commit `865c325`（push 済み）。基準: entry 名を `/` `\` で分割し segment 完全一致の `..` のみ hit。実測 `Vol...01.webp` → pass（Go/Python/YARA 非 hit）、`../evil.exe` → hit。go vet+test ok 36.919s、pytest 26 passed。個別記録は完了。残件（判断要）: 記録記載の `containsTraversal(name)` は実際 `app/export.go:174`（部分一致のまま・承認範囲外・未変更）→ **解決（2026-11-03 ユーザー承認）**: segment 判定の共有ユーティリティへ統一することを承認し、追加項目として実施（単独 commit）。
- D-03 合格 commit `8962fcb`（push 済み）。unique key を `(path, sha256, rule, detail)` へ、Go `ensureSchema` に unique index 追加（schemaVersion 4、迁移は定義置換のみ・行削除禁止）。実測: 新規 Go DB で scan 2回 → findings rows 2→2 不変（INSERT OR IGNORE skip）、Go 作成 DB・canonical DB で key 一致。go test ok 37.201s、pytest 29 passed。個別記録は完了。残件（判断要）: ① 調査用 `repro.db`（rows=20・新 key 重複 group=5）は行削除なしで index 作成不可（copy で失敗実測、original 未変更）→ 重複除去か再生成の判断。② root `hitomi.db`（rows=8・旧 key）はスコープ外で未変更。③ 非破壊迁移の設計判断を `docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md` に記録（旧暗黙 dedup を supersede）。→ **解決（2026-11-03 ユーザー承認）**: 拒否方針（c）で確定、暗黙 dedup へは戻さない。`repro.db` は `kilo-safe-remove` でゴミ箱へ（掃除）。root `hitomi.db` は次回 `ensureSchema` 実行で揃う（今すぐ手入れ不要、次回 canonical server 起動時に確認）。
- D-04 合格 commit `38fb12b`（push 済み）。`is_pe` を `e_lfanew` 参照（seek）へ。実測: `e_lfanew=0x80` fixture（PE signature at 0x80）で `mz_without_pe_header` 非 hit（修正前は hit をスクラッチ `docs/ai/tmp/assess-20261103-1220/d04_fixture_check.py` で再現）、回帰 `0x38` pass・`0x80` signature なし hit・`e_lfanew=0` hit 維持。pytest 33 passed 3.35s（freshness 含む）。個別記録は完了。YARA 規則変更なし（規則は `MZ_Header_At_Offset0` のみ、実測上齟齬なし）。実測根拠 `Galleries\102\b.bin` は現マシン不在 → fixture 生成で代替。
- B-01 合格 commit `c1d2c07`（push 済み）。`form#search` の暗黙 submit を `#search-submit` へ固定、prev/next・nav button に `type="button"`。実測（1280x900 fresh tab・canonical exe 再 build + server 8731）: page 2 で `female:catgirl` + Enter → `GET /api/search?...page=2` 200、`#page` 2 維持、prev/next クリックは `setSearchPage` で page 3/2 実測。go vet clean、go test ok 36.930s（固定テスト `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit`）、pytest 33 passed。スクショ Drive `I:\マイドライブ\kilo\b01-confirm-1280x900-enter-page2.png`。個別記録は完了。残件（判断要）: nav（`#pagenum`）上 Enter の挙動が prev 暗黙発火 → submit event（no-op）へ変化。→ **解決（2026-11-03 ユーザー承認）**: no-op（検索固定）で確定。nav の page 跳转は別項目。`docs/ai/decisions/2026-11-03-b01-nav-enter-noop.md`。

## 実行打ち切り（2026-11-03）→ 再開

- 委譲は実施順4（B-08）〜実施順7（B-01）の4件で停止。B-01 の nav Enter 挙動変更が判断要であり、以後の B-03 は同一 `app/ui.html`（fav/complete handler・form submit セマンティクス）に依存するため、記録から無関係と確認できず打ち切り。実施順8 以降（B-03・B-09・B-05・A-15・A-10・A-07a …）は未実施。
- **2026-11-03 判断解決・再開**: B-08 の export 判定統一・D-03 の拒否方針・B-01 の nav Enter no-op をユーザー承認（推奨案どおり）。B-08 追加項目（`app/export.go` の segment 判定統一）を実施し、以後は実施順8（B-03）から順次委譲。B-09（`newTabMode`・修飾キー click）は方針確認必要のまま保留。

## Verification

- plan タスク自体は docs/ai 記録のみ。code change なし。
- 項目ごとのコマンド・結果・経過時間は個別タスクの Verification と本実施 progress に記録。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
