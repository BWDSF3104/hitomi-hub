# R-08: search card が files 全件 + 件あたり最大 2 回の os.Stat で重い

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 A: 1 gallery 1 回の集合判定、保存済み枚数と代表画像の材料は残す）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

- 外部指摘（Claude）の読み取り検証記録は `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` の R-08 欄。本記録は現行コードの照合と実測による有効性確定のみ（コード変更なし）。
- 案 A 承認済み（`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）: ファイルチェックを 1 gallery 1 回へ減らし、表示に必要な情報（保存済み枚数、代表画像の材料）は残す。

## 調査結果 / 変更内容

### 現在の実装（機構確定）

- `/api/search` は `ids` だけ返す（`app/viewer.go:570-576`）。card は `/api/search-card/{id}` ごとに `searchResult`（`app/viewer.go:604-624`）で構築。card 1 件につき:
  - `entryLocalFormats` が file ごとに最大 2 候補（baseFormat・entryFormat）で `fileLocal`（= `os.Stat`）を呼ぶ（`app/viewer.go:641-643`、`705-717`、`912-919`）
  - `localCount` が `os.ReadDir` 1 回（`app/viewer.go:1116-1129`）
- card 1 件の stat 回数: 全件 webp で 200、全件 avif（hasavif）で 400（1 gallery 200 page）。指摘の計算是有効。

### 有効性: 確定（実測で閉じた）

スクラッチ `docs/ai/tmp/assess-20261106-1056/r08/measure.go`（25 gallery × 200 page、half avif・mock 実ファイル・NTFS 実測）:

- 現状: `os.Stat` 7500 回 + `os.ReadDir` 25 回 → 94.3 ms
- 案 A（1 gallery 1 ReadDir + 集合 membership）: syscall 0（map lookup 7500）+ `os.ReadDir` 25 回 → 6.0 ms（約 1/16）
- local 枚数は両経路一致（5000 = 5000）→ 集合判定で意味は変わらない
- 応答サイズ（card 200 files）: 現状 37,712 B → reduced（hash+local のみ）18,312 B。25 card ページ: 942,848 B → 457,848 B（約半分のサイズ削減は fields を削る場合のみ。1 ReadDir 化だけではサイズは減らない）

「files は死荷重」は誤り（指摘どおり・照合済み）: UI が `files` を使用 - `local N/M` 分子（`app/search.html:408`・per-entry `files[].local`）と cover fallback の first hash（`app/search.html:413-415`）。

### guard test 照合（存在確認済み・矛盾なし）

- `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen` は `app/viewer_test.go:2203-2238` に存在。読み取り記録（findings）の範囲 `2203-2224` は実際の end（2238）より短いが、test 存在と pin 内容（`files`・`scene_indexes` の存在 `2214`、`local_formats` 非 nil・`hasavif` `2228`、`local == false` `2231`）は一致。
- reduced 形（hash+local のみ）へ変更するには test 更新が必要。test を変えない形（field を保持し syscall 経路だけ変更）でも 1 ReadDir 化の速度効果は得られる。

### 実装（2026-11-07・案 A 採用・試行 1 回）

- 採用: A1（最小・response shape 維持・guard test 変更不要）。承認案 A の「1 gallery 1 回の集合判定」を `searchResult` 限定で実装。根拠: 正典 guard test が `files`・`scene_indexes`・`local_formats`・`hasavif` を pin しており、reduced 形（A2）へは test 更新（正典 pin の変更）が必要。委譲指示も response shape 維持時は guard test 変更不要。よって syscall 経路のみ変更し、表示に必要な材料（per-entry `local`/`local_formats`・first hash）はそのまま保持。サイズ削減（A2 限定）は未実施（正典 pin 変更なしで達成不能）。
- 変更（`app/viewer.go` のみ）:
  - `searchResult`: `galleryLocalSet(id)`（1 回の `os.ReadDir` で file 名集合と non-dir 件数）+ `entryLocalFormatsFromSet`（membership）へ置換。top-level `local` は同一 ReadDir の件数（`localCount` 呼出し削除）。card あたり os.Stat 0 回・ReadDir 1 回。
  - helper 追加: `entryLocalFormatsFromSet`・`(v *viewer) galleryLocalSet`。既存 `entryLocalFormats`/`fileLocal`/`localCount` は `/api/gallery`・`pageSource` 用として未変更（波及なし・searchResult 限定）。UI 未変更（分子データ源 per-entry `files[].local`・search.html 408 行不変）。guard test（`app/viewer_test.go:2203-2238`）未変更。
  - membership は `hash.format` の集合 lookup。既存 `fileLocal` の `safeGalleryPath` 判定は site data 由来 hash のため集合 membership と結果一致（実測 card 296403 で一致確認）。
- 作業前チェック（viewer 触る項目）: 正典 decision 3 件読了 — `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。TN 正典（cover 専用・reader 表示源禁止・fallback 禁止）は本変更で無影響（code 触れず・screen で cover src TN URL 確認）。

### UI 表示（served 画面で確定）

- 分子のデータ源は per-entry `files[].local`（`app/search.html:408`）で、server トップレベル `local`（ReadDir count）とは意味が違う: mock（hash 3 件 + 1 hash の重複 format）で server `local`=4、UI 表示分子=3。表示は 3/8 → per-entry flag が分子。案 A で per-entry flag を消すなら UI 変更が必要（UI は現状維持が安全）。
- cover: first hash の TN URL（`data-thumbnail`・src 実測）で表示。TN display fallback なし（正典条件維持）。
- `app/viewer.html` の `entry.local_formats` 使用箇所（296、795、828、887、1064）は `galleryCache`（= `/api/gallery` の data・`app/viewer.html:1185-1187`）から供給され、search card data は supply 経路なし → search card の field を削っても viewer 表示・R-15 の memory 削除優先（DB `cache_entries` へ決定済み、card data 参照は supersede）への影響なし。

### 対応の実現可能性・推奨修正方法

1 ReadDir で gallery の file 集合を作り、per-entry `local`/`local_formats` を membership 判定へ置換（syscall 0）。推荐:
- A1（最小・test 変更なし）: `searchResult` の file ループを集合判定へ変更。response shape は現状のまま（guard test green）。速度効果のみ。
- A2（承認どおりの「表示に必要な情報だけ」）: entry を hash+format+local（+ test が pin する hasavif/local_formats を membership から埋めるか、test を reduced 形へ更新）。サイズ削減（上記実測）は A2 の場合のみ。
- 分子表示は per-entry flag を維持（UI 408 行を変えない）。top-level `local` は raw file 件数で分子意味と違う（実測 4 vs 3）ため、UI を top-level へ切り替える案は分子意味の変更（拒否推奨）。

### 副作用・影響範囲

- `entryLocalFormats` は `gallery` endpoint（`app/viewer.go:688-696`）でも同じ helper を共有。search card だけ変えるなら `searchResult` 限定、helper 共通変換なら `/api/gallery` にも 1 ReadDir 化が波及（viewer の local 判定・R-15 の memory 優先 status にも effect 無し・viewer.html は card data 非使用）。
- card cache（`searchCardCache`）は廃止済みで影響なし（`app/viewer_test.go:2157-2162` の guard 確認済み）。
- 同一 gallery の card 再取得（ページ送り・再表示）は要求ごとに 1 ReadDir が再実行。card cache なし（正典 guard）なので案 A の「1 gallery 1 回」は per-request の話。
- full image 順序・TN 表示源・`x-cache`・generation guard は本項目の範囲外（不変条件へ影響なし）。
- 代替案: per-card ReadDir を infoCache 相当の集合 cache へ載せる案は card cache guard（2157-2162）と衝突するため不採用。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app）→ ok。経過 30.1 s（test 26.272 s）。guard test `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen`（`app/viewer_test.go:2203`）含め green。
- `python -m pytest tests/ -v` → 54 passed・1 failed・経過 11 s。failure は `test_canonical_build_freshness_rule_exists`（AGENTS.md の pointer 欠落・pre-existing・本項目と無関係。AGENTS.md・既存テスト未変更。exe 埋め込み freshness test は合格）。
- 実測（スクラッチ mock・NTFS）: 現状 94.3 ms（7500 stat + 25 ReadDir）→ 案 A 6.0 ms（0 stat + 25 ReadDir）。サイズ card 37,712 B → 18,312 B。script: `docs/ai/tmp/assess-20261106-1056/r08/measure.go`
- 画面確認（served・正典 exe・1280x900・window size 記録済み・isolated origin 8760・warm として記録・cold claims なし・既存テスト変更なし・実 DB・完成フォルダ未変更）。before → after（操作 = term 入力+検索）で card 296403 の `local 3/8` 表示（分子 3）と server `local` 4 の差異を確認。cover src = first hash の TN URL。
  - 保存先: `I:\マイドライブ\kilo\2026-11-06-r08-search-card-light-response\`（`r08-search-card-before-20261106.png`、`r08-search-card-after-20261106.png`、`r08-search-card-confirmation-2026-11-06.txt`）。local copy（Playwright output・project scratch）は正典 `kilo-safe-remove` でゴミ箱へ送った。

### 実装後の検証（2026-11-07・試行 1 回）

- app: `go vet . && go test -count=1 -timeout 300s .` → ok（test 25.807 s）。guard test `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen`（`app/viewer_test.go:2203`）含め green（個別実行 4 test・1.466 s: guard・`TestSearchCardViewedAndSavedStates`・`TestViewerGalleryFilesIncludeLocalAndHitomiFlags`・`TestSearchResultUsesCoverOnlyThumbnail`）。
- `python -m pytest tests/ -v` → 56 passed・1 failed・12.27 s。failure は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 欠落・pre-existing・本項目と無関係。AGENTS.md・既存テスト未変更。freshness の exe 埋め込み test は合格）。
- 実機（正典 build・fresh instance・8731）:
  - CLI download（正典 exe・scratch root `Common_Operations\scratch\p3_canonical_fresh`・実在 gallery 115428・-limit 1）: attempted 1 = written 1（skipped 0）。
  - `/api/image/78a2c688…?format=webp`: 200・`x-cache: cache`・48,144 B（no_full なし）。
- 画面確認（served・fresh canonical exe 起動 server・window size outer 1678x1025・inner 1665x932・dpr 2・人工 resize なし・isolated origin（localStorage clear）・warm として記録（cold claims なし））。before（term `furry` 表示）→ after（操作 = term `Sailor Moon - Secret Lingerin Scent` 入力+検索）で card 296403 を確認:
  - UI 分子: 表示 `local 8/8`。API 実測 per-entry `files[].local` true = 8 件（分子のデータ源は per-entry flag・UI 意味不変）。server top-level `local` = 8 = disk non-dir 件数（`Galleries/296403` の .webp 8 件・集合判定 1 ReadDir）。
  - cover src = `/api/thumb/78a2c688…?format=webp&gallery=296403&variant=bigtn`（first hash の TN URL・TN display fallback なし）。
  - `/api/search-card/296403`: 200・`x-cache: miss`（fresh instance 初回）・response shape 維持（files・scene_indexes・local_formats・hasavif 存在）。
  - 保存先: `I:\マイドライブ\kilo\2026-11-06-r08-search-card-light-response\`（`r08-implementation-before-20261107.png`、`r08-implementation-after-20261107.png`、`r08-implementation-confirmation-2026-11-07.txt`）。local copy（project scratch）は正典 `kilo-safe-remove` でゴミ箱へ送った。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
