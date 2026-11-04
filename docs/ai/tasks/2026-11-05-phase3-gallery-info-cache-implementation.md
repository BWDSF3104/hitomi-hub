# Phase 3 実装 — gallery 情報の永続キャッシュ（優先1・未開始）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 実装開始（委譲承認・リモートタスク発行が承認）。推奨案採用: file方式 cache/galleryinfo・3層参照・TTL30日・上限5000/256MB 書込時間引き・stale暫定表示採用（ログ+カウンター）

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache.md`）の実装タスク。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 1（3項目中。全体順 Phase 1→**P3**→P2→P4。根拠は評価記録の記載のみ。重要度高＝ゲート消費そのものを消す・依存なし）。
- 評価記録（有効性「有効」・実コード＋DB 実測で確定・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase3-gallery-info-cache.md`。
- 検討結果の反映（推奨）: file 方式 `cache/galleryinfo/<id>.json`（parse 済み `GalleryInfo` JSON＋取得時刻・atomic temp+rename A-07a 流儀）。参照順=memory `infoCache`→永続→remote（`info()` 本体実装＝Phase 2 成果の自動永続化、Phase 2 側追加作業不要）。TTL 30 日（既存 TTL 定数ブロック流儀）。上限 5000 件/256MB・取得時刻順 evict（書き込み時間引き・テスト決定化）。stale 暫定表示=採用推奨（metrics/ログで stale 判別可）。CLI 共有は非必須（申し送り）。
- 前実行の known-issues（実装前対処）: `docs/ai/known-issues/KI-20261105-01-noncanonical-running-instance-tn-cache.md`（正典 rebuild・正典 server 再起動・再測定）。

## 調査結果 / 変更内容

実行固有ID: `assess-20261105-0024` / 項目ID: `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`（実装委譲・リモートタスク発行が承認）。

### 棚卸し（実装開始時点）

- HEAD `95e1a2c`（Phase 1 合格コミット）・`git status`: clean。既存の未コミット変更なし。
- 個別タスク: 正典 `task_manager.py add` で登録済み（本ファイル）。正典 `update` で進行中化、終了時に `end`。
- 作業前チェック（viewer 接触項目）必读 decision 読了済み（ファイル名明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。canonical build ポインタ `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`・build 正典 `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`。known-issues `KI-20261105-01` 条件（canonical rebuild・正典 server 再起動）守った。
- 評価記録の推奨と実コードは一致（食い違いなし）。実装前に修正方法・影響範囲・合格条件・検証方法確定: 推奨方式（file `cache/galleryinfo/<id>.json`・3層参照・TTL30日・上限5000/256MB 書き込み時間引き・stale 採用）をそのまま採用。

### 変更内容（採用方式と理由）

- `app/galleryinfocache.go`（新規）: disk 層。置き場 `cache/galleryinfo/<id>.json`（1 gallery 1 ファイル、parse 済み `GalleryInfo` JSON + `fetched_at`）。atomic 書き込み A-07a 流儀（同 dir temp+rename）。壊れたファイル（parse 不可・id 不一致）は除去して remote 再取得（error を返さない）。上限 5000/256MB、取得時刻古い順 evict（書き込み時間引き・scan は超過時・fetched_at JSON field 正 mtime fallback）。stale 暫定表示採用（remote 失敗時のみ、`infoDiskStaleHits` counter + stderr ログで判別）。
- `app/viewer.go`: `info()` を memory → disk → remote の 3 層へ。disk hit は memory へ載せ network（ゲート）非消費。remote 成功は memory・永続両方へ書く。viewer 構造体 disk 層フィールド + `defaultInfoDiskMaxEntries = 5000`/`defaultInfoDiskMaxBytes = 256 << 20`（C-05/A-14 bounded 流儀と同思想）。`infoCacheRoot = root + orDefault(cfg.CacheDir, "cache") + /galleryinfo`。
- `app/hitomi.go`: TTL 定数ブロックに `galleryInfoCacheTTL = 30 * 24 * time.Hour` 追加。
- decision 記録: `docs/ai/decisions/2026-11-05-phase3-gallery-info-persistent-cache-file-store.md`（file 方式採用と理由、TTL・上限・evict・stale 採否）。
- フル画像ガード: 非接触。メタ JSON 専用の置き場で画像バイト経路（imageProxy・TN・full cache）と分離。ゲート間隔（1000/250ms）・リトライ方針変更なし。検索カード `x-cache`（memory 有無）は変更なし。
- 既存テスト 1 件再アンカー（影響・申し送り）: `TestViewerInfoCacheEvictsLRUEntriesWhenOverEntryLimit` の「memory evict された key は refetch」assertion を disk 層導入により「永続層 hit で network 消費なし」（fetches 5→4）へ変更。memory bounded assertion は維持。これは本タスクの効果そのもの。C-05/A-14 合格項目のファイル内テスト変更なので結果で持ち帰る（項目自体は再オープンしない）。
- CLI 共有: 広げなかった（構造変更なし。`client.Gallery` 直呼びは非共有のまま）。申し送り: 将来 `loadGalleryInfo` 程度の共有 helper で disk 層を利用可。Phase 2（未実施）は `info()` 本体実装により側追加作業不要の形。
- 文書矛盾: 評価記録済みの `2026-11-04-search-viewer-separation-approach-approved.md`（実装開始指示未受領表記）と実コード（`app/search.html` 存在・Phase 1 合格コミット `95e1a2c`）の矛盾は日付の新しい実行記録＋実コード優先で解決済み（本実装はリモートタスク発行＝承認の範囲）。

## Verification

- `go vet .`（app）: pass。`go test -count=1 -timeout 300s .`: **pass（17.938s）**。`python -m pytest tests/ -v`: **52 passed**（canonical build freshness 含む）。
- mock テスト（外部 API 呼出なし・`client.fetch` hook）個別 pass（0.687s run）:
  - `TestViewerInfoDiskPersistentHitDoesNotCallRemote` PASS（永続 hit・fetch 1 のまま・memory 載せ確認）
  - `TestViewerInfoDiskTTLExpiredRefetchesAndUpdatesPersistentFile` PASS（31日前 fetched_at → 再取得・ファイル更新）
  - `TestViewerInfoDiskCorruptedFileRemovedAndRefetched` PASS（parse 不可・id 不一致除去・再取得）
  - `TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap` PASS（insert 時刻固定・決定化・count=3）
  - （補助・stale 採用の固定）`TestViewerInfoDiskStaleEntryUsedOnlyWhenRemoteFails` PASS（counter=1・ログ行）
- フル画像ガードの守るテスト個別 pass: `TestStorageURLUsesFormatNotNameExt`・`TestCanonicalGGRuleComposition`・`TestThumbnailURLMatchesVerifiedTNPath`・`TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing`・`TestViewerImageProxyWritesPersistentCacheAndServesCacheHit`・`TestViewerThumbProxyTNOnlyDoesNotServeFullCache`・`TestImageProxyCacheStateIsNotTN`（個別 run 0.687s・全 PASS）。再アンカー後 `TestViewerInfoCacheEvictsLRUEntriesWhenOverEntryLimit` PASS。
- 実機確認（正典 build 作り直し・正典 server・fresh tab・resize 前・strict 条件）: 合格。
  - canonical rebuild: `go build -o ...output\hitomi-hub\hub.exe .`、既知問題 KI-20261105-01 の推奨対処（正典と保証できない起動中インスタンス PID 1256 停止→正典 exe で正典 server 再起動・runtime root `Common_Operations\scratch\p3_canonical_fresh`）。
  - server-direct baseline（UI 計測前）: `GET /api/search?q=kemonomimi` → 200・`x-search-timing: search=7475;record=35;json=1`（stale cache 応答なし）。
  - 再起動前（fresh プロセス・disk 空・fresh tab 実検索 `kemonomimi` page 1）: UI network response ids 25 = カード表示 25。UI search 応答 metrics kind `gallery` calls=27（route 復刻分の remote 呼出）→ カード表示後 server-direct metrics kind `gallery` calls=52（gate_wait 186889ms）。kemonomimi カード 25 件は x-cache miss（remote 取得）。
  - 永続ファイル実測: `cache/galleryinfo` 50 ファイル（例 `4283465.json` 5470B、対象 ID `4304794.json` 含む）。
  - server 再起動（同じ正典 exe・同じ root）→ fresh tab で同じ検索（実検索 search=8666ms）: UI search 応答 metrics に kind `gallery` なし（calls=0）、カード表示後の server-direct metrics も kind `gallery` calls=0 → 対象 ID の remote 取得が発生していない。cards 25 1652ms（1件平均約66ms＝1000ms ゲート非消費）。表示内容一致: target ID `4304794`（title `9月-02`・tags 3・files 2・hash `bb2681ae…`）の search-card 応答本文が再起動前後で一致。
  - フル画像ガード従来手順（canonical build 実機・guard 非接触の確認）: gallery `4284639`・`hub.exe download --id 4284639 --limit 3` → attempted=3・written=3（skipped=0）。`/api/image/<hash>?format=webp`: `54a89686…` 200 x-cache cache 137802B（TN 23688B と同一でない）/ `a55d9b2f…` 200 x-cache miss 39230B（TN 10476B と同一でない）/ `f99065e5…` 200 x-cache miss 32610B（TN 側 no_tn 502）。imageProxy の x-cache は miss/cache/local のみ（`tn-cache` なし）。KI-20261105-01 の異常は正典 build で再現せず（C-par2 の exact hash は P4 記録に未記録→同 hash の再照会のみ未完了。推奨対処＝canonical rebuild・正典 server 再起動・再測定は実施）。KI は未解決のまま（archive mv しない）。
  - 画面キャプチャ保存先（タスク別 Google Drive フォルダ・正典 `2026-11-02-google-drive-task-specific-screenshot-folder.md`）: `I:\マイドライブ\kilo\2026-11-05-phase3-gallery-info-cache-implementation\`（`phase3-cards-before-restart-20261105.png`・`phase3-cards-after-restart-20261105.png`・`phase3-gallery-info-cache-confirmation-20261105.txt`）。repository 内スクリーンショットは保存せず（kilo-safe-remove で除去）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
