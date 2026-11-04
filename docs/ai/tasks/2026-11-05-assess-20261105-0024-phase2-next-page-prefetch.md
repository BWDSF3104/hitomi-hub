# Remote Task Phase 2 — 次ページのアイドル先読み（調査・評価）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 子の調査結果待ち（フォアグラウンド委譲済み）

## 意図 / 背景

- 項目ID: `2026-11-05-hitomi-hub-search-phase2-next-page-prefetch`（実行固有ID `assess-20261105-0024` の項目1/3）
- リモートタスク原文（正典）: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase2-next-page-prefetch.md`
- 個別タスク記録（このファイル）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase2-next-page-prefetch.md`。評価記録一覧: `docs/ai/tasks/2026-11-05-assess-20261105-0024-items.md`

## 調査結果 / 変更内容

実行固有ID: `assess-20261105-0024` / 項目ID: `2026-11-05-hitomi-hub-search-phase2-next-page-prefetch`（項目1/3）。有効性: **有効**（項目の主張は実コード・実測で確認。判断保留なし）。ソースコードは無変更。

### 棚卸し

- HEAD: `2871fc8`（基準 `7160dae` との差分は docs 3ファイルのみ＝`app/`・`tests/` のコードは基準と同一。実コード優先の照合: 差異なし）
- `git status`: 既存の未コミット変更は親の記録ファイル4件（items・phase2/3/4 記録）。本タスク成果物に含めない
- 個別タスク登録: 親が正典 `task_manager.py` で登録済み（このファイルがタスク記録。Status/Next Action は親側）。タスクID: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase2-next-page-prefetch.md`
- 正典 server 起動確認: `hub.exe` PID 19316・8731 応答200。起動コマンドは `hub.exe view`（正典コマンドの `--data-dir/--root/--port` なし＝既定値）。`tests/test_canonical_build_freshness.py` pass により稼働 exe は現行 `app/search.html` を embed → served 画面確認は有効（起動フラグの逸脱は観察として記録）
- 対象ファイル重叠: Phase 1 実装タスク（`docs/ai/tasks/2026-11-05-phase1-visible-priority-implementation.md`・未開始）が `app/search.html` を共有。既存実施計画の順序（Phase 1 優先1）に従う。Phase 2 は「取得スケジューラ」のみで Phase 1 の「表示順序付け」と論理競合なし（同じ file _edit_順序の調整のみ）。Phase 3（gallery 情報の永続キャッシュ）は未実施だが、先読みは `viewer.info()`（memory `infoCache`）経由で乗るため実施済みでも競合しない

### 主張の照合（実コード）

1. **search 側に先読み機構なし**: 確認。`app/search.html` に scheduler なし。次ページは `setSearchPage()`（前/次クリック）→ `runSearch()` で始めて取得。UI の `prefetch`/`priority` 出現は viewer.html の `startFullPrefetch`（対象外）と search.html の「pure TN cover」注释のみ
2. **colon term は nozomi 全件取得＋memory cache（TTL 1時間）**: 確認。`hitomi.go` `searchCacheTTL = time.Hour`、`SearchNozomiParts` cache key = `area|tag|language`（order 含む形あり）。**ページ番号は key に含まれない**（`searchPageSlice` はクライアント側切片）→ 次ページの ID 一覧は同一 cache entry から取得＝実質タダ。同一クエリ形式の先読みは cache hit（ゲート消費なし）
3. **`/api/search-card/<id>` は `viewer.info()` 経由で `infoCache`（LRU 上限 256 entries・4 MiB）に入る**: 確認（`viewer.go` `searchCard`/`info`/`boundedCachePut`）
4. **TN は thumb 経路で TN cache に保存**: 確認（`thumbProxy` → `cacheRoot/thumbs/<hash>.<format>.<variant>`、disk 永続。key は hash+format+variant のみ）
5. **ゲート**: `defaultHitomiConfig()` data 1000ms / image 250ms、システムごとに単一タイムスタンプ（`Client.last` / `Client.imageLast`）。フォアグラウンド card 並行度4（`SEARCH_CARD_CONCURRENCY = 4`）、リトライ方針（429/5xx・MaxRetries 3・backoff 2000ms×(attempt+1)）確認
6. **世代管理**: `searchGeneration` 既存。先読みはこれに紐づけ可能

### 実測（正典 server・外部呼出し最小、2026-11-05 0:33–0:37 JST）

- query `female:yuri`（nozomi cache warm 済み）: page1 `x-search-timing: search=0`（外部呼出しなし）。page2 も `search=0`・nozomi calls 増分 0 → **主張2 live 確認**
- infoCache: cold id 1件 miss（remote gallery 1呼出し・gate 消費）→ 同一 id 再呼出し `x-cache: cache`・16ms・metrics 増分なし → **主張3 live 確認（cache hit はゲートを通らない）**
- TN: card JSON の thumbnail URL 初回 `tn-cache` hit 16ms（disk 永続 cache 已温）→ **主張4 live 確認（同一 URL を叩けば温まる/乗る）**
- **マーカー no-op 確認**: `?priority=background` 付き `/api/search-card/<id>`（body 一致・x-cache 不変）、`&priority=background` 付き `/api/thumb/...`（tn-cache hit 不変）、`&priority=background` 付き `/api/search`（ids 一致・metrics 増分なし）。cache key（infoCache=id・TN path=hash+format+variant・searchCache=area|tag|lang）は余剰 query param で変わらない → **マーカー方式は query param が安全。既存の応答・キャッシュ挙動不変（実測で確定）**
- **データ側ゲート 1000ms live 確認**: 連続2 remote gallery 呼出しで 1件目 203ms（gate 待ち0）、2件目 969ms（1ゲート枠の待ち＋network）。cache hit（31ms）はゲート非消費。metrics gallery gate_wait 増分 761ms
- **UI 画面確認（fresh tab・strict 条件、resize 不使用）**: `#results` card 25 = server `ids.length`、`#query`/`#search` 重複なし、`.side` 480px（CSS 契約）、UI network 応答 = server 直接応答一致（`server search=0`、page1 ids 先頭5件一致）。カード完了後4秒アイドルで追加リクエスト0件（=先読み機構なし）。**「次」クリック後にのみ `page=2` の `/api/search` が発行**（ネットワーク list で確認）。ページ2は cold 2件（約1ゲート枠969ms）を含むため cold 経路で15秒超（先読み実装後の合格条件1秒は warm 経路の実測で、実装前には測れない）

### 実現可能性と推奨（実装方法の要点）

- 推奨: スケジューラは `app/search.html` 側に閉じる。発火=「`loadSearchCards` 完了＋800ms アイドル」（定数公開で node harness 固定可能にする）。即時発火は `#search-next` の hover/focus。対象=page+1 のみ、`searchTotalPages` 超で停止。取得順は ID（cache hit なら外部呼出しなし）→ `/api/search-card/<id>`（並行度2）→ TN cover（同一 URL 形式＋マーカー）
- 中断: `searchGeneration` 変更（新規検索/ページ移動/gallery 開）・query 入力イベント・`visibilitychange` hidden・429/5xx→全体停止＋5分クールダウン（メモリ保持）。1リクエスト完了ごとに中断条件を確認（リクエスト単位で譲る）
- 推奨マーカー: query param `priority=background`（実測で応答/キャッシュ不変を確定済み。既存 `api()` に無理なし）。サーバー側は metrics の kind 区分（例 `gallery.background`）など最小加算のみ。ゲート制御は Phase 4 範囲で実施しない
- テスト: 現行方式（node harness + pytest wrapper）で scheduler 挙動を固定可能。既存 harness の `bodyIn` 抽出パターンに載せる（実装時にテスト抽出用の関数境界を整える必要あり）

### 副作用・影響範囲・リスク

- サーバー側ゲートは system ごとに単一タイムスタンプ共有: 先読みは data 枠を前倒しで消費する（総数は増えない）。中断条件で「発行済み1枠」をキャンセルできないため、競合時フォアグラウンド最大1ゲート枠待ち＝合格条件の設計と整合
- infoCache LRU（cap 256/4 MiB): 先読み25件 insert で cap 超過時、最古 access が eviction される（現ページは直前 access で protected）。TN disk cache は永続で「冷えた状態」の定義に TN warm が残留する点を確認済み（実測で tn-cache hit）
- viewer 側 `startFullPrefetch`・フル画像ガード・`x-cache` 判定は非接触（実測でも触れていない）
- 429/5xx のクールダウンは UI スケジューラ側のみ。サーバー Client のリトライ方針とは分離（変更しない）

優先順位付けへの示唆: 重要度中・緊急度中・リスク低〜中（ゲート共有の競合は設計で吸収）・依存なし（Phase 1 と file 競合のみ、Phase 3 の実装済み/未実施どちらでも兼容）。

### 親控（委譲結果確認）

- 要約と記録有無確認済み: 有効性「有効」（実測で閉じ・判断保留なし）。主張6点すべて実コード一致、マーカー no-op・cache hit 非ゲート消費は実測確定。記録あり（本ファイル）。

## Verification

- `go vet ./...`（app）: pass（output なし）
- `go test -count=1 -timeout 300s .`（app）: pass（17.164s）
- `python -m pytest tests/ -v`: **51 passed（9.84s）**。内訳・要点: `test_canonical_build_freshness.py` 2件 pass（稼働 `hub.exe` は現行 `app/viewer.html`・`app/search.html` を embed）、`test_read_lists_debounce.py` pass（node harness 現行方式）、strip/scan 系 pass
- フル画像ガードの守るテスト（個別 pass 明記・サーバー変更はないが項目条件により記録）: `TestStorageURLUsesFormatNotNameExt` PASS・`TestCanonicalGGRuleComposition` PASS・`TestThumbnailURLMatchesVerifiedTNPath` PASS（個別実行 0.157s）。viewer の imageProxy・TN 非混入系（`TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing` 他、`TestImageProxyCacheStateIsNotTN`、`TestViewerThumbProxyTNOnlyDoesNotServeFullCache`）は suite pass 内
- 実測（正典 server 8731・PID 19316、scratch `docs/ai/tmp/phase2_measure.py`・`phase2_measure2.py`）: 上「実測」節の値（search=0 保持、infoCache miss→cache 16ms、TN tn-cache 16ms、マーカー no-op、データゲート 969ms、metrics gallery 77→79・gate_wait +761ms、image calls 増分なし）
- 画面確認（fresh tab・strict 条件・resize 不使用）: card 25=ids.length、重複 id なし、`.side` 480px、アイドル4秒で追加リクエスト0件、次クリック後にのみ page=2 取得。保存先（タスク別 Google Drive フォルダ）: `I:\マイドライブ\kilo\2026-11-05-assess-20261105-0024-phase2-next-page-prefetch`（`phase2-baseline-search-page-20261105.png`・`phase2-baseline-next-page-on-click-20261105.png`・`phase2-baseline-search-page-confirmation-20261105.txt`）
- 正典文書の矛盾処理: Google Drive 保存は「date folder」（`2026-11-02-google-drive-confirmation-storage-procedure.md`）と「タスク別 folder」（`2026-11-02-google-drive-task-specific-screenshot-folder.md`）が同日で矛盾。後者（future tasks へ適用範囲を明示）を採用。work-pre-check decision 3点（`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`）は読了済み（viewer/download 接触なし）
- 未確認点: 実装後の合格条件（先読み完了後の1秒以内遷移・ベースライン競合実測）はコード未実装のため未測定（実装時測定）。実装前ベースラインは上実測で確定済み

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
