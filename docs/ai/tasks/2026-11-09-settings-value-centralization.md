# 設定値の集約（正典単一出力元と欠落キー・page定数）

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: 実装開始指示待ち。確定済み範囲: dir 既定の単一出力元化（galleryDir() 直書き解消含む）、example へ dir 2 key、Python DEFAULT_LIMITS 廃止、一致テストを limits・viewer・dir へ拡張、disk cap 4000 のテスト re-anchor

## 意図 / 背景

設定値の正典が複数場所に分散している。集約の範囲と、設定化する値／定数として正典に留める値の線引きを決める。Web UI からの変更（別検討）は この線引きに依存する。

## 調査結果 / 変更内容

実測（HEAD 照合）:

- 設定ファイルは 1 本で確定済み（正典）。`config.json` primary、missing 時 `config.example.json` fallback（`app/main.go:219-230`、decision `2026-11-03-d02-config-copyable-nontracked.md`）。
- 単一出力元パターンは hitomi 節だけ成立: `defaultHitomiConfig()`（`app/hitomi.go:278-281`）が正典、example は写し、`TestConfigExampleHitomiKeysMatchCodeDefaults`（`app/main_test.go:1546`）で自動一致。viewer 節は 1 キーのみ一致テスト（`app/main_test.go:1624`）。docs は pointer 化済み（`docs/ai/usage-memo.md:152`）。
- dir 既定値が call site ごとの inline literal: db `app/main.go:1108`、gallery_dir `app/main.go:912`・`1121`、`app/export.go:95`、`app/pdf.go:288`、acquisition `app/main.go:200-202`、cache_dir `app/viewer.go:126`・`130`。
- **設定値が効かない書き込み口**: `galleryDir()` が `"Galleries"` 直書き（`app/library.go:153-155`）。`galleries.dir` 記録はここを通る（30+ call site）ため、`gallery_dir` を変えると disk path と DB 記録がずれる。
- **`cache_dir` の形が非整合**: full cache は `<cache_dir>`（既定 `cache/images`、`app/viewer.go:126`）、galleryinfo は `<cache_dir>/galleryinfo`（`app/viewer.go:130`）。既定名に `images` を含むため、フォルダ名だけを設定すると full cache 先の意味が変わる（AGENTS.md の永続先条と関連）。
- Python 参照実装が独立の既定表を持つ: `DEFAULT_LIMITS`（`src/scan.py:22`、`431-432`）対 Go `defaultLimits()`（`app/main.go:90-101`）。一致テストなし。
- 設定キーを持たない既定値: `defaultCacheDiskMaxEntries` 8000（`app/main.go:157`、`app/viewer.go:132`）、`defaultNoFullCacheMaxEntries` 1024（`app/hitomi.go:43`、`335`）、floor `minHitomiRateFloorMS` 50（`app/main.go:151`）、`minViewerMaxObjectURLs` 1000（`app/main.go:156`）、`schemaVersion`（`app/main.go:247`）、DB pragmas（`app/main.go:292`）。
- page 側定数は config の外: `app/search.html:177-180`、`529-530`、`861-864`、`1013`、`app/viewer.html:207-208`、`224`。config→page は `viewer.max_object_urls` のみ（`GET /api/viewer-config`、`app/viewer.go:557-561`、`app/viewer.html:1494`）。
- `config.example.json` の欠落キー: `cache_dir`・`acquisition_dir`（rate key 欠落は `known-issues/archive/KI-20261105-07-...` で是正済み、dir key は未反映）。

集約で更新が必要になるアンカー: `app/main_test.go:1546`、`1578-1585`、`1624`、`1355`、`app/hitomi_test.go:125-126`、`app/viewer_test.go:4658-4670`・`4892`（page 行照合）、`tests/test_scan.py:417`（`DEFAULT_LIMITS` 参照）、docs pointer（`docs/ai/usage-memo.md:152`、`docs/ai/api-rate-limits.md`）。

決定点（未確定・ユーザー選択が必要）:
1. 正典を「コード既定」のままにするか（既存 pattern・テストがその形）、JSON を正典にしてコードは fallback に降げるか。
2. 設定化しない値（8000・1024・floor・pragmas・page 定数）を「定数正典」と宣言するか。宣言するなら example に key を置かない — 欠落 key を埋める作業と矛盾するため、埋めるなら「設定化可能」と宣言する。
3. `cache_dir` の意味を 1 つに固定する（full image 用と galleryinfo 用を分離するか、`<cache_dir>` 直下へ統一するか）。

## 決定点の解（2026-11-09）

- R-15 決定点 1（disk cap の単位）: 件数・既定 4000 で決定（ユーザー 2026-11-09 00:11 UTC）。`docs/ai/decisions/2026-11-09-disk-cache-cap-count-4000.md`。既存実装が件数判定（DB 行 COUNT・kind 単位・作成順）なので、決定で変わる値は 8000 → 4000、テストアンカー `app/main_test.go:1584-1585`、コメント `app/viewer.go:598` のみ。

## 集約対象の確定案（2026-11-09）

単一出力元へ寄せる（集約対象）:
- dir 既定 4 件（`db`・`gallery_dir`・`cache_dir`・`acquisition_dir`）を `defaultConfig()` へ。現在 call site ごとの inline literal（`main.go:1108`、`912`、`1121`、`export.go:95`、`pdf.go:288`、`main.go:200-202`、`viewer.go:126,130`）
- `galleryDir()` の `"Galleries"` 直書きを cfg 経由へ（`app/library.go:153-155`）
- `config.example.json` の欠落 2 key（`cache_dir`・`acquisition_dir`）
- example とコード既定の一致テストを hitomi 以外の節（limits・viewer・dir）へ拡張（既存形: `TestConfigExampleHitomiKeysMatchCodeDefaults` `main_test.go:1546`）
- Python の自前既定表 `DEFAULT_LIMITS`（`src/scan.py:22`）を廃止し、同一 `config.example.json` を読む形へ

定数正典として code に残す（設定化しない）:
- disk full/thumbs cap 4000 件（kind 単位・作成順）— `decisions/2026-11-09-disk-cache-cap-count-4000.md`
- galleryinfo disk cap 5000 件 + 256 MiB（`viewer.go:141-142`）。full/thumbs は件数だけなので単位が混在
- memory cache cap: search 256/8 MiB、index data 512/8 MiB、index top 1024/1 MiB、index node 1024/1 MiB、galleryinfo 256/4 MiB（`hitomi.go:75-85`、`viewer.go:137-138`）
- no_full cap 1024（`hitomi.go:43`）。TTL 超過分だけを掃除。hard cap で active entry まで消す形は C-08 として未承認
- floor 50 ms・1000 件、TTL（5分・1h・30日・2h）、`schemaVersion`、DB pragmas
- page 定数（concurrency・idle・ahead・cooldown・eager cover）、localStorage keys、表示ラベル、`TYPE_FILTER_OPTIONS`

設定化済み（例外・維持）: `viewer.max_object_urls` 4000

削除禁止の正典条件（2026-11-09 記録）: `decisions/2026-11-09-completion-folders-no-delete-rule.md`。実測で `Galleries`・`acquisition` へ向く削除経路は 0 件。prune は cache 層のみ（`cacheEntryPath` は cache root 内のみ解く、`viewer.go:591-596`）。

副決定点（確定・ユーザー承認 2026-11-09 00:45 UTC）:
1. disk cap 4000 は層（kind）ごとに 4000 件（確定）
2. 捨て順は作成順のまま。last access 列は置かない（確定）
3. galleryinfo disk cap は件数 + 256 MiB を維持（確定）
4. Python の `DEFAULT_LIMITS` は廃止し、同一 `config.example.json` を読む形へ（確定）
5. page 側定数は page 正典として残す。config へ移さない（確定）

集約範囲の確定: 名前の既定値（`db`・`gallery_dir`・`cache_dir`・`acquisition_dir`）だけ `defaultConfig()` へ単一出力元。cap・floor・TTL・`schemaVersion`・pragmas・page 定数は定数正典としてコードに残す。`config.example.json` に加える key は dir 2 件のみ。決定記録: `docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md`。

実装開始指示待ち。実装時に更新が必要: `app/main_test.go:1584-1585`、`app/viewer.go:598`、`tests/test_scan.py:417`、`config.example.json`、一致テストの limits・viewer・dir への拡張。

## 変更内容（実装 2026-11-09）

- `app/main.go`: 名前の既定値の単一出力元を新設（`defaultDBName`・`defaultGalleryDirName`・`defaultCacheDirName`・`defaultInfoCacheDirName`・`defaultAcquisitionDirName`）と Config accessor（`DBName()`・`GalleryDirName()`・`CacheDirName()`・`InfoCacheDirName()`・`AcquisitionDirName()`）。call site ごとの inline literal を accessor へ統一（`main.go` scan・DB path、`export.go`、`pdf.go`、`viewer.go`）。`loadConfig` の acquisition 直書き既定を削除（accessor が解決）。
- `app/library.go`: `galleryDir(cfg, galleryID)` へ変更（`"Galleries"` 直書き廃止）。`root` 引数は DB 記録へ使わないため削除。
- 波及した signature: `recordScanFile(db, cfg, ...)`、`recordDiscoveredBatch(db, cfg, ids)`、`recordDiscoveredBatchTx(t, cfg, ids)`、`runSavedQueries(db, client, cfg, lang, limit)`。テスト call site 更新（`main_test.go`・`viewer_test.go`・`firststatus_test.go`・`hitomi_test.go`・`check_test.go`）。
- `config.example.json`（正典）へ `cache_dir`・`acquisition_dir` 追加。完成フォルダのコピーへ同期。
- `src/scan.py`: `DEFAULT_LIMITS` 廃止。`limits` を config JSON からだけ読む（欠ければ config error）。`yr` 系フラグの inline 既定値（10・50・67108864）も削除。`gallery_dir` は config JSON の値を使う。
- `tests/test_scan.py:417`: `scan.DEFAULT_LIMITS` 参照を `CFG["limits"]` へ更新。
- disk cap: `defaultCacheDiskMaxEntries` 8000 → 4000（`app/main.go`）。`app/viewer.go` の prune コメント更新。テスト re-anchor `app/main_test.go`（4000 per kind）。
- 追加テスト: `TestConfigExampleDirNamesMatchCodeDefaults`（key 存在＋値一致＋accessor 解決値）、`TestConfigExampleLimitsMatchCodeDefaults`（limits 一致＋設定化しない cap が limits key として現れないこと）。

残る設計点（本項目の範囲外・未実施）: `cache_dir` の形（full cache は `<cache_dir>`、galleryinfo は `<cache_dir>/galleryinfo`）は未確定。コードは現状挙動を変えず、`defaultInfoCacheDirName` に注記した。

## Verification

- `go vet .`（app）clean
- `go test -count=1 -timeout 300s .`（app）: ok 25.515 s
- `python -m pytest tests/ -v`: 60 passed 13.38 s（`test_run_yara_exit1_error_is_scan_error` の更新含む。KI-20261106-04 の failure は今回非発生）
- canonical build 再実行: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（UI 非変更のため served 画面確認は該当なし）
- 未確認: 完成フォルダの実機での dir 変更反映（実機 confirm 未実施）。設定値を変えた場合の既存 DB `files.path` の整合は項目範囲外（前記録 `2026-11-08` 分の議論どおり、path 書き換えはしない）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
