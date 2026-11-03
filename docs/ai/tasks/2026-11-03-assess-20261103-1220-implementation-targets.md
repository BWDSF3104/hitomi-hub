# assess-20261103-1220 実施対象とりまとめ（設計承認3件＋remote task 43項目）

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: remote task 43項目の実施対象と設計承認3件の条件を統合して実装タスクへ引き継ぐ

## 意図 / 背景

- `assess-20261103-1220` の remote task 追加タスク 43 項目と、その中で設計承認が必要とされた A-07b、A-08、C-08 の実施条件を、実装タスク分割・順序・合格条件として整理する。
- 本タスクは記録の整理のみで、実装開始指示は含まない。実装する場合は、特にフル画像ガードに触れる項目は単独タスク・単独コミット・単独検証を前提とする。
- 設計承認3件はフル画像ガードに触れるため、実装する場合は 3 件とも単独タスク・単独コミット・単独検証を前提。

## 調査結果 / 変更内容

### Remote task 追加タスクの判定内訳

- Remote task `2026-11-03-hitomi-hub-claude-review-349778d.md` により追加されたタスクは、commit `4073ed4` の 43 項目タスク + 親 `2026-11-03-claude-review-349778d-assessment.md`。
- 判定内訳:
  - 有効 33
  - 機構事実だが hitomi 準拠確認待ちの判定保留 3: A-04、D-09、D-10
  - 解消済み 1: D-05
  - D-08 は `pageSource` 未使用の指摘が誤り。route/handler 現用に付き、production 未使用コードのみ部分有効。

### 実装優先順位（43項目全体）

| 優先 | ID | 実施対象 |
|---|---|---|
| 1 | A-05 / C-01 / C-02 | 検索カードの逐次取得と `currentPriorityDone` 待ちを分離。1000ms gate の無駄消費と体感遅延を減らす。UI最大。 |
| 2 | B-02 | objectURL の全 revoke をやめ、LRU で個別破棄。表示中と現在 gallery の objectURL を除外し、revoke 時は `prefetchedEntries` からも消す。フル画像ガード関連。 |
| 3 | A-01 / B-11 | PDF page 辞書末尾の `>>` を正す。テストは object 内の `<<` と `>>` 数を比較する形にする。 |
| 4 | B-08 | zip path traversal を部分一致ではなく path segment の `..` 判定へ修正。YARA規則と構造判定を揃える。 |
| 5 | D-03 | findings unique に `path` を追加し、Go作成DBとcanonical DBで unique index の存在を揃える。既存DBの重複確認・マイグレーション検証が必要。 |
| 6 | D-04 | `IsPE` を `e_lfanew` を参照する判定へ修正。`MZ` の先頭64バイト固定で PE を見落とさない。 |
| 7 | B-01 | `form#search` / `form#nav` の prev、next、complete、fav に `type="button"` を付け、Enter の暗黙 submit を検索submitへ固定。実操作テスト必須。 |
| 8 | B-03 | fav / complete の cache state を toggle ごとに更新し、`galleryCache`・`searchCardCache` の巻き戻しを防ぐ。 |
| 9 | B-09 | `newTabMode` または修飾キー付き click で handler が `preventDefault` しない。same-document fragment の `target='_blank'` を尊重。実操作テスト必須。 |
| 10 | B-05 | `hitomi` キーがない config でも `defaultHitomiConfig()` を使う。`UseLiveGG` の snapshot fallback を黙って起こさない。フル画像ガード関連。 |
| 11 | A-15 | `isLoopbackHost` の `[::1]:port` 判定を修正。Host loopback gate を IPv6 bracket-port へ対応させる。 |
| 12 | A-10 | `downloadFilesParallel` で既存ファイルを確認し、既存 skip と remote fetch の回数を正す。 |
| 13 | A-07a | cache write 前に image magic / format 検証を入れる。フル画像ガード関連。 |
| 14 | B-06 | VT の空 `last_analysis_results` と positives 0 を `pass` ではなく `not_evaluated` にする。実API確認は key 未保有で未実施。 |
| 15 | B-07 | YARA timeout / `yr` failure を構造 `pass` に優先させず、YARA failure を別 finding・status に分離。export/pdf の skip 条件を維持。 |
| 16 | B-10 | Python 参照実装の VT で `HTTPError` をハンドリングし、404 の `unknown_to_vt` を実経路で扱う。 |
| 17 | A-02 / C-03 | `recordDiscovered` の per-call commit を batch / transaction 化する。実測根拠をテストで固定。 |
| 18 | A-03 | `json_ms` を正しく計測し、metrics の二重返却を修正。`classifyFetchURL` の `ltn.` 注記と関連。テスト更新が必要。 |
| 19 | A-14 / C-05 | `searchCache`、`indexDataCache`、`indexNodeCache`、`infoCache` に上限・削除・TTL運用を導入。 |
| 20 | C-04 | `language:` 併用時に同じ tag を二重 fetch しない。A-04 と関連だが別件。 |
| 21 | C-08 | `no_full` の short-TTL negative cache をメモリ内のみ導入。フル画像ガード関連。 |
| 22 | C-06 / C-07 / C-09 | strip rebuild、resize/fullscreen、read/lists の debounce と generation guard を整理。フル画像ガードに触れる可能性。 |
| 23 | A-11 | PDF の `rawRGB` を全ページ分保持しない。memory 上限か streaming 化を評価。 |
| 24 | A-04 / D-09 / D-10 | hitomi index key / sort convention を確認してから判定。現状は機関事実だが準拠確認待ち。 |
| 25 | A-07b | `imageProxy` remote fetch の singleflight を追加。past singleflight scope 決定の supersede が必要。フル画像ガード関連。 |
| 26 | A-06 | `time.Sleep` を ctx-aware な sleep / cancellation へ置き換える。 |
| 27 | A-09 | `serveStoredImage` の全読み、Range、304、Content-Range を評価し、必要なら部分的な stream/conditional response を追加。 |
| 28 | A-13 | `ScanPDFKeys` の `string(data)` key allocation を避ける。 |
| 29 | C-10 | `go test` の gate sleep をテスト専用 config で 0 にする案のみ評価。本番ゲート間隔を変えない。 |
| 30 | D-01 / D-02 / D-08 | `.gitignore`、config.example の秘密扱い、production 未使用コード整理。trivial。D-05 は解消済み。 |
| 30 | D-06 | gg case の方向、TN fallback 古記述、check 説明、README の UI 実装記述を修正。docs 修正は承認後。 |
| 30 | D-07 | `test_canonical_build_freshness.py` の clean clone / CI 前提を固定。canonical build なしで失敗する前提を区別する。 |

### 設計承認3件の実施条件（43項目内の内訳）

- A-07b: `imageProxy` remote fetch 限定。dedup key に `hash + format + gg`。CLI download、TN、cache write は対象外。x-cache の full actual pixels 判定と `no_full` / TN / TN-cache / error の意味を変えない。単独 request の cancellation を leader fetch へ直結しない。
- A-08: TN/full 経路分離。`thumbProxy` は TN cache / remote TN のみ。TN failure は `no_tn` / 502。full cache と full fallback を禁止。`no_tn` を full actual pixels として数えない。
- C-08: `no_full` negative cache は memory-only、short TTL。negative cache key は `hash + format + gg`、または actual gg change で clear。manual retry、`EnsureGG(force=true)`、actual gg change で bypass。`x-cache: no_full` と 404 の意味を変えない。

## 実装順

1. A-08: TN/full 分離を正典どおりに固める。後の検証が TN/full 混線と混ざらないため最初に実施する。
2. C-08: `no_full` の再照会抑制を単独で決める。negative cache の意味が単独で固定されていないと fetch dedup と混ざり、gg change/manual retry の判定が複雑になる。
3. A-07b: fetch 共有範囲を広げる。共有範囲が広いため、TN 分離と no_full 判定が固定された後に実施する。

上記の 3 件は remote task 43項目全体の優先順位にも組み込まれている。実装開始指示がある場合は、この順序を前提に単独コミットへ分割する。

## Verification

- 本タスクは docs/ai 記録のみ。code change なし。
- task creation は正典 `task_manager.py add` で実施し、task file と `current-task.tsv` を staging した。commit/push は行っていない。
- 実装タスクの検証前提:
  - `app` で `go vet . && go test -count=1 -timeout 300s .`
  - `python -m pytest tests/ -v`、including `tests/test_canonical_build_freshness.py`
  - canonical build rebuild and running server for UI/network confirmation
  - CLI download で実在 gallery の attempted と written が一致すること
  - `/api/image/<hash>?format=webp` で status、`x-cache`、`no_full`、actual pixel counts を確認すること
  - A-08 では `/api/thumb` の TN-only status を単独確認すること

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
