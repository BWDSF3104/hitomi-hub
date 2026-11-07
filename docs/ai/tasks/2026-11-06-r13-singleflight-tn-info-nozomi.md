# R-13: thumbProxy / info / SearchNozomiParts に singleflight がない

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 A: 縮小画像・作品情報・nozomi 分割データへ同じ内容 1 回を追加）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

**実行固有ID**: assess-20261106-1056

## 意図 / 背景

- 指摘: dedup が full image fetch（`fetchFullImageOnce`）・cache write（`cacheWriteGroup`）・`EnsureGG` にしかない。`thumbProxy`・`v.info`・`SearchNozomiParts` は同一内容の同時リクエストを外部へ重複して出す。
- 判断（承認済み）: 案 A（`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）。追加は「既存流儀（key 単位・mutex・network 中に mutex を保持しない）に揃える」形。
- 本記録の範囲: 読み取りと scratch replica の実測のみ。ソースコード・既存テストは変更していない（変更したのはこのタスク記録と `docs/ai/tmp/assess-20261106-1056/r13/` の scratch）。
- 実測の限界: `client.fetch` の mock 注入は in-package test 専用（`client.fetch` が非公開、package main）。scratch から実コードを呼べないため、数値は機構同一の最小 replica 実測として記録する（流儀は `newTestViewer`・`httptest`・join observer）。

## 調査結果 / 変更内容

### 現在の実装（確度: 確定）

- dedup 既存 3 件
  - `fetchFullImageOnce`（`app/viewer.go:787-823`）: key = `imageFetchKey(hash, format, gg)`（`negativeNoFullKey` 再利用・`app/hitomi.go:264-266`）。register → mutex unlock → network → relock → delete → close(done)。waiter は leader result を共有、joiner は gate lane を消費しない。leader fetch は `context.WithoutCancel`。
  - `writeImageCacheAtomically`（`app/viewer.go:373-408`）: key = `cachePath`。existing target skip、group、result 共有、join observer `cacheSingleflightJoinHook`。
  - `EnsureGG`（`app/gg.go:118-202`）: key = gg.js URL。mutex は network 中に保持しない。joiner は `<-call.done`（ctx guard なし）。
- dedup なし（追加対象）
  - `thumbProxy`（`app/viewer.go:758-785`）: `serveStoredImage`（tn-cache hit）後に `client.getImage` を request ごとに呼ぶ。TN write は `os.WriteFile` 直書き（`app/viewer.go:779-780`、atomic 化は R-16 の範囲）。
  - `v.info`（`app/viewer.go:1074-1114`）: memory hit → disk hit → remote。memory・disk を通過した request は `client.Gallery` を request ごとに呼ぶ。
  - `SearchNozomiParts`（`app/searchhtml.go:114-153`、fetch は `app/searchhtml.go:135`）: `searchCache` TTL hit を通過すると `c.get` を request ごとに呼ぶ。

### 有効性: 有効（確定・実測で数値確認）

同一 key の同時 5 リクエストで、外部呼出し回数は追加前 5 → 追加後 1（3 箇所すべて）。数値（scratch replica 実測、`go test -v ./r13`）:

| 箇所 | key | baseline（現行） | proposed（追加） | join | gate slot | marker record |
|---|---|---|---|---|---|---|
| thumb（TN） | hash+TNFormat+variant | 5 requests → 5 calls | 5 requests → 1 call | 4 | 5 → 1 | 5（per request 維持） |
| info | id | 5 requests → 5 calls | 5 requests → 1 call | 4 | 5 → 1 | — |
| nozomi | 既存 cacheKey | 5 requests → 5 calls | 5 requests → 1 call | 4 | 5 → 1 | — |

- key 境界（別 group）実測: nozomi は cacheKey 2 本で 6 requests → 2 calls、TN は variant 2 本で 6 requests → 2 calls。
- mutex を network 中に保持しないことの実測: leader が network 中に別 key の request が elapsed 0 ms で完了（2 calls）。
- waiter の cancellation は leader に伝播しない: canceled waiter は 502・leader 完了・calls 1。
- joiner の x-cache は source を保持（`tn`）。実測 codes `[tn tn tn tn tn]`。

### key 設計（推奨）

- TN: key = TN identity（hash + `TNFormat(format)` + variant）に gg を加える。理由: 既存 `imageFetchKey` 流儀と揃え、subdomain が違う fetch を join しない。cache path は gg を含まない（`app/viewer.go:322-332`）ため、gg 違いの group は write helper（key = cachePath）で収束する。
- info: key = id。`background` は gate lane 判定だけで内容同一 → key に含めない。
- nozomi: key = 既存 `searchCache` の cacheKey（`app/searchhtml.go:114-125` で確定済みの文字列）。`termIDs` が `state` を更新する（`app/searchhtml.go:155-183`）ので、key は state 参照ではなく cacheKey 文字列で持つ。joiner は per-call copy（`copyInts`）を返す（実測: 一方の変更が他 waiter へ泄漏しない `first=[99 22 33]` / `second=[11 22 33]`）。
- info は既存 `infoCache` が pointer を共有しているので、joiner 共有は既存意味と同一（新規リスクなし）。

### D-05（group write との衝突）照合 — 推奨: 分離を維持

- 前提の補正: 現行 `fetchFullImageOnce` は `cacheWriteGroup` を呼ばない。cache write は caller（`app/viewer.go:852-859`、`app/viewer.go:867-874`）が request ごとに `writeImageCacheAtomically` を呼ぶ。したがって「共有応答で他の waiter へ group write が届かない」は現行コードでは発生していない。
- 実測（分離案 = 推奨、4 requests）: helper 呼び出し 4 回・実 write 1 回・write group join 3 回・leader の write failure の result が 4 件すべてへ届く。
- 実測（fetch group に write を抱合する案）: helper 呼び出し 1 回（leader のみ）・error が届くのは 1 件だけ（4 件中 1）→ group write が waiter へ届かない。
- 推奨: fetch group と write group を分離して維持する（joiner も write helper を呼ぶ形）。TN を単一 group に抱合しない。TN write の atomic 化は R-16 の範囲であり、本項目では流儀の照合にとどめる。
- joiner の待ちは ctx select を選ぶ（`EnsureGG` の join は ctx guard がない。leader がハングすると joiner が永久待ちになるため `fetchFullImageOnce` 流儀に揃える）。

### TN 正典照合（破らないことを確認）

- 参照実在（Glob 確認済み）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`（追加で `docs/ai/decisions/2026-11-01-tn-no-store-object-url-display.md`）。
- 本項目の案は fetch dedup のみを追加し、TN の表示源（search-card cover 専用・reader/`#thumbs` 表示源にしない）、TN display fallback 禁止、TN バイトを full cache に保存しない、`x-cache` 判定、generation guard を変えない。
- 実測: TN 失敗時 `x-cache=no_tn`（code 502・full へのフォールバックなし）、TN write は `cacheRoot/thumbs` 配下のみ（full cache への write 0）、joiner の `x-cache` は source（`tn`）を保持。
- 却下案: joiner に `tn-cache` を返す案（x-cache 判定を弱める）、TN バイトを full cache へ保存する案、TN 失敗時に full へフォールバックする案。

### 副作用・影響範囲

- gate lane: joiner は slot を消費しない（実測 5 → 1）。politeness gate の FIFO・将来予約・grace 意味は変えない（leader 1 回だけ lane を使う）。既存の data lane interval（90 ms・R-11 値確定）と矛盾しない。
- metrics: joiner は `recordMetric` を呼ばないため call 回数が減って見える。既存 dedup（`fetchFullImageOnce`）と同じ挙動で、新規の意味変更はない。
- `recordMarkerRequest`（`app/metrics.go:69-74`）は handler 側（per request）に残す。joiner の priority 記録を消すと Phase 2 priority 意味が変わる（実測 marker_calls=5 を維持）。
- 既存 guard test は変更しない（`app/viewer_test.go:2203-2224` 等）。単一 group 化で per-request の side effect（`recordDiscovered`・DB 書き込み・response header）を skip しないこと。dedup は network fetch 層のみ。
- 影響ファイル（実装時）: `app/viewer.go`（`thumbProxy`・`info`）、`app/searchhtml.go`（`SearchNozomiParts`）、`app/viewer_test.go`（新規テスト 3 件 + key 境界 1 件）。既存 helper 型（`imageFetchCall`・`singleflightCall`・`cacheWriteCall`）を流儀として共用。UI（served 画面）は x-cache 判定と generation guard に影響しないため確認不要（未実施）。

### 代替案（比較した結果）

- 抱合案（fetch group に cache write を入れる）: group write が waiter へ届かない（実測 1/4）。採用しない。
- joiner に `tn-cache` を返す案: x-cache 意味を変え、正典の full/TN 判定を弱める。却下。
- key から gg を外す案: 内容同一の group 数は減るが、既存 `imageFetchKey` 流儀とズレ、subdomain 違いを join する。write helper で収束する利点是致だが、流儀維持（gg を key に入れる）を推奨。

### 実装（案 A・実施済み 2026-11-07・assess-20261106-1056）

- `app/viewer.go`: `thumbProxy` → `fetchTNOnce`（key = `tnFetchKey`（`negativeNoFullKey` 流儀 + variant））。joiner は ctx select（TN は request ctx を持つ）。`info` → `resolveInfoOnce`（key = id・joiner は shared pointer）。正典不変条件（TN display fallback 禁止・TN バイトを full cache に保存しない・full 取得経路の順序・x-cache・generation guard）は維持。TN write は per request の `os.WriteFile` を維持（atomic 化は R-16）。joiner は gate lane を消費しない（leader のみ fetch）。
- `app/hitomi.go`: `tnFetchKey` 追加、`nozomiFetchCall` 型、Client の group field（`searchGroup`/`searchGroupMu`/`searchGroupJoinObserver`）。
- `app/searchhtml.go`: `SearchNozomiParts` に singleflight group（key = 既存 cacheKey）。joiner は `copyInts` を返す。network 中に mutex を保持しない。
- 流儀の照合（D-05・推奨）: fetch group と write helper を分離維持（TN write は handler 側 per request）。既存 helper（`imageFetchCall`/`cacheWriteCall`）流儀を共用。
- 採用した判断（段階1）: joiner の wait は `fetchFullImageOnce` の ctx select に揃える方針だが、`info`・`SearchNozomiParts` は既存 signature に ctx を持たない。signature 変更は承認範囲外のテスト（`galleryinfocache_test.go`・`searchhtml_test.go`）の更新を要するため、これらは `EnsureGG` 流儀（ctx guard なし `<-call.done`）で join。leader fetch は常に完了（client timeout 60 s）→ `close(done)` されるので joiner は永久待ちにならない。TN は request ctx を持つため ctx select を適用。既存 guard test（viewer_test.go:2203-2224）と既存テストは変更なし。

### 実装後の実測（合格条件）

- TN・info・nozomi: 同一 key の同時 5 リクエスト → 外部呼出し 1 回・join 4（`TestViewerThumbProxyConcurrentTNFetchDedupsToSingleCall` / `TestViewerInfoConcurrentRemoteFetchDedupsToSingleCall` / `TestSearchNozomiPartsConcurrentFetchDedupsToSingleCall`）。
- key 境界: TN variant 2 本 → 2 calls、nozomi cacheKey 2 本 → 2 calls（同じ test 内の key 境界セクション）。
- joiner の x-cache は source（tn）保持（tn-cache にしない）。marker record は per request（5）維持。joiner gate lane 消費なし。TN 失敗は `no_tn`・full へ fallback なし。joiner の cancellation は leader に伝播しない（TN・canceled waiter 502・leader 1 call）。nozomi joiner は per-call copy（独立 backing）。info joiner は shared pointer。

## Verification

- `go vet . ; go test -count=1 -timeout 300s .`（`app`、既存テスト変更なし・ソース変更なし）: vet clean、`ok hitomi-hub/app 26.799s`。
- scratch 実測: `go test -v -count=1 -timeout 120s ./r13`（module `assess20261106-1056` 配下 `docs/ai/tmp/assess-20261106-1056/r13/singleflight_measure_test.go`）→ `ok assess202611061056/r13 1.514s`。7 test PASS。数値は上記表と D-05 照合欄（separation: helper 4 / attempts 1 / joins 3 / errors 4、merged: helper 1 / errors 1）。
- 実装後の検証（本項目分・2026-11-07）: `app` で `go vet .` clean。`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 25.984s`（新規テスト 3 件 + key 境界 1 件・既存テスト変更なし）。新規テスト: `TestViewerThumbProxyConcurrentTNFetchDedupsToSingleCall`（calls 1・join 4・joiner x-cache `tn`・marker per-request 5・canceled joiner `no_tn` 502・leader 1 call・full fallback なし）、`TestViewerInfoConcurrentRemoteFetchDedupsToSingleCall`（calls 1・join 4・shared pointer）、`TestSearchNozomiPartsConcurrentFetchDedupsToSingleCall`（calls 1・join 4・joiner copy 独立・key 境界 TN variant 6→2・nozomi cacheKey 6→2）。
- `python -m pytest tests/ -v`: `1 failed, 56 passed`。failure は `test_canonical_build_freshness_rule_exists`（既存 failure・本項目不合格ではない）。本項目は Go のみ変更（Python・AGENTS.md 非触）。
- network 1 call 実証: 既存流儀（`newTestViewer`・`httptest`・join observer）の mock 実測で合格（上記新規テスト）。cold hygiene 実機 network は本項目の合格条件に不要（UI x-cache 判定・generation guard に影響しない）。UI（served 画面）確認: x-cache 判定と generation guard に影響しないため確認不要として記録（未実施・画面キャプチャなし）。画面確認不可項目を合格条件に含まない。
- 編集後の文件破損確認: `git diff` で日文コメントと実装内容を確認済み（edit ツール使用・`Set-Content` パイプライン不使用）。変更ファイルは `app/viewer.go`・`app/hitomi.go`・`app/searchhtml.go`・`app/viewer_test.go`（承認範囲内）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
