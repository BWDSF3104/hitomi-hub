# 現在の検索条件で表示済み等のステータス付き作品が最初に見つかるページへ移動する機能の検討

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: 子へ委譲（Phase 1-4 実装・検証）

## 意図 / 背景

現在の検索条件（term・lang・orderby を含む結果順）で、表示済み・既読・保存済み・お気に入りなどのステータス badge が付いている作品が「最初に見つかるページ」へ直接移動する機能の検討。現状の UI には prev/next と page 入力しかなく、ステータス付き作品の位置を知るためにページを1枚ずつ進める必要がある。設計検討のみ（実装開始指示未受領）。

## 調査結果 / 変更内容

- `SearchHTML`（`app/searchhtml.go:275-439`）は内部で全 ordered ID list を作り、`searchPageSlice`（`app/searchhtml.go:252-269`）で 1 page 分（`galleriesPerPage = 25`・`app/hitomi.go:21`）だけ返す。full list を外部へ渡す口がない → `searchAllIDs` 相当の refactor が必要（SearchHTML = searchAllIDs + slice）。
- ステータス源:
  - `read_state`（`viewed` / `fully_read` / `favorite`）— `app/library.go`。DB だけで判定可。
  - `saved` は card API（`app/viewer.go:653-655`）で galleryinfo の `Files` 全 entry が local に揃うこと。純 DB join では確定できない（部分保存は badge に出さない）。badge の4状態定義は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md`（表示済み/既読/保存済み/お気に入り）。
- UI 遷移は hash route `#/q/<query>/page/<N>`（`app/search.html:251-256`）と page input。jump 機能・highlight は現状なし。card は `data-search-id`・badge は card 応答由来。
- dataset order は cache TTL 内で安定。`orderbydir=random` は shuffle が呼出ごとに変わるため「最初のページ」が再現性なし（設計判断が必要）。

### 設計案（推奨）

1. server 側: `GET /api/search-first-status?query=...&lang=...`（flag で status mask）。手順:
   1. full ordered IDs を取得（既存 searchCache の dataset fetch を再利用。追加 network なし）
   2. DB で candidate set を1 query で構築（read_state flags + `files` の `gallery_id` 由来 saved）
   3. rank 0 から順に walk し、最初の一 hit について saved を card API と同一の strict 判定で確認（galleryinfo cache 参照・lazy）。不一致なら次の candidate へ
   4. `page = rank / 25 + 1` と hit の gallery_id・total を返す
2. UI: page-nav に「最初のステータス付きページへ」button。route を該当 page へ変更し、hit card を highlight（badge は card 応答で既存表示）。
3. CLI: `search` に `--first-status` 追加、JSON で page と hit id を出力。
- コスト: dataset fetch 1 回（SearchHTML と同一）+ DB query 1 + walk O(n)。典型は hit が上位 rank。全 miss なら「なし」報告。

### 既存 idle prefetch との統合（ユーザー指摘 2026-11-05）

既存の先取り: `app/search.html:551-642` の Phase 2 idle prefetch。現 page の card 完了後、次の 1 page だけ `priority=background` で `/api/search` + card + TN cover を取得。interrupt/cooldown（429/5xx で 5 分）・generation guard・route 遷移では `prefetchEligible=false`。

統合条件:

1. **単一 cache 経路**: `searchAllIDs` は既存 `searchCache`（key `area|tag|lang`・TTL 1h・`app/hitomi.go:47`）をそのまま使う。SearchHTML は元々 dataset 全体を cache してから slice するため、prefetch の page fetch と jump は同一 cacheKey を共有する。jump が先攻すれば以降の page めくりが cache hit になり、jump 専用に再 fetch しない（remote 取得を無駄にしない）。`copyInts` で cache は immutable（rand shuffle は copy のみ）→ 複数 caller 共有で順序が壊れない性質を維持。
2. **lane**: jump はユーザー click 起点の foreground（marker なし・metrics kind 分離は無変更）。click 時に既存 `interruptPrefetch()` を呼んで走行中 prefetch と競合させない。jump 後 idle prefetch は target+1 から通常どおり継続。
3. **DB 書込み**: jump endpoint は read-only。記録は target page 描画時の既存 `recordDiscoveredBatch`（`app/viewer.go:565`）のみで、未表示 candidate を書かない。
4. **saved strict 判定の cache 共有**: `v.info(id)`（infoCache・persistent file store）経由。card 描画と同じ源なので fetch を増やさない・捨てない。判定は walk 中の hit candidate について lazy に 1〜2 件分だけ。
5. **UI maps**: jump 後の card 描画は既存 route 経路（hashchange → runSearch）を通す。`prefetchEligible=false`（route 由来）の既存挙動どおり、jump 単独で idle prefetch を走らせない。

### 未確定の設計判断（ユーザー確認必要）

1. ステータスの範囲: **確定 2026-11-05**: 4状態のいずれか（既定）+ 絞り込み可（単一/複数 flag 選択）。UI は「いずれか」+ checkbox、CLI は `--status` 値
2. `saved` の定義 → **別タスク化 2026-11-05**: `docs/ai/tasks/2026-11-05-saved-status-definition.md`（後で定義が必要）。本件は現行正典定義（`docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項）をそのまま使用し、算出は既存源 `v.info`（近似は表示不一致のため不採用）。本件の判断要件ではない
3. ~~`orderbydir=random` の扱い~~ → **確定 2026-11-05: 無効**（`docs/ai/decisions/2026-11-05-jump-first-status-page-random-order-disabled.md`。UI は button disabled、CLI は error）
4. hit card の highlight: **確定 2026-11-05**: 付与（`.card.highlight` + scrollIntoView）
5. 入口の範囲: **確定 2026-11-05**: UI + CLI（UI button・CLI `search --first-status`）
6. 全 page に hit が無い場合: current page にとどまり「該当なし」を status 行へ表示（default 案・承認不要なら確定扱い）

default 案として追加確認済みの挙動（個別承認不要ならこのまま):
- target page = current page のとき移動せず hit card だけ highlight
- jump endpoint は read-only（DB 記録は target page 描画時の既存 `recordDiscoveredBatch` のみ）
- 判定は cache TTL（1h）内の同一 dataset list を使うため total 表示と page 番号の一致は保証される。TTL 切れで refetch した場合の total drift は既存 search/prefetch と同一の既存挙動（新問題なし）

## 実装計画（確定 2026-11-05・実装開始指示未受領）

作業前チェック（reading 記録・タスクが search/viewer に触れるため）:
- 読了済み decision: `2026-11-04-search-viewer-separation-approach-approved.md`（正典 build/UI 確認 rule の pointer）・`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`・`2026-11-05-search-result-state-badges-approved.md`

### Phase 1 — refactor（挙動無変更）

- `SearchHTML` から term parse/state 構築（`buildSearchState`）と full ordered list 生成（`searchAllIDs`）を抽出。`SearchHTML` = `searchAllIDs` + `searchPageSlice`。`searchCache` 単一路径・`copyInts` の cache immutable を維持
- 既存 `searchhtml_test.go` を変更なしで合格（refactor で test anchor を動かさない）

### Phase 2 — 計算核（新 file `app/firststatus.go`）

- 入力: query・lang・status mask（既定=4状態いずれか・単一/複数選択可）
- `orderbydir=random` → error `random order is not supported`
- full ordered IDs を `searchAllIDs` から（既存 cache 共有・追加 fetch なし）
- candidate set を 1 query で: `read_state` の flag（mask）+ `files` の `gallery_id` 存在（saved pre-filter）
- rank 順 walk: hit candidate について saved を現行正典定義どおり strict 判定（`v.info` + `entryLocalFormats` ループ・lazy）。pre-filter の限界: 未 scan の filesystem 単独ファイルは candidate に出ない（app 自身の download は scan 経路を通るため実務上問題なし・限界として記録）
- `page = rank / galleriesPerPage + 1`。返す値: page・rank・hit gallery_id・hit flags・total。hit なしは null
- endpoint `GET /api/search-first-status`（foreground・read-only）。CLI: `search --first-status`（`--status` 値）で同一 JSON 形状

### Phase 3 — UI（`app/search.html`）

- page-nav へ「最初のステータス付きページへ」button。query 解析で random order 有効のとき disabled
- 検索 form へ 4 status checkbox（既定=いずれか・選択時絞り込み）
- click: `interruptPrefetch()` → endpoint 呼出 → hit なら page input 更新 + hash route（既存 `runSearch` 経路）。target = current page のとき移動せず highlight のみ
- hit card: `.card.highlight`（card height contract 無変更・badge 行は既存）+ scrollIntoView。badge 表示は既存 card 応答 maps
- hit なし: status 行へ「該当なし」

### Phase 4 — 検証・実機確認

- app: `go vet . && go test -count=1 -timeout 300s .`
- 新 test（`firststatus_test.go`）: rank→page 換算、mask  membership、saved strict/partial 区別、random error、hit なし null、cache 共有（2度目の呼出で fetch 増えない）、test 方針は外部 API 呼出し可（`docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`）
- 正典 build 実機 UI 確認（rule pointer 2026-11-04・normal screenshot rule）: jump 移動・highlight・random disabled・該当なし表示・network list で prefetch 済み page の重複 fetch なし（cache hit）を報告

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
