# N-16: GET の /api/* と /files/* に Host 検査がない（R-20 承認済み・未実装）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: GET の `/api/*` と `/files/*` に Host 検査がない（前批 R-20 は案 A 承認済み・未実装。`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）。外部にポートを公開していなくても DNS rebinding は成立し、同じブラウザで開いた悪意あるページが名前解決を 127.0.0.1 へ切り替えて読み取れる。既存の `isLoopbackHost` を GET にも適用すれば塞げる（LAN 運用時は configured private-network/host allowlist への更新が必要）。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-16 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- GET 経路に Host 検査なし（実測で閉じた）。route 登録 `app/viewer.go:175-196`、`withPOSTSecurity` は `r.Method != POST` でそのまま next へ（`app/viewer.go:199-216`、通過は 201-204）。POST 検査は `sameOriginPost`（`app/viewer.go:218-238`、`isLoopbackHost` 呼出は 223）で効く。
- `isLoopbackHost`（`app/viewer.go:240-254`）は `strings.ToLower`（241）+ `==` 比較（253）。**原文の「`strings.EqualFold` を使う」は現行コードの事実として無効**（EqualFold は不在）。case 耐性は ToLower で既に成立（`LocalHost` → `localhost` で一致）。ASCII host 名で EqualFold と ToLower+== の判定結果は同一（sim 表 `LocalHost` = 200）。
- 実測（fresh canonical build・fresh instance・port 8747・mock seed・実 hitomi API 不使用）: `Host: evil.example.com` の GET `/api/image/a1b2…dee11?format=webp` → **200**（`x-cache: cache`・32 bytes）。GET `/api/lists` evil Host → **200**（loopback Host → 200）。POST `/api/fav` evil Host → **403**（既存検査が効いている）。注記: `/api/info` は route として存在しない（GET route は 175-195 の集合。`/api/gallery/{id}` が info 経路）。

### 有効性: 有効（実測で閉じた）。原文に2点の訂正

1. 「mutation endpoint を叩ける」は現状記述として無効。DNS rebinding で Host は attacker domain になるため `sameOriginPost` の loopback 要求（`app/viewer.go:223`）で POST は 403（実測）。塞がれていないのは GET 読み取り（lists・gallery・search・files・image）。原文の中心指摘（GET の Host 検査未実装）は有効。
2. EqualFold 指摘は現行実装（ToLower）と異なる（上記）。case 耐性は現行で十分に成立。

### 正典照合

- 前批案 A（`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` R-20・承認済み）と KI-20261106-02（未解決・実装未実施）は現 HEAD と矛盾なし（code 未変更・実施開始指示待ちで一致）。
- LAN: LAN 運用は正典で承認済みではない（`docs/ai/decisions/2026-11-01-viewer-single-page-web.md`「既定 bind 127.0.0.1・LAN に公開しない」）。allowlist の意味変更（loopback 之外を許可）は承認案件として分離。案 A の sim 形（loopback・literal IP のみ・reverse DNS なし）は正典決定と一致。
- DNS rebinding 成立条件の照合（KI-20261106-02 記載まで）: origin（URL hostname）の名前解決が 127.0.0.1 へ切り替わり、リクエストの Host が attacker domain のまま server へ届く。Origin/Host の不一致で POST 検査は 403 になるが GET は無検査 → 読み取り成立。code と一致。

### 実現可能性と推奨修正方法

推奨: 既存 `isLoopbackHost` を GET にも適用する middleware へ拡張（`withPOSTSecurity` を要求元検査として GET/POST 共通化。GET は loopback Host 判定のみ、POST は既存 same-origin Origin 検査維持）。応答は 200/403（案 A・sim 表参照）。EqualFold への変更は機能上不要（判定同一）。
sim（`docs/ai/tmp/assess-20261108-1332/r16/r16_sim.py`）:

| Host | 現状 GET | 案 A 実装後 GET |
|---|---|---|
| localhost | 200 | 200 |
| 127.0.0.1 | 200 | 200 |
| [::1] | 200 | 200 |
| evil.example.com | 200 | 403 |
| LocalHost | 200 | 200 |

既存テスト照合（変更なし）: `httptest.NewRequest` の既定 Host は `example.com`（非 loopback）で、GET 成功を固定している — `app/viewer_test.go:119/136/158`（page source）、`277`（lists）、`376`（image proxy）。実装時はこれらを loopback Host へ re-anchor する必要があり（現状（検査なし）を固定しているテスト）。POST 側テスト（`299-368`）は既存検査と整合・非接触。

### 副作用・影響範囲（今の挙動 → 変更後の挙動）

- 今の挙動: 非 loopback Host の GET が成功（rebound origin のページから既読・お気に入り・保存検索・files・画像が読み取り可能）。browser は Host を page fetch で上書き不可（禁止ヘッダ・Playwright 注入実測 `net::ERR_INVALID_ARGUMENT`）→ 成立条件は DNS 名前解決の切替。
- 変更後: loopback origin の画面（objectURL 表示・fetch）は現状と同一（sim 200→200）。非 loopback Host（LAN アクセス・`localhost`・`127.0.0.1`・`[::1]` 以外の hostname・別 literal IP）の既存運用は GET が 403 になり、served 画面の fetch・objectURL 表示が失敗として現れる → LAN allowlist 拡張は承認案件として分離（正典未承認）。x-cache・no-store fetch・generation guard は middleware 追加で弱まらない（cache 判定・header 設定経路非接触）。
- N-15 との補完: N-15 の escape は served origin 内での script 実行を止める層。script が正典 origin 内で実行されれば Origin==Host（loopback）で `sameOriginPost` を通過するため、Host 検査は注入防御ではなく origin 検証（信頼境界）として独立の層。両者は補完関係（N-15 記録の照合済み）。

推定規模: 小（middleware 共通化 5–8 行 + GET 固定テスト 5 件の re-anchor）

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean・ok 25.501s（既存テスト・コード変更なし）。
- `python -m pytest tests/ -v` → 59 passed / 1 failed（10.57s）。不合格は既知 `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約）＝本項目非接触。
- 実測（fresh canonical build・fresh instance・pid 8328・port 8747・mock seed・実 hitomi API 不使用）: GET image evil Host → 200（x-cache: cache・32 B）、GET lists evil Host → 200・loopback → 200、POST fav evil Host → 403。
- sim: `python docs/ai/tmp/assess-20261108-1332/r16/r16_sim.py` → 表どおり（evil 403・loopback 系 200・LocalHost 200）。
- 画面確認: served 画面（isolated origin `n16-assess-20261108`・window outer 1678x1025 / inner 1665x932）で loopback origin の fetch 200・image fetch 200（x-cache: cache・32 B）を確認。不正 Host path は browser で再現不可（Host 禁止ヘッダ・`net::ERR_INVALID_ARGUMENT`）→ **未確認**（DNS 変更を要る）。操作による表示変化なし → 前後1組。
- Drive 保存先: `I:\マイドライブ\kilo\2026-11-08-n16-get-host-check-not-implemented\`（`n16-served-screen-loopback-origin-20261108.png`、`n16-get-host-check-not-implemented-confirmation-2026-11-08.txt`）。project root 内 local copy なし（ゴミ箱送り対象なし）。project root 外残件（削除せず記録）: `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\n16-served-screen-loopback-origin-20261108.png`・`console-2026-11-08T12-01-54-311Z.log`（favicon 404 1件は mock 由来・項目証拠ではない）。
- scratch（gitignore・`docs/ai/tmp/assess-20261108-1332/r16/`）: `r16_sim.py`・`r16_seed.py`・`plan_a_sim_server.py`・fresh build `hub_r16.exe`・seed cache。server（pid 8328・8747）と sim server（pid 22944・8748）は確認後に停止。ビルド時に `go build -o` の相対指定で `app/docs` に exe が出たため、scratch へ移し `app/docs` は正典 `kilo-safe-rmdir` でゴミ箱へ。

## 2026-11-09 再調査（assess-20261109-1320）

- 項目ID: `#19` / `assess-20261109-1320`。この節のみ追記。`Status` と `Next Action` は変更しない。
- 有効性: 有効（GET 読み取り経路の Host 検査未実装、案A承認済み・実施開始指示待ち）。前批の中心判断（GET Host 検査未実施）は現行コードと一致。

### 現行コード

- `app/viewer.go:213-230` の `withPOSTSecurity` は非 POST をそのまま `next` へ通過。GET 経路に Host 検査なし。
- POST は `app/viewer.go:232-252` の `sameOriginPost` で loopback Host と Origin を検査。
- `isLoopbackHost` は `app/viewer.go:254-268`。`strings.ToLower` + `==` 比較。`strings.EqualFold` は不在。原文の EqualFold 指摘は現行実装事実として無効。

### 実測とテスト再確定

- baseline: `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean、`ok hitomi-hub/app 27.812s`。
- `httptest.NewRequest` の path-only GET の既定 Host 実測: `example.com`。`jsonPOSTRequest` は POST 用 `req.Host = "127.0.0.1:8731"` を設定。
- 前批の `119`, `136`, `158`, `277`, `376` は line-level で、対応テストは `TestViewerPageSourceLocalFirst`, `TestViewerPageSourceUsesHasavifFormatWhenRemote`, `TestViewerReadFavoriteAndQueryHandlers`, `TestViewerImageProxyServesSameOriginWebp` に含まれる。
- 現行の relevant GET handler tests は test-level で **54 件**（line-level GET usage **85 件**）。前批 5 件は test-level 相当としては不足。
- scratch copy `docs/ai/tmp/assess-20261109-1320/r19/appcopy` に最小案A patch を適用。Patch code: `if !isLoopbackHost(r.Host) { writeJSONError(w, http.StatusForbidden, "host not allowed"); return }`。`go vet .` clean。
- full `go test` は単独 run の完全な件数として信用できない。理由: `TestViewerReadFavoriteAndQueryHandlers` の panic（`viewer_test.go:282`）で test binary が abort し、以降の GET 成功 tests が実行されない。さらに `TestViewerImageProxyFetchDedupsConcurrentRemoteFetch` は patch 下では 403 で test が wait して timeout/hang する。
- 個別 `-run` で current relevant 54 tests を再実測。`docs/ai/tmp/assess-20261109-1320/r19/reanchor_indiv_fail.txt` に 54/54 件の FAIL/hang を記録。

### Current relevant tests（54 test-level）

`TestBackgroundPrefetchMarkerRecordedSeparatelyInMetrics`, `TestCacheClearResetsMemoryCachesWithoutChangingCacheHeaders`, `TestCacheEntriesPruneEvictsOldestByCreationOrderAndKeepsCurrentGallery`, `TestImageCacheWriteRecordsCacheEntryRow`, `TestImageProxyCacheConcurrentWriteSingleflight`, `TestImageProxyCacheMagicCheckSkipsMismatchedBytes`, `TestImageProxyCacheMissWritesAtomically`, `TestImageProxyCacheStateIsNotTN`, `TestSearchCardHandlerIsReadOnlyForExistingRow`, `TestSearchCardViewedAndSavedStates`, `TestSearchFirstStatusHandlerShapeAndReadonly`, `TestSearchHandlerIncludesMeasurementShape`, `TestSearchHandlerRecordsAllIDsInOneTransactionAfterFetch`, `TestSearchNozomiPartsConcurrentFetchDedupsToSingleCall`, `TestServeStoredImageStreamsConditionalResponses`, `TestSettingsEndpointServesEditableSettingsWithoutAPIKey`, `TestSettingsResponseServesDBRowCountsForCacheClearPanel`, `TestTNWriteRecordsThumbsRowAndPrunesThumbsKindOnly`, `TestThumbProxyTNWriteFailureLeavesNoBrokenTNCache`, `TestViewerConfigEndpointServesEffectiveMaxObjectURLs`, `TestViewerGalleryFilesIncludeLocalAndHitomiFlags`, `TestViewerImageProxyDoesNotCacheNetworkErrorAsNegativeNoFull`, `TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing`, `TestViewerImageProxyDoesNotFetchAvifTNVariantForImage`, `TestViewerImageProxyDoesNotFetchTNVariantForImage`, `TestViewerImageProxyDoesNotRetryWhenGGBucketUnchanged`, `TestViewerImageProxyFetchDedupDoesNotPropagateWaiterCancellationToLeader`, `TestViewerImageProxyFetchDedupsConcurrentRemoteFetch`, `TestViewerImageProxyForceBypassesNegativeNoFullCache`, `TestViewerImageProxyNegativeNoFullCacheReducesRepeatStorageFetches`, `TestViewerImageProxyNegativeNoFullKeyChangesWhenGGChanges`, `TestViewerImageProxyRefreshesGGOnceOnStorage404`, `TestViewerImageProxyRetriesUsingInitialGGWhenAnotherRequestUpdatedGG`, `TestViewerImageProxyReturnsNoFullWhenStorage404AndDoesNotPersistTN`, `TestViewerImageProxyServesSameOriginWebp`, `TestViewerImageProxyWritesPersistentCacheAndServesCacheHit`, `TestViewerIndexServesEmbeddedPage`, `TestViewerInfoCacheConcurrentHandlersDoNotWriteUnprotectedMap`, `TestViewerPageSourceLocalFirst`, `TestViewerPageSourceUsesHasavifFormatWhenRemote`, `TestViewerReadFavoriteAndQueryHandlers`, `TestViewerSearchCardHandlerReturnsResult`, `TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen`, `TestViewerSearchHandlerReturnsIdsOnlyForLazyCards`, `TestViewerThumbProxyConcurrentTNFetchDedupsToSingleCall`, `TestViewerThumbProxyPersistsTNThumbnailCacheAndServesCacheHit`, `TestViewerThumbProxyTNFailureDoesNotUseLocalOrFullFallback`, `TestViewerThumbProxyTNFailureHeadersDoNotTreatNoTNAsFull`, `TestViewerThumbProxyTNFirstWhenGalleryQueryAndLocalExists`, `TestViewerThumbProxyTNOnlyDoesNotServeFullCache`, `TestViewerThumbProxyUsesAvifTNBaseAndContent`, `TestViewerThumbProxyUsesRealTNPathAndContent`, `TestViewerThumbProxyUsesSmalltnVariantWhenRequested`, `TestViewerThumbProxyUsesTNZeroSubdomainAndContent`.

### 推定規模・優先順位

- 実装規模: 小（非 POST GET の loopback Host 検査を `withPOSTSecurity` に 1 分岐追加。POST は existing `sameOriginPost` 維持）。
- テスト re-anchor 規模: 中〜大（現行 test-level 54 件、line-level GET usage 85 件）。前批 5 件は line-level の一部であり、現行の actual relevant set は 54 test-level。
- 優先順位要素: 重要度は security・local origin 境界として中〜高、実装リスク low、テスト修正コスト medium。UI・DB・full cache の x-cache/generation guard は middleware 追加で弱まらない。LAN allowlist 拡張は正典未承認で承認案件として分離。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
