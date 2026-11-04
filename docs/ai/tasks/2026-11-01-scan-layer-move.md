# hitomi-hub 分割とスキャン層の移動（単一正典リポジトリ化）

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: 記録済みの問題は解消（YARA 役割分担・ggset drift → archive）。残りは backlog の P2（#8 AVIF の pdf embed、#9 `bookmark_queries.notify` の用途）。完了判定はユーザーの承認後、`current-task.tsv` の行削除と archive 操作で締める

## 意図 / 背景

hitomi.la 用の作業（検索・閲覧・DL・既読・お気に入り・構造検査）を「コピペで動く完成フォルダ」として独立させたい。目的・ライセンス・リリース単位が汎用作業用の Common_Operations（正典ツール群）と違うため、リポジトリを分割した。単体 exe は版固定＋GitHub SHA-256 digest 検証済みのもののみ受け入れる方針も同じ理由から（正典ツールと混在させない）。

分割先 `S:\Documents\Kilo_Commons\hitomi-hub`、public・MIT。Kilo_Commons 自体は git リポジトリではないため nested repo 問題はなく、正典ルート判定は `hitomi-hub/.git` に解決される。正典ツール（`task_manager.py`・`kilo-safe-*` 等）は Common_Operations の `bin/` を PATH 経由で参照し、プロジェクトごとのコピーは作らない。

## 調査結果 / 変更内容

移動したファイル（Common_Operations からはゴミ箱経由で空ディレクトリを削除）:

- `src/scan.py`、`src/schema.sql`、`src/config.example.json`、`src/rules/structural.yar`（旧 `scripts/hitomi_hub/`）
- `tests/test_scan.py`（旧 `tests/test_hitomi_scan.py`。SCRIPT_DIR を `parents[1]/src` に修正）
- `tools/VERSIONS.md`（単体 exe の版固定と digest）
- `docs/ai/decisions/2026-11-01-hitomi-hub-scan-layer-portable.md`

新規作成: `LICENSE`（MIT）、`README.md`、`AGENTS.md`、`.gitignore`（exe・`*.db`・scratch/ 非追跡）、`docs/ai/`（memory-bank-guide・architecture・backlog・api-rate-limits・workflows）。

Common_Operations 側は `.gitignore` の `tools/*.exe` 行を取り消し（移動により該当ディレクトリが消滅）。

## Verification

- `python -m pytest tests/ -v`（本リポジトリ）→ 13 passed (0.93s)。うち `test_run_yara_rules_on_fixture` は `tools/yr.exe` を実際に呼ぶ integration test（`tools/yr.exe` が無い環境では skip）
- `python -m pytest tests/ -q`（Common_Operations・移動後）→ 109 passed (6.38s)
- `git status` で移動後の混入なし（exe・DB・scratch/ は非追跡）

## checkpoint 2（2026-11-01）: 単体 exe の取得と規則の実行検証

- `yr` v1.22.1 を公式 release から取得。GitHub API の asset digest と SHA-256 一致（`dbe8c1ec…b327`）。`gh attestation verify` は attestation 未登録（404）→ 判断根拠は digest 一致のみ。導入先 `tools/yr.exe`（非追跡）、`yr --version` = `yara-x-cli 1.22.1`
- YARA-X の zip モジュールフィールドを実測（`file_path`・`uncompressed_size`・`compressed_size`）。規則を実測ベースで書き換え
  - `Zip_Path_Traversal`（`..`、`/`、`\`）
  - `Zip_Executable_Entry`（exe/dll/scr/jar/js/vbs/ps1/bat/cmd/lnk）
  - `Zip_Decompression_Ratio`（`uncompressed_size > 100 * compressed_size`。`/` は正規表現デリミタと衝突するため割り算は使わない）
  - `MZ_Header_At_Offset0`（`$mz at 0`）。フル PE ヘッダ検査は scan.py 側のまま
  - `PDF_Structural_Risk`（`uint32be(0) == 0x25504446`）
- `--max-rules` は `yr` に存在しない（classic YARA のフラグ）。scan.py を `yr scan` の有効フラグ（`--timeout`/`--cpu-limit`/`--skip-larger`/`--fast-scan`）に修正。ndjson 解析を `parse_yara_output()` に分離
- 規則実行結果（fixture・`--fast-scan --timeout 10 --cpu-limit 50`）: cbz で `Zip_Path_Traversal`・`Zip_Executable_Entry`・`Zip_Decompression_Ratio`、拡張子偽装 PNG で `MZ_Header_At_Offset0`、PDF で `PDF_Structural_Risk`
- scan.py 端到端（`yr` 設定込み）: hit 3 / pass 0 / scan_error 0。findings に structural 8 件

## checkpoint 3（2026-11-01）: Go 単体静的バイナリ port（アプリ本体）

正典 data ファイルをリポジトリルート（完成フォルダ直下）へ移動: `schema.sql`、`rules/structural.yar`、`config.example.json`。Python 参照実装は `DATA_DIR`（`src` の親）から読む。Go 実装は exe と同フォルダの data ファイルを読む（`--data-dir` で上書き）。

`app/`（Go 実装）:

- `main.go`: CLI（`--root`/`--config`/`--gallery-dir`/`--data-dir`/`--no-vt`/`--verify`）、DB（`database/sql` + `modernc.org/sqlite`）、walk、findings 記録、`verify_missing`、JSON summary
- `scan.go`: magic 判別、zip entry 検査（`max_entries` で切る・`infos[:max_entries]` と同じ）、PDF key 検査、status 決定
- `vt.go`: hash-only lookup、分/日 rate gate、cache、`yr` 呼び出し（有効フラグのみ）、ndjson 解析
- `scan_test.go`、`go.mod`/`go.sum`

parity 方針: Python 参照実装と同じ status 語彙と同じ finding 文字列（`path_traversal:`、`executable_entry:`、`ratio_exceeded:`、`entry_size_exceeded:`、`entry_count_exceeded:`、`magic_mismatch:<ext>_vs_<magic>`、`mz_without_pe_header`、`bad_zip`）。

FTS5 は現行 schema で未使用 → pure-Go ドライバの未確認領域に触れていない。導入する場合は使用前に `modernc.org/sqlite` の FTS5 対応を確認すること。

### Verification（2026-11-01）

- `go vet ./...` → clean
- `go test ./...`（`app/`）→ ok 0.18s（magic 判別、zip findings、decompression ratio、status 語彙と `bad_zip`、VT rate gate（分・日・skipped）、ndjson 解析、data ファイル位置）
- `python -m pytest tests/ -q` → 13 passed（0.89s。data ファイル移動に合わせて `DATA_DIR` を追加）
- parity 点検 `docs/ai/tmp/parity_check.py`（fixture 7 件・`--no-vt`）→ status/findings 完全一致（`mismatches: []`）。両実装 summary: hit 3 / pass 2 / not_evaluated 1 / scan_error 1 / cached 0
- build（`CGO_ENABLED=0 -trimpath -ldflags="-s -w"`）→ 19.5s、`scratch/dist/hub.exe` 10.9 MB（非追跡・exe はコミットしない）。ビルド済み exe を同一 fixture で実行 → summary 一致（exe 自身の検証）

## checkpoint 4（2026-11-01）: 完成品フォルダと機能動作確認

完成品フォルダ `output/hitomi-hub/`（exe + data ファイル）に `使い方メモ.md` を作成。

機能点検（`docs/ai/tmp/feature_check.py`・fixture 4 件・exe 単体・VT key なし・外部 API 呼ばず）:

| 機能 | 結果 |
| --- | --- |
| 検索 | 未実装（hitomi.la 通信口なし） |
| 閲覧 | 未実装（UI なし。DB 参照のみ） |
| 一括ダウンロード | 未実装 |
| 既読管理 | `read_state` テーブル作成は成功。書き込み口なし（0 行） |
| お気に入り | `read_state.favorite` 列あり。書き込み口なし |
| ウイルスチェック（構造） | 動作: hit 2（`path_traversal`・`executable_entry`・`pdf_object:/Launch`）/ pass 1 / not_evaluated 1。files 4 件・findings 3 件を DB に記録 |
| VirusTotal | `no_key` を返し何も書かない（key なし）。rate gate・cache はテストで検証済み |
| `--verify` | 消失ファイルの `external_removal` 更新をテストで確認 |

exe を data ファイルと同フォルダに置いた形（`--data-dir` 省略）で実行 → 参照実装と同一 summary。書き込みはフォルダ内（`hitomi.db`）に閉じる。

- YARA は main ループで実質 no-op（`yr` は構造 findings 無しファイルにのみ走る。現行規則は構造検査と重複）。integration test（`run_yara` 直接呼出し）では規則.hit を確認 → `docs/ai/known-issues/2026-11-01-yara-no-op-in-main-loop.md` に記録

## checkpoint 5（2026-11-01）: 検索・DL・既読・お気に入り層（Go 実装）

追加: `app/hitomi.go`（nozomi 検索・gallery メタ・URL 構築・rate gate）、`app/library.go`（galleries upsert・read_state・favorite・bookmark_queries）、`app/ggset.go`（`gg.js` の 0 集合 2112 件を `docs/ai/tmp/gen_ggset.py` で生成）、CLI サブコマンド（`search`/`gallery`/`download`/`read`/`fav`/`list`）。`processFile()` を scan と download で共有。

endpoint と呼出し方針は `decisions/2026-11-01-hitomi-client-endpoints.md`（実測で確定）。`api-rate-limits.md` の hitomi 行を運用値で更新。

### Verification（2026-11-01）

- `go vet ./...` clean。`go test ./...` → ok（13 tests・0.38s）。external API は呼ばない（`Client.fetch` mock）。テストで確認: URL 構築（実測値 `w2/1793469601/2488/<hash>.webp`）、nozomi decode（big-endian int32）と Range（page 2 → bytes=100-199）、gallery JSON 解析（`id` は文字列）、rate gate、read/favorite/query の DB 書き込み
- live 確認（6 呼出し・間隔 1000 ms）:
  - `search --term "female:yuri" --lang english` → 25 件（page 1・page 2）
  - `gallery --id 4294614` → title `Heroine Profile⑪`・22 files
  - `download --id 4294614 --limit 2` → 2 件保存。files に sha256・magic `webp`・status `not_evaluated` 記録（実体は webp）
  - `read --id 4294614 --page 3` → `last_page=3`。`read --id 4294613 --complete` → `fully_read=1`（gallery 行を先に upsert する修正後）
  - `fav --id 4294614` → `favorite=1`。`list --list unread` / `favorites` / `queries` すべて期待通り
- `ltn.hitomi.la` はこの環境で A レコードなし（解決失敗）。data 正典は `ltn.gold-usergeneratedcontent.net`

## checkpoint 6（2026-11-01）: nozomi の namespace 分区規則と `--url`

`app/hitomi.go` の `Search` を hitomi の `get_galleryids_for_query` と同じ分区規則に修正（実測で確定）:

- `female:`/`male:` → area = `tag`・tag = クエリ全体
- other namespace（`type:`/`artist:`/`group:`/`series:`/`character:`）→ area = namespace・tag = 右辺
- `language:` → lang に反映
- 旧実装は area を常に `tag` にしていたため `type:doujinshi` が 404（`/n/tag/type:doujinshi-japanese.nozomi`）

`ExtractGalleryID`（hitomi の `GetGalleryId` と同じ `<digits>.html` 規則）を追加し、`gallery`/`download`/`read`/`fav` で `--url` を受けられるようにした（`targetID()`）。

### Verification（2026-11-01）

- `go vet ./...` clean。`go test ./...` → ok（13 tests・2.4s。`TestSearchNozomiDecodeAndRange` は area 分区と raw colon を期待するよう更新）
- live 確認（追加 5 呼出し・間隔 1000 ms）:
  - `search --term "female:yuri" --lang english --page 1` → raw colon（`/n/tag/female:yuri-english.nozomi`）で 25 件
  - `search --term "type:doujinshi" --lang japanese --page 1` → `/n/type/doujinshi-japanese.nozomi`（200・1192956 bytes）で 25 件
  - `gallery --url "https://hitomi.la/doujinshi/ケモ姉さん10-日本語-4252697.html"` → id 4252697 を抽出、title `Kemo Nee-san 10`（`ケモ姉さん10`）・203 files・artists `kikunyi`・groups `kimuchi-tei`
  - 同一 URL で `read --page 1` → `last_page=1`、`fav` → `favorite=1`、`list --list favorites` に表示
- namespace なし multi-word / タイトル全文検索は `nozomiurlindex` の B-tree 降下が必要で未実装 → `docs/ai/known-issues/2026-11-01-plain-title-search-needs-index.md`
- `使い方メモ.md` に `--url` と分区規則、全文検索の制限を追加。`hub.exe` 再ビルド（10.9 MB）

## checkpoint 7（2026-11-01）: 実装済み層の動作確認と 2 件の修正

### 修正

1. サブコマンド位置（`app/main.go` の `findCommand`）
   - 旧: 最初の非フラグ語をサブコマンドと解釈 → `hub.exe --root <path> --no-vt` で `<path>` をコマンドと誤読し、`--root` の値が `--no-vt` になって DB path が `app\--no-vt\hitomi.db` で失敗（sqlite error 14）
   - `valueFlags` の値を消費するフラグを踏んで走査する方式に変更。サブコマンドは前後どちらに置いても可。テスト `TestFindCommandKeepsFlagValues`（4 ケース）
2. ダウンロードの URL ext（`app/hitomi.go` の `StorageURL`）
   - shard には webp / avif ののみ格納。実測: hash `2d18a9a8…42b89` は `…/2488/<hash>.webp` → 200（210784 bytes）、同じ hash の `.jpg` → 404
   - `galleryinfo.files[].name` の ext（例 `001.jpg`）は元の形式で、shard の ext と違う。旧実装は name の ext を URL に使って全件 404（203 件）
   - URL は保存形式（既定 webp、config `hitomi.image_format`）で組み、保存名は `<hash>.<format>`
   - `--limit` は成功数ではなく試行数を上限化（失敗時に上限を無視して全件試行していた）

### Verification（2026-11-01）

- `go vet ./...` clean、`go test -count=1 ./...` → ok（15 tests・2.4s。追加: `TestFindCommandKeepsFlagValues`、`TestStorageURLUsesFormatNotNameExt`）
- `python -m pytest tests/ -q` → 13 passed（0.39s）
- live（`scratch/live`・呼出し 6 件・間隔 1000 ms）:
  - `search --term "artist:kikunyi" --lang japanese --page 1` → 25 件（4252697 を含む）
  - `gallery --url "...-4252697.html"` → title `Kemo Nee-san 10`・203 files・artists `kikunyi`・groups `kimuchi-tei`
  - `download --id 4294614 --limit 1` → 1 件保存（attempted 1）
  - `download --url "...4252697.html" --limit 1` → 1 件保存（attempted 1・修正前は 203 件試行で全 404）
  - scan summary（`--no-vt --verify`）: not_evaluated 3（画像は引擎なし。clean を約束しない）。files 行に sha256・magic `webp`・path を記録
  - read/fav（`--url`）: `last_page=7`、`favorite=1`、`list --list favorites` に 3 行

## checkpoint 8（2026-11-01）: 閲覧（ビューア）実装

`app/viewer.go` + `app/ui.html`（embed）+ `view` サブコマンド。設計は `decisions/2026-11-01-viewer-single-page-web.md`。

- native GUI 不採用（CGO・webview 依存が完成フォルダの前提を壊す）。1 ページのローカル Web を `127.0.0.1:8731`（config `viewer.port`/`viewer.bind`）
- local-first 表示: `Galleries/<id>/<hash>.<format>` があれば `/files/...`、無ければ `StorageURL`
- ページ表示で `POST /api/read` → `read_state` 更新（ページ到達基準）。`POST /api/fav`、`POST /api/query`
- `GET /api/lists`（unread/favorites/queries）、`GET /api/search`、`GET /api/gallery/<id>`
- path guard: `safeGalleryPath` が `filepath.Clean("/"+rel)` で gallery dir に固定（`..`・絶対路径は dir 内の名前に解消され外へ出られない）
- SQLite 単一接続（`SetMaxOpenConns(1)`）で handler 同時書込みの SQLITE_BUSY を回避

### Verification（2026-11-01）

- `go vet ./...` clean、`go test -count=1 ./...` → ok（19 tests・2.6s。viewer 4 件: path guard、local-first・CDN fallback、read/fav/query handler、embed index）
- live（`scratch/live`・exe 単体・hitomi 呼出し 0。guard 確認のみ）:
  - `GET /` → 200（外部 asset なし。favicon 404 のみ）
  - `GET /api/lists` → unread 2・favorites 3・queries 1
  - `GET /api/gallery/4252697/page/1` → `local:true`、`src:/files/4252697/899dc188…webp`
  - `GET /files/4252697/899dc188…webp` → 200（357106 bytes）
  - `GET /files/4252697/../../hitomi.db` → 404（gallery dir 内に解消。外へ出ない）
  - `POST /api/read` → `last_page=9`→`1`、`POST /api/fav` → `favorite=0`（DB 更新を確認）
  - browser（playwright）で `#/g/4252697` を開き title/tags・`203 files・local 1（format webp）`・local-first の src を表示。初期 hash は load 時に読む（`ui.html` の load 時 open を追加して再 build 後に確認。favicon 404 のみで console error なし）

checkpoint B commit: `12f71fc`（push 済み）。使い方メモの正典は `docs/ai/usage-memo.md`、完成フォルダ側はコピー。

## checkpoint 9（2026-11-01）: YARA の役割分担と main ループの gate

- `rules/structural.yar` に YARA 固有規則を追加: `Zip_Reference_Entry`（`.url`/`.ini`/`.desktop`）、`PDF_OpenAction_Launch`、`PDF_AA_Script`、`PDF_XFA_Embedded`
- gate を status 依存から変更: Python `yara_gate(ext, cfg)`、Go `YaraGate(ext, cfg)`。main ループは scan target すべてで YARA を走らせる
- 決定: `docs/ai/decisions/2026-11-01-yara-role-division-and-gate.md`。known issue は解決して `known-issues/archive/` へ

### Verification（2026-11-01）

- `python -m pytest tests/ -q`（hitomi-hub）→ 16 passed（gate・固有規則・hit 済みファイルへの `yara` 記録）
- `go vet ./...` clean、`go test -count=1 ./...` → ok（`TestYaraGateRunsForAllScanTargets`、`TestReferenceEntryIsYaraOnlyFinding`）
- 端到端 `docs/ai/tmp/yaracheck.py`（fixture cbz）: `yara | Zip_Reference_Entry` + `yara | Zip_Path_Traversal` + `yara | Zip_Executable_Entry` + structural findings
- Common_Operations 側 `python -m pytest tests/ -q` → 109 passed、`pwsh -NoProfile -File tests\smoke_safe_rmdir.ps1`・`smoke_recycle_external.ps1` は正典ツール変更なし（該当なし）

## checkpoint 10（2026-11-01）: gg.js の実行時読取（snapshot は fallback）

- 正典生成スクリプト `tools/gen_ggset.py`（live gg.js → `app/ggset.go`・`--dry-run`）
- `app/gg.go` に `GG` / `parseGG` / `Client.LoadGG`（1 実行 1 回・失敗時 snapshot、stderr に `gg: snapshot fallback`）。`ImageURL`・`StorageURL`・`ThumbnailURL` は `GG` を受け取る
- CLI 各ネットワークコマンド（search/gallery/download/check/view）で `loadGG()`。config `hitomi.use_live_gg`（既定 true）
- tests は rotating な実値をハードコードしない（fixture で規則を検証）。snapshot は非空のみ検証
- 決定: `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。known issue は解決して `known-issues/archive/` へ

### Verification（2026-11-01）

- `go vet ./...` clean、`go test -count=1 ./...` → ok、`python -m pytest tests/ -q` → 16 passed
- exe 再ビルド（`-trimpath -ldflags="-s -w"`）→ `output/hitomi-hub/hub.exe`（12.5 MB。`app/scratch/dist/hub.exe` のRelative 出力先 mistake に注意: 正典 exe は `output/hitomi-hub/hub.exe`）
- live（exe 単体・`view`・呼出し 3 件・間隔 ≥1 s）:
  - `GET /api/thumb/2d18a9a8…42b89` → `200 image/webp` 55320 bytes（TN は bucket を使わない）
  - `GET /api/image/2d18a9a8…42b89` → 404。URL は live gg.js の bucket（`1793494802/`）と `m`（`w2`）で組まれていた → 実行時読取は動作しているが、その時点で data が窓に無い
  - `docs/ai/tmp/gg_probe2.py`（同一 hash の 4 組合せ）→ すべて 404。9:35 に 200 だった `w1`+`1793491201` は 10:05 に 404（bucket は 1 時間刻みで前進）

## checkpoint 11（2026-11-01）: 画面での動作確認（viewer・playwright）

exe は `output/hitomi-hub/hub.exe`（`--data-dir` = リポジトリルート、`--root` = `scratch/live`）。live gg.js を読んで起動（stderr に fallback 警告なし）。

- UI 表示: 検索（`furry` → 32834 件・page 1）、保存検索 2 件、既読/お気に入り一覧、reader（title・tags・files・local/cdn マーカー、前/次/読み完了/お気に入り、page strip）
- local-first: `/files/4252697/899dc188…webp` → `200`、reader の main image 表示（`naturalWidth > 0`）
- 画面操作 → DB 反映: reader open で `POST /api/read`（`last_page=1`）、`お気に入り` クリックで `POST /api/fav` → `favorite=1`。CLI `list --list favorites` / `unread` で確認
- 画像取得: 検索結果（現行窓）の TN は 200、古い gallery（4252697）の page 1 hash は 19/20 が upstream 404（proxy は 502）。TN が 200 の hash の shard も 404（`w2/1793494802/3190/<hash>.webp`）。同一 hash の `a`/`b` は同一 content、`c` は DNS 解決失敗
- 記録: `docs/ai/known-issues/2026-11-01-viewer-cdn-time-window-404.md`（実装変更なし）。スクリーンショット `scratch/hitomi-usage/screen_4252697_page1.png`

## checkpoint 12（2026-11-01）: viewer thumbnail の仕様変更（cover だけ）

- 検索 result card は cover thumbnail だけ。`searchResult()` は `pages` を返さず、`thumbnail` に cover proxy path（`gallery=<id>`）を持つ。UI results card の mini strip を削除。
- reader strip は page image を表示する必要があるため、`/api/thumb/` に `gallery` query を追加し local-first で `Galleries/<id>/<hash>.<format>` を読む。TN が無い hash は full image proxy へ fallback。image generation/cache は追加しない。
- reader strip の UI は cover/local entry だけを描画。`/api/gallery/<id>` の `files[].local` を基に、非 local の page thumbnail を broken image にしない。`searchResult()` は cover proxy path だけ返す。
- page model は変更しない（viewer の page は file index。hitomi の page model との差は既知）。
- テスト: `TestSearchResultUsesCoverOnlyThumbnail`、`TestViewerGalleryFilesIncludeLocalFlags`、`TestViewerThumbProxyLocalFirstWhenGalleryQuery`、`TestViewerThumbProxyFallsBackToFullImageWhenTNMissing`、`TestEmbeddedUISearchCardUsesCoverOnlyThumbnail`。

### Verification（2026-11-01）

- `go vet ./...` → clean
- `go test -count=1 ./...` → ok
- `python -m pytest tests/ -q` → 16 passed
- completion exe rebuild → `output/hitomi-hub/hub.exe`（相対 `app/scratch/dist/hub.exe` mistake を避ける）

### Screen verification（2026-11-01）

- search results: cover only. First card cover proxy `97cb6a05…?format=webp&gallery=4295193` displayed naturalWidth `460`。
- remote `4295193` local 0: reader strip only cover。Non-local page thumbnails are not rendered, so no broken alt-only images。
- local `4252697` local 1: main image local `/files/4252697/899dc188…webp` displayed naturalWidth `2880`。reader strip local entry displayed。
- remote full image proxy for `97cb6a05…` still upstream `502` in current window (known CDN window issue).
- Screenshot: `scratch/hitomi-usage/screen_cover_only_strip_local.png`
- known issue は解決して `docs/ai/known-issues/archive/2026-11-01-mini-strip-thumbs-404.md` へ移動

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
