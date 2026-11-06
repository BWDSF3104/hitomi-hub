# R-15: viewer が gallery 全体を先読みし、thumbs/full の disk cache に上限がない

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計確定（memory 4000 件・設定化、disk 8000 件ずつ、disk 削除は作成順、memory 削除は保存済み status 優先→LRU、現在閲覧中・表示中は捨ない、通知 UI なし、last access 記録なし、sha256 lookup なし）。件数と作成順と status は DB の 1 表（例 cache_entries: hash、format、kind、created_at、source_status）で持つ。テスト viewer_test.go:816 の 2000 固定更新。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

- viewer は gallery 全体（strip items 全部）を 8 並行で先読みし、client の objectURL cap は 2000 件（定数 `MAX_FULL_CACHE_ENTRIES`）、server の disk cache（`cache/images`・`cache/images/thumbs`）には容量上限・prune 機構がない、という指摘の照合と実測。
- 確定済みの承認・指示（前提）: 全体先読み維持（window 縮小不採用）、memory cap 4000 件・設定化（例 `viewer.max_object_urls`、0 は既定、下限 1000）、disk cap 8000 件ずつ（件数・byte 計算なし）、disk 削除は作成順、memory 削除は保存済み status 優先→LRU、現在閲覧中・表示中は捨ない、件数・作成順・status は DB の 1 表で持つ（filesystem scan で数えない）、通知 UI 置かない。
- 本作業は読み取り＋実測のみ。ソースコード・schema・実 cache フォルダ・実 DB 内容は変更していない（変更したのは本タスク記録と scratch `docs/ai/tmp/assess-20261106-1056/` のみ）。

## 調査結果 / 変更内容

### 有効性: 有効（確定）

指摘の機構は現コードから確定でき、承認済み方針（案 B・上限・削除順）に対して有効な項目。実測で memory 差・cap 外成長・現況件数を数値で確定。判断保留なし。

### 現在の実装と判断根拠（読み取り）

- client（`app/viewer.html`）:
  - `PREFETCH_CONCURRENCY = 8`・`MAX_FULL_CACHE_ENTRIES = 2000`（`viewer.html:207-208`）。先読み queue は `stripItems()` 全部を current からの距離順で並べ、±N window 制限なし（`resetBackgroundPrefetch`・`viewer.html:997-1034`）。
  - objectURL 系 Map/Set（`fullObjectUrls`・`fullObjectUrlLru`・`activeObjectUrlKeys`）は cap 対象（`evictFullObjectUrls`・`viewer.html:390-403`、B-02 実装済みの per-entry LRU、表示中・gallery 保護 key skip）。
  - cap 対象外の Map が separate に成長する: `prefetchedEntries`・`countsByGallery`・`fullCache`（status のみ entry）。`updatePrefetchCounts`（`viewer.html:763-791`）は current gallery の計上のみだが、gallery を切り替えても既に入った key は消えない。cap 超過時に減るのは eviction された objectURL entry の key だけ。
  - status「local」entry は objectURL を持たない（local_formats がある entry は blob memory に入らない: `prefetchPageImage` の local-direct 短絡・`viewer.html:828-833`）。memory 削除順の「保存済み status 優先」は実運用では status 集合（約 653 B/entry）に対する優先度であり、blob LRU の実質大部分（cache/miss）は LRU 順どおり。実装は `sourceStatusByEntry` の書き込み時記録値を見る形で追加 lookup なし（承認どおり）。
- server（`app/viewer.go`）:
  - `safeCachePath`（`viewer.go:311-320`）と `safeThumbnailCachePath`（`viewer.go:322-332`）はフォルダ内 path を組むだけで、件数・容量上限、prune 機構なし。
  - cache 書き込み（`writeCacheFileOnce`・`viewer.go:423-484`）は temp+rename のファイル書き込みのみで、DB への記録行を持たない。`files` 表は scan 成果物（scan 判定の `scan_status` CHECK 値）のみで、cache 項目は入らない（`recordFile` 経路は scan 側・`app/main.go`）。
- DB（正典 `schema.sql`）: `cache_entries` に相当する表なし。`files`・`pageorder`・`scan_cache` はあるが、件数・作成順・cache status を filesystem scan 抜きで保持する手段はない（`pageorder` は hash→gallery 解決に使えるが未 scan gallery の cache 項目は行を持たない）。

### 実測（memory 使用量差・cap 外成長）

既存 node harness 流儀（`tests/harness_lib.mjs` の `bodyIn` で `app/viewer.html` の実ブロックを抽出し `new Function` で実行）の scratch ハーネス `docs/ai/tmp/assess-20261106-1056/memory_cap_harness.mjs`。blob 相当バイト数を固定: 実測の `cache/images` 平均 **172,179 B/件**（197,317,546 B ÷ 1,146 件）。Node RSS 近似（ブラウザ V8 とは環境差、近似値として提示）。

- memory cap 2000 まで填充: RSS delta **358,457,344 B ≈ 342 MiB**（179,229 B/件）
- memory cap 4000 まで填充: RSS delta **722,251,776 B ≈ 689 MiB**（180,563 B/件）
- 差（2000→4000 の増分）: **363,794,432 B ≈ 347 MiB**（理論値 2000 × (172,179 + 構造 overhead 2,623) = 349,604,000 B と 4% 以内一致）
- 構造 overhead 実測: 約 **2,623 B/件**（blob 除く Map/Set/objectURL 文字列）
- `countsByGallery`・`prefetchedEntries`（cap 対象外）: 実測 DB 規模（galleries 125）× 代表 gallery 100 件で `prefetchedEntries` **12,500 件**・`countsByGallery` 125 行・`fullCache` status みの 12,500 件 → RSS delta **8,167,424 B ≈ 7.8 MiB**（653 B/件）。セッション内は無制限成長（cap 超過で減るのは eviction された objectURL key のみ）。
- ハーネス 10 checks passed（cap 超過時 eviction・保護 key 保持・LRU 順・cap 4000 で 4000 保持を機構確認）。

### disk cap（8000 件）実現の案比較と推奨

- **案 A（推奨）**: DB の 1 表 `cache_entries`（hash、format、kind、created_at、source_status）。書き込み時に 1 行 INSERT、削除時に 1 行 DELETE。件数・作成順は SQL だけで取れ、正典ルール「filesystem scan で数えない」に適合。追加コストは 1 行分。「現在閲覧中の gallery は捨ない」は書き込みリクエストの gallery 判定で解決できるが、例表に `gallery_id` 列が必要（hash→gallery は `pageorder` で解るが未 scan gallery の cache 項目に行がない）。書式は「例 cache_entries」の範囲内なので列追加は実施段階の判断として記録。
- **案 B（不可）**: 既存 `files` 表の status + cache ファイルの存在で数える案。cache 書き込みは `files` に行を作らない（`writeCacheFileOnce` は DB に触れない）。数えるには (a) filesystem scan（正典ルール違反）、(b) `files` に cache 行を挿れる（`scan_status` CHECK は scan 判定値のみで cache status が入らず scan 意味を汚す）、(c) 作成順を mtime で読む（scan 読み）。いずれも正典ルールに対して不可。よって案 A が適切。
- **正典矛盾の記録（判断ルール適用）**: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope.md` の Rejected proposals に「Do not add DB tables for image cache state」・Accepted facts に「image_files / gallery_files table additions are not accepted」があり、これは newer の `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（確定 2026-11-06 10:52）が「件数・順・status は DB の 1 表（例 `cache_entries`）で持つ」と指示することと矛盾する。日付新しい文書を優先し、2026-11-06 の決定を採用（2026-11-02 の当該条はこの点のみ supersede）。実装段階で supersede 記録を decisions へ立てる。

### 副作用・影響範囲（正典照合）

- B-02 LRU（表示中捨ない）: memory cap 値変更と status 優先追加は弱化しない（ハーネス run D で cap 4000・保護 key 保持・最古 unprotected 削除を確認）。
- C-05 bounded 化（memory cache 4 種・top/noFull/galleryinfo）: server 側の memory cache は別物で、disk cap は影響しない。
- D-02・image cache path rules（`docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope.md` と correction）: 永続先 `cache/images`（既定）、temp を同一 cacheRoot 内、scratch/output 禁止、TN を full cache に保存しないは維持（案 A は DB 行＋フォルダ内ファイル削除のみで永続先を移さない）。
- TN rules（TN display fallback 禁止・TN を原寸保存として数えない）: `kind`（full/thumbs）で件数を集計し、folder ごとに 8000 件ずつ。TN の表示源・判定は変わらない。thumbs 書き込みは R-16（非 atomic）と同一地点（`viewer.go:780`）で、prune 追加は書き込み直後の 1 行 DELETE 相当。
- `x-cache` 判定・generation guard: 弱化しない。disk 項目が消えた後の同一 hash 要求は `cache` hit から `miss`（remote 再取得・gate 消費）になるだけで、判定経路と `no_full` 意味は不変。これは承認済みの「一度見て二度と使わない分を消すこと」の許容どおり（再取得待ちを不便としない運用は memory→disk 再表示で担保）。
- schema 追加（`cache_entries`）は実施段階の判断（既存 schema は変更していない）。DB は単一接続・WAL（`openDB`）。

### テスト関連（読み取り修正）

- 指摘記録の `app/viewer_test.go:816` は `imageFetchKey` の format/gg 分離テストで、2000 を固定していない（当回の読み取りで確認）。cap 関連を固定しているテストは `app/viewer_test.go:2483`・`2567`・`2572` の文字列一致チェック（`MAX_FULL_CACHE_ENTRIES` 識別子と eviction 文の文字列を固定）で、値 2000 自体は固定していない。設定化で変数名を変えるとこの 3 箇所が壊れる。識別子 `MAX_FULL_CACHE_ENTRIES` を保ち値だけ config（既定 4000・0→既定・下限 1000）へ移す形ならテスト変更は当該文字列の更新で済む。

### UI 確認

- 未確認。通知 UI を置かない項目のため、served 画面の before/after 比較が成立しない（画面に変化がない）。Google Drive への保存なし。正典ルール参照: `docs/ai/rules/confirmation-test-rule.md`（実装段階で canonical build・served 画面確認・cold hygiene・`x-cache`・generation guard が合格条件として残る）。

### 推定規模

- memory 側: 小（定数の設定化＋ページへの受け渡し＋文字列テスト 3 箇所更新＋evictFullObjectUrls の status 優先フィルター）。
- disk 側: 中（`cache_entries` 表追加＋書き込み時 1 行 INSERT・削除時 DELETE＋書き込み後の prune 判定（kind・作成順・現在 gallery skip））。UI なし。

### 優先順位付けに効く要素

- 重要度: 中（無制限成長の予防。実測では `cache/images` 1,146 件・`thumbs` 36 件で 8000 件 cap は現況未到達）。
- 緊急度: 低（機能停止・誤判定はなく、上限到達時の挙動設計のみ）。
- リスク: 正典 decision 矛盾の supersede 記録要否（上記）、テスト文字列 3 箇所の破損、browser 実測（Node RSS 近似）の乖離。
- 依存: 設定 JSON（`viewer` 節）の schema・下限 guard、schema migration（新表追加のみ）、R-16（TN 書き込みの atomic 化）と同一地点、`files`/`pageorder` を使う案 B の不可確認済み。

## Verification

- 実測コマンド 1: `python -m pytest tests/ -v` → 54 passed / 1 failed（9.51 s）。failed は `test_canonical_build_freshness_rule_exists`（AGENTS.md のポインタ確認）で R-15 と無関係（本項目は read-only、コード未変更、既存状態）。R-15 関連テストは passed。
- 実測コマンド 2: `node tests/search_prefetch_harness.mjs` → 42 checks passed（0.1 s）。既存 harness 実行可能。
- 実測コマンド 3: `node --expose-gc docs/ai/tmp/assess-20261106-1056/memory_cap_harness.mjs` → 10 checks passed（0.8 s）。数値は上（memory 342 MiB → 689 MiB、差 347 MiB、cap 外成長 7.8 MiB）。
- 環境実測（read-only）: `cache/images` 1,146 件 / 197,317,546 B（平均 172,179 B）、`cache/images/thumbs` 36 件 / 1,569,072 B、`hitomi.db` galleries 125・files 6。disk cap（8000 件）は現況未到達。
- 制約遵守: 既存テスト変更なし、実 cache フォルダ・実 DB 内容変更なし（DB は read-only URI `mode=ro`）、本番コードディレクトリへの書き込みなし（scratch は `docs/ai/tmp/assess-20261106-1056/`）、1 コマンド 1 行・インラインコード実行禁止（`db_stats.py`・`memory_cap_harness.mjs` をファイルとして実行）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
