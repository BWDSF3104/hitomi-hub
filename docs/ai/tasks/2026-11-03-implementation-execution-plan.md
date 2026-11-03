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

## Verification

- plan タスク自体は docs/ai 記録のみ。code change なし。
- 項目ごとのコマンド・結果・経過時間は個別タスクの Verification と本実施 progress に記録。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
