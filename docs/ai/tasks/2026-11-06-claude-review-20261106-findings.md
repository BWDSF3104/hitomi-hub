# 外部指摘（Claude）2026-11-06 読み取り検証記録

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 優先順位確定（assess-20261106-1056）。実施開始指示待ち。承認案件: R-04 歴史記録・R-05 命名・R-14 alpha/format・R-16 atomic scope・R-17 B1・R-18 page化。R-18 は対応不要で終了。R-19 実例待ち。known-issue: KI-20261106-04（pytest 既存 failure）。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

- 外部指摘（Claude）を正典記録として残す。レビュー側は実行しておらず、すべて読み取りによる指摘。
- 本記録の目的は指摘の保存と現コードに対する照合であり、実装・修正の開始ではない。コード・UI・schema は変更していない（変更したのは `docs/ai/` の記録のみ）。
- 照合基準: HEAD `e018c1c`（`docs: 確認テスト系ルールを単一ファイルへ集約`）。照合は read-only（`git status --short` は照合開始時点で clean）。
- 確度は 3 段階で分ける: **確定**（現コードから機構が確定できる）／**機構確定・影響は推定**（コードの機構は確認、影響の大きさ或未再現は実測なし）／**未検証**（実例・実機なし）。
- 既存の指摘記録と重複する項目は、新規タスクを作らず既存タスクの残存点として照合した（重複欄に記録）。

## 実行固有ID と対象項目一覧（assess-20261106-1056）

- 開始時点の Git 状態: clean（HEAD `9f867fc`）。開始時点の未コミット変更なし。
- 対象: R-01〜R-18 の 18 件。R-19〜R-21 は既知問題として記録済み・今回の対象外。
- 個別タスク記録: `docs/ai/tasks/2026-11-06-r01`〜`r18`（18 件・正典 `task_manager` で登録済み）。全記録に実行固有ID を書く。
- このセッションで確定した承認・指示（子への追加指示として扱う）:
  - R-05 案 A、R-08 案 A、R-13 案 A、R-20 案 A（承認済み）
  - R-11: 逐次維持。data lane interval を 90 ms に値確定（100 ms の点のみ supersede）。下限 guard 50 ms・image lane 60 ms・politeness 1000 ms は不変
  - R-15: 案 B（全体先読み維持）。memory 参照 4000 件・設定化。disk は `cache/images` と `cache/images/thumbs` 各 8000 件。disk 削除は作成順。memory 削除は保存済み status 優先、次に最後に使った順。現在閲覧中・表示中は捨ない。last access 記録なし、sha256 lookup なし。件数・順・status は DB の 1 表（例 `cache_entries`）で持つ
  - 大前提: ユーザーに関知させる必要はない。不便が起きることは許容しない。通知 UI は置かない

## 項目一覧

| # | 項目 | 判定 | 登録先 |
|---|---|---|---|
| R-01 | `boundedCachePut` の上書き計上 | 確定 | `docs/ai/tasks/2026-11-06-r01-bounded-cache-put-overwrite-double-count.md` |
| R-02 | Go `IsPE` の 64 バイト窓 | 確定 | `docs/ai/tasks/2026-11-06-r02-go-ispe-window-parity.md` |
| R-03 | 7z magic の最終バイト | 確定 | `docs/ai/tasks/2026-11-06-r03-7z-magic-last-byte.md` |
| R-04 | `recordFile` が `missing_at_verify` を戻さない | 確定 | `docs/ai/tasks/2026-11-06-r04-record-file-missing-flag-reset.md` |
| R-05 | acquisition が title だけで決まる | 確定（衝突は実データ依存） | `docs/ai/tasks/2026-11-06-r05-acquisition-title-collision.md` |
| R-06 | scan skip が同一内容の別 path で効かない | 確定 | `docs/ai/tasks/2026-11-06-r06-scan-cache-meta-per-path.md` |
| R-07 | HTTP 接続が毎回張り直される | 機構確定・改善幅は推定 | `docs/ai/tasks/2026-11-06-r07-http-transport-idle-conns.md` |
| R-08 | search card の応答が重い | 確定（指摘の一部は誤り） | `docs/ai/tasks/2026-11-06-r08-search-card-light-response.md` |
| R-09 | search-card の DB 書き込み重複 | 確定 | `docs/ai/tasks/2026-11-06-r09-search-card-record-dedup.md` |
| R-10 | ページ送りごとの全件 intersection 再計算 | 確定 | `docs/ai/tasks/2026-11-06-r10-page-result-cache-per-query-lang.md` |
| R-11 | 複数 term の逐次取得 | 機構確定（gate 方針の判断要） | `docs/ai/tasks/2026-11-06-r11-parallel-term-fetch-decision.md` |
| R-12 | 起動時の検索二重実行 | 確定 | `docs/ai/tasks/2026-11-06-r12-startup-double-search.md` |
| R-13 | singleflight がない箇所 | 確定（追加は設計承認が必要） | `docs/ai/tasks/2026-11-06-r13-singleflight-tn-info-nozomi.md` |
| R-14 | `rawRGB` の `img.At` と alpha | 確定（速度の実測値は未測定） | `docs/ai/tasks/2026-11-06-r14-pdf-raw-rgb-type-switch.md` |
| R-15 | viewer の全体先読み・disk cache 上限 | 機構確定（方針判断要） | `docs/ai/tasks/2026-11-06-r15-viewer-prefetch-window-and-disk-lru.md` |
| R-16 | TN・download 書き込みが非 atomic | 確定 | `docs/ai/tasks/2026-11-06-r16-atomic-write-tn-download.md` |
| R-17 | findings が内容変化で残る | 確定 | `docs/ai/tasks/2026-11-06-r17-stale-findings-on-content-change.md` |
| R-18 | 未読一覧の肥大化 | 確定 | `docs/ai/tasks/2026-11-06-r18-unread-list-limit.md` |
| R-19 | scene_indexes と page 基準 | 未検証 | `docs/ai/known-issues/KI-20261106-01-scene-page-count-basis.md` |
| R-20 | GET の Host 検査（DNS rebinding） | 機構確定・方針除外 | `docs/ai/known-issues/KI-20261106-02-get-host-check-dns-rebinding.md` |
| R-21 | `json_ms` の `bytes.Replace` | 機構確定・低優先 | `docs/ai/known-issues/KI-20261106-03-json-ms-bytes-replace-fragile.md` |

## 調査結果 / 変更内容

### R-01（確定）: 上書き時に古い entrySize を引かない

- `boundedCachePut` は `cache[key] = entry` 後、既存 entry の `entrySize()` を引かずに `*totalBytes += entry.entrySize()` する（`app/hitomi.go:92-95`）。eviction ループは eviction 時のみ減算（`app/hitomi.go:104-105`）。
- TTL 切れ entry は map から消えない（hit 時に delete しない）。TTL 超過 key の再 fetch は同一 key の再 insert で同じ helper を通る（`app/searchhtml.go:144-150`、`app/viewer.go:1106-1112`、`app/viewer.go:1086-1091`）。
- 結果: 同一 key の再 insert 回数ぶん `totalBytes` が水増しし、cap より早く eviction が走る。対象は `searchCache`・`indexDataCache`・`indexNodeCache`・`indexTopCache`・`infoCache`（helper 共通）。
- 補強根拠（既存記録）: `docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md` の実測で「TTL 超過 key の access は refetch・同一 key 再 insert のみ」を確認済み。当時は accounting のずれは記録していない。

### R-02（確定）: Go `IsPE` が窓外 PE を false にする

- `IsPE` は `head`（64 bytes）だけ見て `e_lfanew + 4 > len(head)` で false を返す（`app/scan.go:95-107`）。`ScanFile` は `readHead(path, 64)` の結果のみで `mz_without_pe_header` を判定（`app/main.go:193-201`）。
- `e_lfanew` が 0x80（窓外）の実在 PE は hit になる。Python 参照実装は D-04 で seek 修正済み（`docs/ai/tasks/2026-11-03-d04-ispe-header-window.md`、`src/scan.py` の `is_pe(path, head)`）。Go 側は未修正で非対称。

### R-03（確定）: 7z magic の最終バイト

- Go: `hex.EncodeToString(head[:6]) == "377abcaf2738"`（`app/scan.go:89-90`）。
- Python: `head[:6] == b"7z\xbc\xaf\x27\x38"`（`src/scan.py:79-80`）。
- 7z の signature は `37 41 7A BC AF 27 1C`（7z フォーマットの一般仕様）。両側とも最終バイトが `38` で、`1C` ではないため 7z は検出されない。実 fixture での再現実行はしていない（読み取り＋仕様照合）。

### R-04（確定）: `missing_at_verify` がリセットされない

- `recordFile` の upsert は `scan_status`・`scan_source`・`scan_time`・`sha256`・`magic`・`gallery_id` しか更新しない（`app/main.go:578-586`）。`missing_at_verify` を触らない。
- `verifyMissing` は `missing_at_verify = 0` の行だけを走査し、実在しない行を 1 へ立てる（`app/main.go:619-642`）。一度 1 になった行は以後走査されない。
- 帰結: `external_removal` 記録後に同 path へ再 download / 再 record しても、そのファイルの消失は以後検出されない。

### R-05（確定）: acquisition の folder が title 基準

- folder 名は `acquisitionTitleName(title, strconv.Itoa(galleryID))`。gallery id は title が空のときの fallback でしか使わない（`app/acquisition.go:27-40`、`88-90`、`162-167`）。page file は `%03d.<format>`（`app/acquisition.go:46-48`）なので、同名 title の別 gallery は同一 folder の同名 page file へ書き、互いに上書きする。
- sanitize は control 文字と `/\:*?"<>|` を `_` にするのみ（`app/acquisition.go:33-39`）。末尾の `.`・空白、Windows 予約名（CON・PRN など）、255 文字超は未処理。
- 方針接触: `docs/ai/tasks/2026-11-03-b04-acquisition-title-sanitize.md`（decision 2026-11-03）は「acquire 側命名（acquisitionTitleName）は変更しない」と記録。本項目はその決定と衝突するため、実装案ではなく方針変更の提案として登録（R-05）。

### R-06（確定）: scan skip が同一内容の別 path で効かない

- `scan_cache` の PRIMARY KEY は `sha256` で、`size`・`mtime` は 1 組（`schema.sql:31-40`）。
- `recordScanMeta` は `ON CONFLICT(sha256) DO UPDATE SET size, mtime`（`app/main.go:536-542`）。同一 sha256 の 2 path（例: Galleries と acquisition）では後登録の mtime が前登録を上書きする。
- `scanSkipStatus` は `scan_cache JOIN files ON f.sha256 = sc.sha256 WHERE f.path = ?` で size/mtime 一致を見る（`app/main.go:548-570`）。上書きされた側は不一致になり再ハッシュ。A-12 の skip 意味が同一内容の重複で消える。

### R-07（機構確定・改善幅は推定）: default Transport

- `http.Client` は `Timeout` だけ設定し Transport を指定していない（`app/main.go:1047`、`1095`、`1112`、`1226`、`app/viewer.go:86`）。Go の default Transport の `MaxIdleConnsPerHost` は 2（`DefaultMaxIdleConnsPerHost`）。
- prefetch は 8 並行（`app/viewer.html:207`）、download も worker pool（`app/main.go:712-742`）。並行数が idle 上限を超えたぶん再接続（TLS ハンドシェイク）になる。16 への引き上げで改善する見込みという部分は推定（実測なし）。

### R-08（確定・指摘の一部は誤り）: search card の重さ

- `searchResult` は `files` 全件を返す。file ごとに `entryLocalFormats` が最大 2 候補（baseFormat・entryFormat）で `fileLocal`（= `os.Stat`）を呼ぶ（`app/viewer.go:640-649`、`705-717`、`912-918`）。加えて `localCount` が `ReadDir` 1 回（`app/viewer.go:1116-1129`）。
- 200 ページ gallery の card で 200〜400 回の stat。card 25 件で 5000〜10000 回。指摘の計算是有効。
- 誤り部分: `files` は死荷重ではない。search page が `files` を使う - `local N/M` 表示（`app/search.html:408`）と cover fallback の first hash（`app/search.html:413-415`）。加えて guard test `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen`（`app/viewer_test.go:2203-2224`）が `files` と `scene_indexes` の存在を固定している。軽量応答へ分けるには設計承認とテスト変更が必要。
- `searchCardCache` の廃止（`app/viewer_test.go:2157-2162` の guard）は確認済み。死荷重ではなく「1 gallery 1 ReadDir で集合照合」への置き換えが現実的な改善。

### R-09（確定）: card ごとの commit 重複

- `/api/search` は `recordDiscoveredBatch` で 1 transaction（`app/main.go:1351-1385`、`app/viewer.go:566`）。
- `searchCard` は card ごとに `recordDiscovered` を autocommit で呼ぶ（`app/viewer.go:622`）。単一接続・WAL（`app/main.go` openDB）なので 25 card = 25 commit。A-02/C-03 の batch 化は解消済み、この重複は残存。

### R-10（確定）: ページ送りごとの intersection 再計算

- cache は term ごとの dataset だけ（`searchCache`・key は area/tag/language 基準、`app/searchhtml.go:144-150`）。
- `searchAllIDs` は每次呼び出しで `results` に copy（`copyInts`）を作り、positive / or group の intersection を map 化して再計算（`app/searchhtml.go:373-428`）。30 万件級で call ごとに copy + map。
- (query, lang) 単位の最終結果 cache 案は R-10。page 移動・prefetch・`firstStatus`（`app/firststatus.go`）で共有できる。

### R-11（機構確定・判断要）: 複数 term の逐次取得

- term 取得は逐次（`app/searchhtml.go:389-428` の loop 内で `termIDs` を順に await）。
- gate は間隔だけを直列化する（`gateLaneState`・`app/hitomi.go:155-166`）。並行発行すれば latency が重なるが、politeness gate の意味（FIFO・将来予約・grace）を変えるため設計判断を要る。既存 A-05/C-01/C-02 は card 側の並行化まで（`c77f92a`）で、term 側の並行化は未実施。

### R-12（確定）: 起動時の二重実行

- `restoreSearchState()` は `loadSearchCards(ids, searchGeneration)` を呼ぶ（`app/search.html:468`）。直後の `runSearch()` が `searchGeneration` を進める（`app/search.html:699`）。restore 側の worker は `searchGen !== searchGeneration` で停止（`app/search.html:575`、`605`）。
- 帰結: 每次 page load で restore 分の card fetch が捨てられる。cold 確認の汚染源にもなる（既存記録 `docs/ai/tasks/2026-11-02-external-code-review-findings.md` B-3 startup の同一件、実装未開始）。

### R-13（確定・追加は承認が必要）: singleflight

- dedup は full image fetch（`fetchFullImageOnce`・key = hash+format+gg、`app/viewer.go:787-823`）と cache write（`app/viewer.go:373-408`）、`EnsureGG`（`app/gg.go:118-202`）のみ。
- `thumbProxy`（`app/viewer.go:758-785`）、`v.info`（`app/viewer.go:1074-1114`）、`SearchNozomiParts`（`app/searchhtml.go:135-153`）は同一キーの並行リクエストを重複して外部へ出す。
- past 決定: A-07b の追加 dedup は設計承認が必要と記録（`docs/ai/tasks/2026-11-03-claude-review-349778d-assessment.md` 優先順位 25）。既存流儀（key 単位・mutex・network 中に mutex を保持しない）に揃える案。

### R-14（確定・速度の実測なし）: rawRGB

- `rawRGB` は pixel ごとに `img.At(x, y).RGBA()`（`app/pdf.go:42-61`）。`*image.NRGBA` / `*image.YCbCr` の型 switch または `draw.Draw` で RGBA にまとめれば速くなる（読み取り確認）。
- alpha を捨てている（`r, g, b, _`）ため透過部分は黒。Flate の raw RGB 保存は肥大化（JPEG を DCTDecode で embed する判断は format 分岐を要る）。
- A-11 は memory の streaming 化まで解消済み（`docs/ai/tasks/2026-11-03-a11-pdf-raw-rgb-memory.md`）。本項目は速度・alpha の残存点。3746x2525 での体感は未測定。

### R-15（機構確定・方針判断要）: 先読み範囲と disk cache

- `resetBackgroundPrefetch` は `stripItems()`（全 scene items）を current からの距離順で queue する。±N 制限はない（`app/viewer.html:997-1034`）。
- client 側 objectURL は 2000 件の LRU cap 済み（`app/viewer.html:208`、`373-388`、`816`・B-02 実装済み）。指摘の「最大 2000 個の blob URL」は机构どおり、cap は既存。
- server 側 `cache/images`（full）と `cache/images/thumbs`（TN）に容量上限がない（`app/viewer.go:322-332`、`779-781`・prune 機構なし）。C-05 の bounded 化は memory cache 4 種と `indexTopCache`、`noFullCache`、disk galleryinfo 層まで（`docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`）で、image disk cache は範囲外。
- 先読み範囲の変更は TN display fallback 禁止方針（`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`）と strip の full DL 依存の帰結に触れるため、方針判断を要る。

### R-16（確定）: TN・download 書き込みが非 atomic

- `thumbProxy` は `os.WriteFile` 直書き（`app/viewer.go:780`）。クラッシュ時、壊れた TN が `tn-cache` として返り続ける（`serveStoredImage` は magic を確認しない、`app/viewer.go:350-371`）。
- `downloadFilesParallel` も `os.WriteFile` 直書き（`app/main.go:737`）。
- full cache 側は temp+rename・close 前 rename なし・失敗時 temp cleanup・magic 照合を持つ（`app/viewer.go:373-408`、`410-416`・A-07a）。共通化可能。

### R-17（確定）: findings の残存

- `addFinding` は `INSERT OR IGNORE`（`app/main.go:572-576`）。unique key は `(path, sha256, rule, detail)`（D-03 反映済み）。
- 同一 path で内容（sha256）が変わると新 sha256 の行は増えるが、旧 sha256 の行は消えない。現在の判定と食い違って見える。D-03 の合格条件（重複 skip）は満たしたまま、残存行の扱い未定義。

### R-18（確定）: 未読一覧

- `listRows("unread")` は `SELECT ... FROM read_state WHERE fully_read = 0 ORDER BY gallery_id` で LIMIT がない（`app/main.go:1387-1397`）。
- `viewed` だけ・`saved` だけの行も `read_state` に行を持つため `fully_read = 0` で含まれ、開いた gallery が増えるほど `/api/lists` と DOM（`app/search.html:689-691`）が大きくなる。

### R-19・R-20・R-21

- 読み取りで機構を確認し、それぞれ `docs/ai/known-issues/` へ記録（R-19 未検証、R-20 はローカル専用前提のセキュリティ除外方針、R-21 低優先）。実装項目としては登録していない。

## 優先順位（assess-20261106-1056 で確定・子の要約と個別タスク記録だけを根拠）

この節は「優先実装候補（指摘どおり）」を supersede する。実装開始指示は出ていない（優先順位の確定のみ）。

1. **R-01** 小・会計の正しさ（上書き時に old `entrySize` を引く）・常時稼働で効く・R-10 の依存（先行修正推奨）。テスト固定可。
2. **R-03** 小・正典 magic（`37 7A BC AF 27 1C`）と実コードが 1 バイト不一致。検出漏れは実測で確認済み。
3. **R-02** 小・Python（D-04 seek）と Go の非対称。実在 PE fixture で hit 漏れ。
4. **R-04** 小・機能停止が永続（一度 `missing_at_verify=1` の行は以後検出されない）。歴史記録を保持するか更新かは承認が必要。
5. **R-12** 小（`app/search.html:468` の card fetch 呼び出し削除）+ harness re-anchor。実測 33→25 requests（2 runs→1 run）。cold hygiene の汚染源を直接解消。案 B（generation を進めない）は UX 意味変更なので承認案件として分離。
6. **R-05** 案 A 承認済み・小。Windows 境界（予約名・末尾点/空白・255/260）は実測済み。既存フォルダは再命名しない。
7. **R-08** 案 A 承認済み・実測 94.3 ms → 6.0 ms（25 gallery・1 `ReadDir`）。response shape を維持側は guard test（`app/viewer_test.go:2203-2238`）変更不要。UI の per-entry `local` 意味は変えない。
8. **R-13** 案 A 承認済み・実測 5 requests → 1 call（3 箇所）。D-05 は fetch group と write group を分離維持が推奨。TN 正典（表示源・fallback・byte 保存）は維持。
9. **R-10** 実測 20–28 ms/page → ≤0.10 ms/page（warm path・mock）。bounded・copy 不変条件・key 正規化・random order 除外をテスト固定。R-01 依存。
10. **R-06** 小・path-key meta 表（推奨）。A-12 skip 意味の回復。PK 変更（不可逆）は回避済み。テスト 1 件追加必要。
11. **R-09** 小・card 経路の commit を 25→0（page 合計 26→1）で固定可。wall time 便益は A-02 より小。`last_seen` 更新の数据的 caveats は app 内読み出しなし。
12. **R-14** 実測 130–160 ms/page（1300 page で 320 s→147 s 目安）、割当 63 MB→27 MB、Flate raw RGB 20.63×肥大化。alpha 保持（`/SMask`）は出力の意味変更＝承認が必要。`draw.Draw` 案は A-11 gate の再導出が必要。
13. **R-15** 案 B 確定（全体先読み維持）。実測 memory cap 2000→4000 の差 347 MiB（近似）、cap 外 `prefetchedEntries` 7.8 MiB（無制限）。disk cap（各 8000 件）は DB 1 表（`cache_entries`）案が正典「filesystem scan で数えない」に適合。現データ（1,146 件・36 件）は cap 未到達。
14. **R-11** 小（値 90 ms + 設定 JSON + テスト 2 箇所 re-anchor）。「数十秒級」は 3語/5語 cold で実測不支持（gate 支配 451/631 ms、network 265 ms/呼では 100 ms と差ゼロ）。並行化は予算 1.5 s 超過時の検討案として記録・採用しない。
15. **R-16** 小・crash 限定だが TN への影響は永続（magic 照合なしで `tn-cache` として返る）。混ざりは 0.5%。atomic write の正典適用範囲は full cache のみで TN を明示除外 → TN・download への適用は方針変更の提案（承認が必要）。
16. **R-17** 小〜中・実測で内容変化後 stale finding 1 行残存（join view 0 行）。UI 読み取り口なし。案 A（歴史として残す・現判定 = join view 定義可）は対応不要、B1（`resolved_at` 追加）を推奨として承認案件に分離。
17. **R-07** 優先降格（旧 4 位）。現行 gate 下では default と 16 で接続数同一（改善ゼロ）。改善は burst/pause 条件のみ（18–53→8 接続、handshake 注入で wall −11.5%）。推奨は `http.DefaultTransport` 直接変更ではなく `Clone()` + `MaxIdleConnsPerHost=16` の helper。
18. **R-18** 対応不要（現状維持）。理由: 実測で 20,000 件でも JSON 1.3 MB・応答 53 ms・DOM 120 ms と許容内、正典上 `read_state` は cache でなく bounded 流儀（C-05）適用不可、D-03 非破壊と衝突。page 化・上限は一覧の意味（全件 vs window）と CLI `hub list` へ波及するため承認案件として分離。

### 照合で訂正した点（読み取り記録 → 子の実測）

- R-03: 正典 signature は 6 バイト `37 7A BC AF 27 1C`。`37 41 7A BC AF 27 1C`（7 バイト）は誤り。修正は最終バイト `38` → `1C`。
- R-08: guard test の実測行は `app/viewer_test.go:2203-2238`（読み取り記録の `2203-2224` を訂正）。`files` は `local N/M` 表示と cover fallback で使用済み（死荷重ではない）。
- R-14: 速度は実測済み（読み取り記録の「実測なし」を訂正）。`docs/ai/decisions/` 内の「YCbCr 130 ms」記録は実体なし。
- R-16: 原文の「TN 側は rename 衝突で内容が混ざる」は誤り（TN に rename がない。混ざりは同時直書き 0.5%、atomic 側は上書きで 0）。
- R-17: `no_full`・`error` は findings rule ではない（viewer の in-memory negative cache）。`deleteFindingsForGallery` は本番コードに存在しない。
- R-18: 前提「既読・お気に入り表は bounded（C-05）」は正典上不成立（C-05 は memory cache 限定、`read_state` は state 表）。
- R-11: `app/hitomi_test.go:1507-1528` は `Test429WithoutRetryAfter...`（1495 行）で 100 ms は `RetryBackoffMS`。更新が必要なのは `TestRateGateReturnsImmediatelyOnContextCancellation`（`app/hitomi_test.go:865`）と `TestConfigExampleHitomiKeysMatchCodeDefaults`（`app/main_test.go:1304`・`config.example.json:15`）。正典 pointer `2026-11-03-a05-...` は実体なし（採用: `2026-11-05-phase4-two-lane-gate-implementation.md`）。
- R-15: `viewer_test.go:816` の 2000 固定ではなく、pins は `app/viewer_test.go:2483/2567/2572`（`MAX_FULL_CACHE_ENTRIES` 文字列）。
- 正典 pointer 実体なし（実測で確認）: `docs/ai/tasks/2026-11-03-a14-search-freshness-checks-cache.md`、`docs/ai/tasks/2026-11-03-d13-search-freshness-decision.md`、`docs/ai/tasks/2026-11-03-b03-search-card-generation-guard.md`、`docs/ai/tasks/2026-11-03-b06-startup-double-search.md`、`docs/ai/decisions/2026-11-03-a05-interactive-search-priority-fifo-and-30s-grace.md`。
- 既存 verification blocker（全項目共通）: `python -m pytest tests/ -v` で `test_canonical_build_freshness_rule_exists` が 54 passed / 1 failed。原因は commit `e018c1c` で AGENTS.md Rules 条が `docs/ai/rules/confirmation-test-rule.md` へ集約され、テストが照合する literal pointer が AGENTS.md から消えたこと。known-issue: `docs/ai/known-issues/KI-20261106-04-canonical-build-freshness-pointer-missing.md`。

### 判断保留（有効性を左右する承認が必要）

- R-14: alpha 保持（PDF 表示の意味変更）と format 分岐の設計判断。
- R-17: findings 照会結果の意味を変える側（B1）。案 A は承認不要。
- R-18: page 化・上限（一覧の意味 = 全件 vs window、CLI 波及）。
- R-16: atomic write の TN・download への scope 拡張（正典 guard）。
- R-05: 命名変更（decision 2026-11-03 の「acquisitionTitleName 変更しない」と衝突）。既存 folder 衝突の実例はデータ依存。
- R-04: 歴史記録の扱い（update するか insert-only 保持か）。

### 対応不要（理由記録）

- R-18（案 A・現状維持）: 上記優先順位 18 位の理由。実施項目として閉じた（`task_manager end`）。
- R-21（known-issues・低優先）: `bytes.Replace` の脆さは機構確定。既存の低優先判定を維持。
- R-19（known-issues・実例待ち）: 実 gallery 実例が未発現。判定は未検証のまま。

## Verification

- 実装ファイル・UI・schema の変更はしていない。変更したのは `docs/ai/` の記録のみ。
- `task_manager add` で本記録タスクと R-01〜R-18 の 18 項目を登録（`current-task.tsv` = header + 19 行）。既知問題として R-19〜R-21 の 3 エントリを `docs/ai/known-issues/` に作成。
- 照合は read-only。実機確認・テスト実行・fixture 再現はしていない（R-03 の signature、R-19 の page 基準、R-14 の速度は実測なし。R-03 の `27 1C`、R-19 の実例未確認はそれぞれ未実行として明示）。
- `git status --short`: 照合開始時点で clean（本記録の staging は task_manager による）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
