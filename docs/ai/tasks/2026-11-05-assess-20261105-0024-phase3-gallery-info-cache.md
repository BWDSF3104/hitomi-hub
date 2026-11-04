# Remote Task Phase 3 — gallery 情報の永続キャッシュ（調査・評価）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 子の調査完了（有効性「有効」）→ 親の優先順位付けへ

## 意図 / 背景

- 項目ID: `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`（実行固有ID `assess-20261105-0024` の項目2/3）
- リモートタスク原文（正典）: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache.md`
- 個別タスク記録（このファイル）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase3-gallery-info-cache.md`。評価記録一覧: `docs/ai/tasks/2026-11-05-assess-20261105-0024-items.md`

## 調査結果 / 変更内容

項目ID: `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`（実行固有ID `assess-20261105-0024` 項目2/3）。子の調査完了（以下が子記入分。コードは無変更）。

### 棚卸し

- 開始 HEAD `2871fc8`（基準 `7160dae` の後にコミット2件 `52b9f27`・`2871fc8`、いずれも docsのみ。`app/viewer.go`/`app/hitomi.go` の変更なし → 実コードは基準時点と一致、食い違いなし）。
- `git status`: 本実行の`current-task.tsv`＋評価記録4ファイルが staged/unstaged。開始時点の既存変更（親の登録分）で本項目の成果物に不含。実装成果物なし。
- タスク登録: 親が正典登録済み（`current-task.tsv` に行存在）。子は加えて登録しない（子は記録ファイルのみ編集可）。個別タスクID = 項目ID（ファイル名 `2026-11-05-assess-20261105-0024-phase3-gallery-info-cache.md`）。本項目ファイルと重なる進行中実装タスクなし（Phase 1実装タスクは実装開始指示待ち・gate/guard領域非接触）。

### 現行実装（実コード確認）

- `viewer.info()`（`app/viewer.go:965-986`）: 参照はメモリ `infoCache` のみ。miss時 `client.Gallery(id)` へ直接、disk層なし。`infoCache` は C-05/A-14 でbounded化済み（LRU entry上限＋size cap、`defaultInfoCacheMaxBytes = 4 << 20` `app/viewer.go:76-81`）だがメモリのみ → 項目主張「再起動で消失」成立（コード確認で確定）。
- 検索カード: `searchCard`（`app/viewer.go:578-595`）→ `searchResult`（`597-627`）は `Tags`/`Artists`/`Groups`/`Characters`/`Files`(hash・name・width・height・hasavif)/`scene_indexes` を `info()` から生成。`x-cache` はメモリ有無のみ（`585-592`）。
- DB `galleries`: `schema.sql:1-9` の7列（gallery_id/title/author/lang/dir/first_seen/last_seen）のみ。実DB `hitomi.db` 実測（119行）でも同7列、tags/files/hash相当列は全テーブル皆無 → 「カード生成に使えない」主張成立。`pageorder` はダウンロード済みページ分hashのみでカード全項目の代替にならない。
- `Client.Gallery`（`app/hitomi.go:614-636`）: `ltn.…/galleries/<id>.js` を `c.get` 経由（データ側ゲート `MinIntervalMS`=1000ms、`config.example.json`）でparse。メモリhit時はゲート非消費（network自体発生せず）。metricsは `classifyFetchURL` のkind `gallery`（`app/metrics.go:28-31`）でカウントされ、検索レスポンスにkind別snapshot同梱（`app/viewer.go:556`）→ 実機確認「再起動後remote呼出ゼロ」はkind `gallery` で測定可能。
- CLI: `main.go:953`（gallery）・`970`（download）は `client.Gallery` を直接呼び `viewer.info()` を経由しない → viewer層のみ実装ならCLIは非共有（実測確認）。

### 有効性

**有効**（両主張を実コード＋DB実測で確定。機構の目的—永続hitでゲート消費消去—は現状構造で正しく成立し、既存流儀で実装可能）。

### 推奨（判断ルール上「推奨」として記録。decision記録は実装段階で作成）

1. 保存方式: **file方式**（cache root配下 `cache/galleryinfo/<id>.json`、parse済み `GalleryInfo` JSON＋取得時刻、1 gallery 1ファイル）。理由: 既存cache配置の流儀と同型（正典decision `docs/ai/decisions/2026-11-01-viewer-persistent-image-cache.md` の `cache/images` と同型）、DBスキーマ変更なし、移植性必須条件（完成フォルダ内書き込み）に自走、壊れたファイルの除去が容易。DB新テーブル案は非破壊可能だがスキーマ移行・backup方針を要す → 非推奨。
2. 参照順: memory → 永続 → remote。永続hitはmemoryへ載せ、remote成功は両方へ書く。実装は `info()` 本体（共有helper）に置くこと — こうすればPhase 2の先読み成果も自動で永続に残り、Phase 2側の追加作業が不要になる（タスクの独立条件に合致）。
3. TTL 30日: 定数置き場は `app/hitomi.go:43-46` の既存TTL定数ブロック流儀（例 `galleryInfoCacheTTL = 30 * 24 * time.Hour`）。TTL切れはremote再取得。stale暫定表示は**採用を推奨**: remote失敗時stale提供＋fetch metrics外カウンター（metricsSnapshot拡張）またはログ行で判別（fetch metricsはnetwork呼び出し基準なのでstale hitはkind `gallery` callsに現れない。カウンターの形は実装選択）。
4. 上限・evict: 件数5000または総容量256MBの先に到達した方、取得時刻古い順evict（書き込み時超過分間引き、Directory scanは超過時のみ。取得時刻はJSON field正・mtime fallback）。C-05/A-14流儀（bounded cap＋LRU）と矛盾しない: disk層はaccess orderを持たず「取得時刻順」で正（memory LRUとは層が別）。テストはinsert時刻固定で決定化（既知issue `docs/ai/known-issues/archive/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md` の教訓）。
5. atomic書き込み: 既存A-07a流儀（temp+rename、`writeImageCacheAtomically` `app/viewer.go:350-452` と同型、tempは `cache/galleryinfo` 内）。parse失敗は除去してremote再取得（壊れた内容を根拠にエラーを返さない）。
6. CLI共有: viewer層必須。CLI（`main.go` gallery/download）は `client.Gallery` 直呼びで非共有。共有には `loadGalleryInfo(client, cacheDir, id)` 程度のhelper wiring（またはClient層永続化）が必要＝構造変更。タスクの「無理に広げない」条に従い、**実装段階の申し送り（任意）**として残す。
7. フル画像ガード: 本項目はguard領域（URL組立・gg・imageProxy・download・x-cache表示）を一切触らない。メタJSONのみ・cache/galleryinfo専用の置き場で分離維持。

### 文書矛盾（記録）

`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` の「実装開始指示未受領・未実施」の記述は実コード（`app/search.html` 存在）と新しい実行記録（`2026-11-04-recommendations-approved-execution.md`、commit `5f9d8a7`）と矛盾。日付の新しい文書＋実コードを優先（本項目の有効性には影響なし）。

### 推定規模・優先順位付けに効く要素

- 規模: 中（Go実装 約80〜150行＋mockテスト4件 約150〜250行＋decision記録＋canonical build実機確認）。
- 重要度: 高（ゲート消費を再起動横断で消す目的そのもの。Phase 2のbenefitが本項目経由で永続へ及ぶ）。
- 緊急度: 中（依存なし・独立。Phase 2より先に行うとPhase 2成果が自動永続化）。
- リスク: 低〜中（disk cacheの件数/容量上限は既存disk cacheに前例なし＝新流儀確立。LRU系テストのtie決定化。guard分離）。
- 依存: なし（Phase 2/4と独立）。既存流儀依存（A-07a atomic、C-05/A-14 bounded、cache root）。

### 親控（委譲結果確認）

- 要約と記録有無確認済み: 有効性「有効」（両主張を実コード＋DB実測で確定・判断保留なし）。推奨=file方式・参照順3層・TTL30日・上限5000/256MB・stale採用推奨。記録あり（本ファイル）。実機確認は未実装のため未確認（実装段階の合格条件へ移行）。

## Verification

- 実測（DBスキーマ照合）: `python docs/ai/tmp/assess0024_p3_dbcheck.py`（read-only）→ `galleries` 7列・119行・tags/files/hash列なし（全テーブル）。上記引用どおり一致。scratch（gitignore）: `docs/ai/tmp/assess0024_p3_dbcheck.py`。
- `go vet .` pass。`go test -count=1 -timeout 300s .`（app）pass・16.850s。
- フル画像ガードの守るテスト個別pass（単独run 0.163s）: `TestCanonicalGGRuleComposition` PASS / `TestStorageURLUsesFormatNotNameExt` PASS / `TestThumbnailURLMatchesVerifiedTNPath` PASS。TN非混入系もsuite内でpass（例 `TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing` `viewer_test.go:306`、`TestViewerThumbProxyTNOnlyDoesNotServeFullCache` `viewer_test.go:1078`、`TestViewerImageProxyWritesPersistentCacheAndServesCacheHit` `viewer_test.go:388`）。
- 実機確認（再起動前後metrics・表示一致）: **未確認**（本項目の挙動未実装のため前後比較不可。canonical build・正典serverでの実機確認は実装段階の合格条件3へ移行。画面キャプチャも挙動未確立のため未適用・未確認として記録）。
- 子テスト4件（永続hitでremote呼出なし／TTL切れ再取得／壊れたファイル除去・再取得／上限超過evict）は実装段階でmock方式（既存 `client.fetch` hook）で実施・記録予定。コードは変更していない（変更ファイルは本記録とスクラッチのみ）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
