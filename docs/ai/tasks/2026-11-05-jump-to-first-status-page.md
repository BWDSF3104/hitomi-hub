# 現在の検索条件で表示済み等のステータス付き作品が最初に見つかるページへ移動する機能の検討

**Status**: 完了
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

- app 変更後: `go vet .` → clean（1.36s）・`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 18.335s`（2026-11-05 実行・合計約21.3s）。既存 `searchhtml_test.go` は変更なしで合格
- 正典 build 実機 UI 確認（rule pointer `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`・normal screenshot rule）。正典 exe 再build（`go build`→`output\hitomi-hub\hub.exe`）後の canonical server（runtime root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage`・port 8731・PID 10228）で Playwright browser tool（直接 target 使用・browser executable launch なし・OS foreground 不要）で確認。実測 window size 1920x1040・content viewport inner 1920x953・content `.main` 1377x844（固定 size 不使用）
- 確認シナリオ: query `female:yuri language:polish`（total 59・page 2 = 25 items）。initial load の runSearch（network #6）→ idle prefetch page 2（#49 background dataset + #50-74 background cards）→ 以降 jump click（実操作）
  1. 該当なし: status 絞り込みなし・未マーク時 click → `GET /api/search-first-status`（#82）hit null → status 行「該当なし」、page input 1 のまま・hash 変更なし・追加 fetch なし。スクリーンショット `2026-11-05-jump-first-status-page-1-nomatch.png`
  2. jump 移動 + highlight: 準備として rank 25（page 2 先頭）の gallery `2299308` を既存 `POST /api/viewed`（正典 write path・runtime scratch DB）で表示済み化 → click → status 行「jump: page 2・rank 25・#2299308・表示済み」、page input 2、hash `#/q/female%3Ayuri%20language%3Apolish/page/2`（既存 hashchange → runSearch 経路）、hit card `li[data-search-id='2299308']` class `card has-status highlight` + scrollIntoView（top 93px・viewport 内）、badge「表示済み」は既存 card 応答 maps。スクリーンショット `2026-11-05-jump-first-status-page-2-jump-highlight.png`
  3. prefetch 済み page の重複 fetch なし（cache hit）: jump 後の dataset #85 `x-search-timing: search=0`（cache hit・remote refetch なし）、card #86-116 は prefetch 済み ids の再 request が `x-cache: cache`（perf 表示 `cache cache=28 miss=22`・2 round 目 full cache hit）、target=current click（#118）は status API のみで card/dataset fetch 増なし。JSON 形状 API 正典と一致: `{"hit":{"flags":{"favorite":0,"read":0,"saved":0,"viewed":1},"gallery_id":2299308,"page":2,"rank":25},"lang":"all","query":"female:yuri language:polish","status_mask":15,"total":59}`
  4. random disabled: query 入力欄に `female:yuri language:polish orderby:random` を実入力（input listener → `updateJumpButton`）→ button `disabled=true`、network 追加なし。スクリーンショット `2026-11-05-jump-first-status-page-3-random-disabled.png`
- スクリーンショット3枚は Playwright browser 画面画像で `I:\マイルドライブ\kilo` 保存（filename に task ID 含む）。OS full-screen 例外は不使用（Playwright browser 画面で確認可）

### スクリーンショット保存のタスクフォルダ化（ユーザー指示 2026-11-05 08:54）

- 保存先をタスクフォルダへ変更: `I:\マイルドライブ\kilo\2026-11-05-jump-to-first-status-page\`（既存3枚を同名で移動）
- 項目2は操作前後の対が必要（ユーザー指示）。正典 build 再確認（canonical `go build` flags 再build・runtime root `Common_Operations\scratch\hitomi-usage`・port 8731・server PID 30676。viewed 状態 `#2299308` は既存 record のまま。API hit 確認: page 2・rank 25・status_mask 15・total 59。人工 resize なし・actual window size 1920x1040・content viewport inner 1920x953（前回と一致））
  - 操作前: `2-jump-before.png`（page 1・status「検索完了・カード表示済み 25件」・jump button enabled・25 cards）
  - 操作後: `2-jump-after.png`（click 後 page 2・hit card `card has-status highlight`・top 93px・badge「表示済み」。click 直後の status は「jump: page 2・rank 25・#2299308・表示済み」→ runSearch 完了で「検索完了」へ更新（既存 shared status 挙動））
- フォルダ内: `2-jump-before.png` / `2-jump-after.png`（新規対）＋既存 `2026-11-05-jump-first-status-page-1-nomatch.png` / `-2-jump-highlight.png`（単発・対導入前の記録として保持） / `-3-random-disabled.png`

### 先取り範囲外での動作確認と時間（ユーザー指示 2026-11-05 09:07）

- idle prefetch の範囲は次の 1 page のみ（`search.html` runPrefetch）。範囲外 hit で確認: query `female:yuri language:polish`（total 59）。テスト準備（scratch runtime DB・既存正典 write path）: `#2299308` を reset、rank 50（page 3 先頭）の `#296403` を viewed=1 → hit は page 3（prefetch 済み page 2 の範囲外）
- 動作: page 1 表示・page 2 prefetch 完了（background search + 25 cards）後に click → status 行「jump: page 3・rank 50・#296403・表示済み」→ page 3 へ移動・hit card `card highlight`（top 93px）✓。判定は `read_state`（DB）のみで成功（viewed 判定に galleryinfo fetch 不要）
- 時間（client 実測・performance.now）:
  - status API 単発 fetch: **3ms**（dataset cache hit・DB 1 query）
  - click → status 行表示: **66ms**
  - click → page 3 全 card（9件）表示完了: **22.7s**（server cache hit でも data call gate 約1s/request）
  - 参考: 初回 page 3 進入（card 未 cache）の full display は **48.7s**
- 留意（既知問題化）: page 3 の旧形式 gallery は live galleryinfo が数値フィールド（id・tags 値）で Go parse 失敗 → card error・badge 表示不可（hit card 本文は error 表示）。`docs/ai/known-issues/KI-20261105-06-old-format-galleryinfo-numeric-parse-fail.md`。移動・highlight・status 行の合格条件には影響なし
- スクリーンショット（タスクフォルダ・Playwright browser 画面・window 1920x1040/inner 1920x953）: `4-out-of-range-before.png`（操作前・page 1） / `4-out-of-range-after.png`（操作後・page 3・highlight・status 行 jump 表示）

### 同条件再確認（ユーザー指示 2026-11-05 09:50・KI-20261105-06 修復後）

- 条件: 正典 build（`test_canonical_build_freshness.py` 2 passed）・runtime root `Common_Operations\scratch\hitomi-usage`・port 8731（server PID 13284）。正典 write path で `#2299308` reset・`#296403` viewed=1（saved 0・項目原文の条件一致）。API hit 確認: page 3・rank 50・`#296403`・viewed・status_mask 15・total 59。window outer 1920x1040・inner（Playwright viewport）1920x1040。
- 手順: page 1 submit（runSearch・カード25件表示済み）→ idle prefetch page 2 完了（background dataset+cards+cover 102 requests 確認）→ jump click（MutationObserver で status 行捕捉）。
- 結果: status 行 `jump: page 3・rank 50・#296403・表示済み` ✓（項目原文の記録と一致）。page 3 へ移動・hit card class `card has-status highlight`・top 93px（問題発生時と同一）。**card 本文 error 表示なし・badge「表示済み」+ reset 表示**（KI-20261105-06 修復 commit `8d9ead1` により問題発生時の症状「error 本文・badge 空」が解消。page 3 の9件すべて error 0・badge card 1）。tags 出力 `female:futanari, female:schoolgirl uniform, female:yuri`（`names()` の意味不変確認）。
- 時間（client 実測・performance.now・条件差: server 再起動後・disk cache あり）: click → status 行表示 **3ms**（問題発生時 66ms）。click → page 3 全 card（9件）表示完了 **14,337ms**（問題発生時 22.7s・TN cover 34 miss・perf row `cache=43・miss=34`）。
- スクリーンショット（Playwright browser 画面・タスクフォルダ保存）: `4-out-of-range-recheck-before.png`（操作前・page 1） / `4-out-of-range-recheck-jumpstatus.png`（click 後・status 行 jump 表示を observer で捕捉 `jump: page 3・rank 50・#296403・表示済み`・hit card highlight） / `4-out-of-range-recheck-after.png`（9件表示完了・badge 表示）。保存先: `I:\マイドライブ\kilo\2026-11-05-jump-to-first-status-page\`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
