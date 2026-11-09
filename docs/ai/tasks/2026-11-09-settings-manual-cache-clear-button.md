# 設定画面の手动キャッシュクリアボタン（設計検討）

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: 設計承認済み（decisions/2026-11-09-manual-cache-clear-button-approved-design.md）。実装計画・テスト計画確定。実装開始指示待ち

## 意図 / 背景

ユーザー要求（2026-11-09 03:18 UTC）: 設定画面に手动キャッシュクリアボタンを追加する。対象は「リスト化前」を含む、つまり `cache_entries` 表に行がない cache file（表生成以前の実 cache・backfill 禁止分）まで消す。本タスクは設計検討のみ。実装開始指示はまだ無い。

## 調査結果 / 変更内容

### cache 層の現況（実測ではなくコード読読による確認）

- disk full cache: `cacheRoot` = root + `cache_dir`（既定 `cache/images`）。file 名 `<hash>.<format>`（`app/main.go:79`、`app/viewer.go:129`、`app/viewer.go:365-374`）
- TN disk cache: `<cacheRoot>/thumbs/<hash>.<format>.<variant>`（`app/viewer.go:376-386`）
- atomic write 中の temp: 同一 dir 内 `.cache-*.tmp`（`app/viewer.go:489`）
- galleryinfo disk 層: root + `cache_dir` + `galleryinfo`（既定 `cache/galleryinfo/<id>.json`）。`InfoCacheDirName()` は同一 config key `cache_dir` を既定 `cache` で使う（`app/main.go:80`、`app/main.go:87`、`app/viewer.go:133`）
- DB 記録表 `cache_entries`（schemaVersion 9）。件数・作成順・status は DB 行だけで判定し filesystem scan で数えない。表生成以前の実 cache file は backfill しない（`app/main.go:380-384`、`app/main.go:558-572`、decision `2026-11-08-r15-cache-entries-table-supersede.md`）
- server memory caches: `infoCache`（`app/viewer.go:34-38`）、client `noFullCache` TTL 5 分（`app/hitomi.go:22`、`:242-244`）、searchindex caches（`app/searchindex.go:240`、`:282`、`:315`）
- browser page objectURL cache（document ごと、cap `viewer.max_object_urls`）
- `scan_cache`（VT verdict cache・TTL 30 日）は `hitomi.db` 内。LRU 適用範囲条は `hitomi.db` を削除対象にしない（decision `2026-11-06-review-findings-approval-decisions.md:49`）
- 完成フォルダ `Galleries`・`acquisition` はアプリ側削除禁止（decision `2026-11-09-completion-folders-no-delete-rule.md`）
- 現況の画面に削除操作はない（decision `2026-11-06-review-findings-approval-decisions.md:48`）。ボタン追加はこの条の supersede 承認案件。

### 設計案（ユーザー承認 2026-11-09 03:23 UTC・正典 `docs/ai/decisions/2026-11-09-manual-cache-clear-button-approved-design.md`）

1. route: `POST /api/cache/clear`（既存 `withPOSTSecurity` の loopback + `application/json` 条件を流用）。settings form の submit へ混入させず、専用 section に独立 button を置く。
2. body: `{"scope": ["full","thumbs","galleryinfo","memory"], "confirm": true}`。`confirm` なしは拒否（2段階確認）。
3. 実行順（`cacheEntriesMu` を保持）:
   - DB 行から path を組んで file 削除 → 行 DELETE（削除失敗の行は保持。prune と同じ流儀 `app/viewer.go:1155-1165`）
   - `cacheRoot` 内の file walk で DB 行のない file（リスト化前）を削除。これは件数判定の filesystem scan ではなく削除列挙として扱う（条の例外承認を得た）
   - memory cache map（`infoCache`・`noFullCache`・search caches・index caches）を空にする
4. guard: `cacheRoot` が `galleryRoot`・`acquisitionRoot`・root 外へ解決する場合は拒否。singleflight で write 中の path は skip して報告。
5. response: 層別削除件数（full / thumbs / galleryinfo / unlisted / memory entries）、rows_deleted、skipped_inflight、failed。UI は新 `#cache-clear-status`（`aria-live`）へ件数を出し、実行中は button を disabled。

### 論点（承認結果）

- 論点1「リスト化前」の範囲: 行なし file を消す。件数判定の scan 条は無変更、削除列挙の例外として承認。
- 論点2 削除層の範囲: full・thumbs・`cache/galleryinfo`・server memory cache 層を含む。`scan_cache`（VT 判定）は `hitomi.db` 条により除外。
- 論点3 画面の削除操作: 「削除操作は画面にない」条の cache 層限定 supersede を承認。1回 click で削除は非推奨。
- 論点4 報告の形: UI preview は DB 行数のみ、walk preview は置かない。確認は2段階（DB 行数表示 → confirm check → 実行）、削除後に層別件数を報告。

影響（挙動対比）: クリア後の再表示は remote 再取得（gate 消費・R-07）になる。`x-cache` 意味、TN 表示源禁止、full 判定、永続先、完成フォルダ不削除条は無変更。UI は `cache: "no-store"` fetch を使うため browser cache 由来の表示残りは表示に影響しない。

### 実装計画（承認済み設計に基づく）

**1. route（`app/viewer.go` `handler()`）**

- `mux.HandleFunc("POST /api/cache/clear", v.clearCache)` を settings route の直後へ追加。
- POST 条件（loopback origin + `application/json`）は既存 `withPOSTSecurity` を流用（`app/viewer.go:205-222`）。

**2. handler `clearCache`（`app/viewer.go`）**

- body `{"confirm": true, "scope": ["full","thumbs","galleryinfo","memory"]}`。
- `confirm` が false / 欠け → 400。未知 scope → 400。scope 空 → 400。既定は4層全部。
- allowlist（config key）は増やさない。`isAllowedSettingsField` は無変更。
- guard: `v.cacheRoot` と `v.infoCacheRoot` が `v.galleryRoot`・`v.acquisitionRoot` と一致、または `withinRoot(v.root, …)` 外へ解決する場合は拒否（完成フォルダ不削除条）。
- 実行は `v.cacheEntriesMu` を保持（prune と同一 mutex で直列化）。
  1. `cache_entries` 行から `cacheEntryPath` で path を組み file 削除 → 成功行のみ DELETE。失敗行は保持（prune 流儀 `app/viewer.go:1155-1166`）。
  2. 削除列挙（リスト化前）: `filepath.WalkDir(v.cacheRoot)` で行のない file を削除。名前は cache 命名に一致するものだけ（full `<64hex>.<format>`、thumbs `thumbs/<64hex>.<format>.<variant>`）。`.cache-*.tmp` は skip。
  3. galleryinfo disk 層（`v.infoCacheRoot` 内の `<gallery_id>.json`）。
  4. memory 層: `v.infoCache` を空・`v.infoCacheBytes = 0`。client 側 `ClearMemoryCaches()` で `noFullCache`・`searchCache`・`searchResultCache`・`indexTopCache`・`indexNodeCache`・`indexDataCache` を mutex 単位で空に。`singleflight` group・`versions`・`gg`・gate state・`blockCooldownUntil`・`metrics` は消さない。
- in-flight skip: `v.cacheWriteGroup` に key が在る path と write 中の temp は skip して件数へ。
- bounded 列挙（C-05/A-14 流儀）: walk 件数は `maxCacheDiskMaxEntries = 20000` を上限とし、超過分は削除せず `truncated` として報告（无限制 walk 禁止）。
- test seam: `cacheRemoveHook func(path string) error`（viewer field。nil は非適用。A-07a seam 流儀）。既存 `atomicWriteSeam` の counter・hook は無変更。
- response（`jsonOut`）: `{"deleted": {"full": n, "thumbs": n, "galleryinfo": n, "unlisted_full": n, "unlisted_thumbs": n, "memory": n}, "rows_deleted": n, "skipped_inflight": n, "failed": n}`。

**3. settings view の read-only counts**

- `settingsView` へ `cache_counts`（`cache_entries` の kind 別 DB 行数）を追加。walk preview は UI に置かない。
- 実装時の確定（計画からの逸脱点・記録）: `cache_counts` は config 値ではないので `settingsBaseMap`（config write base）へ置かない。テストで固定。
- memory clear 後の `x-cache` 確認は `/api/search-card/{id}` を使う。`/api/gallery/{id}` は `x-cache` を置かない（`x-cache` の源は searchCard 側の memory membership）。

**4. UI（`app/settings.html`）**

- `#settings-form` の外に `<section id="cache-clear-section">`（`fieldset`/`legend` で区切る既存 group 流儀）。
- read-only 行: DB 行数 by kind（`cache_counts`）。
- `#cache-clear-confirm` checkbox（label は「削除することを理解しました」）。check 前は button disabled。
- `#cache-clear-button`（type=button、label「キャッシュをクリア」）。実行中は `button.disabled = true`。
- `#cache-clear-status`（`class="status" role="status" aria-live="polite"`）。既存 `#settings-status` は設定保存専用として維持。
- `collectPayload()` へ混入させない（`input[data-key]` を使わない）。`fetch("/api/cache/clear", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({confirm: true}) })`。
- viewer page は無変更（settings panel・cap 通知 UI 禁止条維持）。

**5. 非触（不変条件）**

- `imageProxy` 順序、TN 表示源・fallback、`x-cache` 意味、atomic write・temp・singleflight、gate、gg、永続先、完成フォルダ、`files` 表、`scan_cache`。

### テスト計画

**Go 固定テスト（`app/viewer_test.go`・`newTestViewer` / `jsonPOSTRequest` / `cacheEntryRow` / `uiText` 流儀）**

1. `TestCacheClearRequiresConfirmAndRejectsBadRequest` — `confirm` 欠け / false → 400。未知 scope → 400。scope 空 → 400。content-type 不正 → 415。cross-origin POST → 403。
2. `TestCacheClearDeletesDBRowsAndUnlistedFiles` — DB 行2件＋行なし file（表生成前相当、`untracked*.webp`）2件 → 両方削除。`rows_deleted` と `unlisted_full` 件数を response で確認。
3. `TestCacheClearKeepsRowsWhenFileDeleteFails` — `cacheRemoveHook` で1件失敗注入 → file 削除失敗・行保持・`failed = 1`。
4. `TestCacheClearSkipsInflightWritePaths` — `cacheWriteGroup` に key が在る path は skip（`skipped_inflight = 1`）、行と file 保持。
5. `TestCacheClearRefusesCompletionFolderAndOutsideRoot` — `cache_dir` が `Galleries`（または `acquisition`）へ解決 → 拒否、file 削除なし。
6. `TestCacheClearDoesNotTouchScanCacheOrFilesTable` — `scan_cache` 行と `files` 行、完成フォルダ内 file を fixture 置き、clear 後は無変更。`DELETE FROM` は `cache_entries` のみ（decision `2026-11-09-completion-folders-no-delete-rule.md` の実測条を固定）。
7. `TestCacheClearOnlyRemovesCacheNamedFiles` — cache 命名に一致しない file（例 `notes.txt`）と in-flight temp `.cache-*.tmp` は削除しない。
8. `TestCacheClearResetsMemoryCachesWithoutChangingCacheHeaders` — warm `infoCache`・`noFullCache`・index caches → clear 後空。clear 後の `/api/gallery/<id>` は `x-cache: miss`、`/api/image` の `x-cache` が `miss`（`no_full`・TN 意味は無変更）。
9. `TestSettingsPageRendersJapaneseLabelsAndGroupCards` へ anchor 追加: `id="cache-clear-section"`, `id="cache-clear-confirm"`, `id="cache-clear-button"`, `id="cache-clear-status"`, `aria-live="polite"`, `button.disabled = true`, `fetch("/api/cache/clear"`。既存の settings panel 禁止条件（viewer.html）は維持。
10. 既存 `TestCacheEntriesPruneCountsDBRowsNotFilesystemScan` は無変更（prune の意味を変えない。clear は別 route の操作）。

**Python**: `python -m pytest tests/ -v`（`tests/test_canonical_build_freshness.py` の `UI_PATHS` に `app/settings.html` は既在。新 anchor は Go 側で固定）。

**実機確認（`docs/ai/rules/confirmation-test-rule.md`）**

- freshly rebuilt canonical `hub.exe` → served `/settings`。cold hygiene（fresh server instance、isolated tab、storage clear）。
- before / after screenshot（同一 window size・同一被写体範囲・同一取得方法、file name に `before` / `after`）。
- 合格条件: before の DB 行数表示 → confirm check → button click → status に層別件数。clear 後の `/api/image/<hash>?format=webp` が `x-cache: miss`（`no_full` でない）。`Galleries`・`acquisition` の file 数は before/after 一致。DB の `cache_entries` 行数 0、`scan_cache` 行数は無変更。
- 保存: `I:\マイドライブ\kilo\2026-11-09-settings-manual-cache-clear-button\` に画像と `<confirmation-topic>-confirmation-2026-11-09.txt`。local copy は `kilo-safe-remove`。

**記録**: 実行後に `go vet . && go test -count=1 -timeout 300s .`（app）と pytest のコマンド・結果・経過時間を本タスクの Verification へ記録。実装開始指示は未受領。

## Verification

- `go vet .`（app）: 合格（出力なし）
- `go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 24.119s`
  - 初回 run 23.970s で3件不合格 → 原因分析して修正（`cacheWriteGroup` nil map・`recordTestFileSize` の FK（`recordDiscovered` 先）・memory clear 後の `x-cache` は `/api/search-card/{id}` の意味（`/api/gallery/{id}` は x-cache を置かない））→ 修正後合格
- `python -m pytest tests/ -v`: `60 passed in 13.54s`
  - canonical rebuild 前は `test_canonical_build_freshness` が stale `hub.exe` で 1 failure（settings.html 不一致）→ 正典 build command 実行後 60 passed
- canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
- 実機（served 画面・fresh instance・window size 実測 1666x932・人工 resize なし・before/after 同一条件 full page）
  - 測定1（8731・正典 measurement root `Common_Operations\scratch\p3_canonical_fresh`）
    - before: status 空・表示 `full 544 / thumbs 160`・button disabled・file `cache\images` 1399・`thumbs` 1190・`Galleries` 35
    - click（confirm check 後）→ status: `削除 full 544 / thumbs 160 / galleryinfo 0 / 記録前 full 855 / 記録前 thumbs 1030 / memory 0・行 704・skip 0・失敗 0`
    - after: 表示 `full 0 / thumbs 0`・file `cache\images` 0・`thumbs` 0・`Galleries` 35（一致）
    - 突合: 544+855=1399（full）・160+1030=1190（thumbs）・rows_deleted 704 = 544+160。合格。
  - 測定2（fresh root `scratch\clear_confirm2`・port 8743・fresh instance）
    - app 自身の galleryinfo 経路を `GET /api/gallery/4304794` で生成（`cache\images\galleryinfo\4304794.json`）。cache file + DB 行は `GET /api/image/bb2681ae…?format=webp` → 200 `x-cache: miss`。記録前 file は `cache\images\thumbs\bb2681ae….webp.bigtn`（行なし）
    - before: 表示 `full 1 / thumbs 0`・file images 1・thumbs 1・galleryinfo 1
    - after status: `削除 full 1 / thumbs 0 / galleryinfo 1 / 記録前 full 0 / 記録前 thumbs 1 / memory 1・行 1・skip 0・失敗 0`。file 0・0・0。`cache_counts` full 0 / thumbs 0。合格。
- Drive 保存: `I:\マイドライブ\kilo\2026-11-09-settings-manual-cache-clear-button\`（before/after 4 画像と `settings-cache-clear-confirmation-2026-11-09.txt`）。local copy（project `scratch/`）は正典 `kilo-safe-remove` でゴミ箱へ送った。Drive 側は保持。
- 記録: `docs/ai/decisions/2026-11-09-manual-cache-clear-button-approved-design.md`（設計承認正典）、`docs/ai/known-issues/KI-20261109-01-galleryinfo-shape-vs-cache-dir.md`（galleryinfo 置き場の shape 差。現実装は app 自身の経路のみ削除・誤削除なし）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
