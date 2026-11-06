# Manual Japanese search button minimal impact implementation plan

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 単独コミット・push（実施済みでクローズ）

## 意図 / 背景

Manual Japanese search buttonを既存の prefetch・TN・full image path・card height contractに最小影響で導入する。目的は、non-Japanese cardに「日本版を検索」buttonを置き、click時の foreground data requestだけを追加する形にする。

前提条件:
- group既知で Japanese bestId がある card は existing `sameTitleJapaneseAnchor` を変えない。
- TN prefetch は link機能の必要要素ではなく、button clickでTN requestを増やさない。
- hitomi plain term searchは exact normalized term searchである。fuzzy title searchにしない。

## 調査結果 / 変更内容

Recommended minimal impact implementation plan:

1. UI render rule
   - `renderSearchCard` で non-Japanese cardかつ `titleGroups` に Japanese bestId が未確定のときだけ button を existing `.text` / `.meta` area に inline表示する。
   - separate rowを増やさない。existing `.meta` rowまたは existing anchor equivalent row内 controlとして表示する。
   - group既知の Japanese link表示は current `sameTitleJapaneseAnchor` のまま。

2. Helper boundary
   - `normalizeTitleKey` を変更しない。既存 group key semanticsを維持する。
   - search query用 helper `manualJapaneseSearchTerm(card)` を追加する。
   - query term は `card.japanese_title` があれば优先、なければ `card.title`。
   - hitomi canonical exact term形式として lowercase + `_` → space を mirrorする。既存 group key helperとは分離する。
   - title に colon が含まれる場合、query parserの field term誤読リスクがあるため、live testで確認できるまで button clickを unsupported messageとして扱う。

3. Click behavior
   - click handlerは `searchGeneration` で守る。stale generationでは anchor更新しない。
   - click時に `titleGroups` に既知 Japanese groupがあれば、buttonを linkへ置き換え、fetchしない。
   - `prefetchCooldownUntil` が activeな間は button clickを data fetchへ進めない。foreground clickでも cooldownを尊重する。
   - same normalized keyの duplicate clickは in-flight mapで止め、button busy stateにする。

4. Request path
   - queryは `/api/search?query=<canonical exact term>+language:japanese&lang=all&limit=1` にする。
   - `lang=japanese` parameterだけにはしない。plain termでは canonical language conditionにならない。
   - top idが取れたら `/api/search-card/<id>` を foreground card JSONとして取る。
   - TN requestを追加しない。`searchCardCoverSource`、TN lane、prefetch TN queueに変更しない。

5. Result application
   - result cardが `language == japanese` かつ errorがない場合だけ `setSearchTitleGroup(resultCard)` を呼ぶ。
   - clicked cardだけ anchor/control更新する。既存 card全体 re-renderはしない。
   - v1では same normalized keyの他の cardへ immediate DOM更新しない。必要なら later optionで追加する。

6. Button states
   - `idle`: button enabled。
   - `busy`: click後 fetch中、disabled。
   - `error`: fetch failure、error message。block/cooldownは existing `PREFETCH_COOLDOWN_MS` を尊重。
   - `none`: exact term no result、result non-Japanese、colon title unsupported。
   - `found`: existing Japanese anchor表示、button disabledまたは removal。

7. Non-contact areas
   - `PREFETCH_AHEAD_PAGES`、`PREFETCH_CONCURRENCY`、`PREFETCH_IDLE_MS`、`prefetchQueue` を変更しない。
   - `loadSearchCards` worker orderを変えない。
   - `searchCardCoverSource`、TN display fallback prohibition、full image path、`imageProxy`、objectURL display pathを変えない。
   - saved query、lists、read state、favorite、download pathを変えない。

8. Exact verification commands
   - `node tests/search_prefetch_harness.mjs`: existing labels must remain `PREFETCH_AHEAD_PAGES = 2`, `PREFETCH_CONCURRENCY = 4`, `PREFETCH_IDLE_MS = 800`, `PREFETCH_COOLDOWN_MS = 300000`; manual helper must preserve no-TN path and stale-generation guard.
   - `python -m pytest tests/ -v`: must pass existing canonical freshness tests and new tests for query builder, stale generation, duplicate in-flight, no TN request, no saved query change, colon-title unsupported behavior.
   - `go vet . && go test -count=1 -timeout 300s .` in `app`: must pass because UI changes require served canonical confirmation.
   - Canonical build:
     `go build -o "S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe" .`
   - Canonical served UI/network confirmation is required. Source `ui.html` string tests alone are not enough.
   - Card-height confirmation: inline button must not exceed existing card height contract. Screenshot evidence required.
   - Live exact-term confirmation: sample real work titles must prove `manualJapaneseSearchTerm(card)` + `language:japanese` returns a Japanese result. Colon-title behavior must be confirmed or explicitly unsupported.

## Important implementation facts for new session

- Current task file: `docs/ai/tasks/2026-11-06-manual-japanese-search-button-minimal-impact.md`
- Plan commit: `3fca8ad`
- Do not change `normalizeTitleKey`; it is the existing title-group key.
- New query helper must be separate and must mirror hitomi canonical exact term normalize: lowercase and `_` to space.
- Query must be `query=<canonical exact term>+language:japanese`, not only `lang=japanese`.
- Manual button path must fetch only `/api/search` and `/api/search-card/<id>`.
- TN prefetch is not required for the link feature and must not be added in this path.
- `PREFETCH_AHEAD_PAGES`, `PREFETCH_CONCURRENCY`, `PREFETCH_IDLE_MS`, `PREFETCH_COOLDOWN_MS`, `prefetchQueue`, TN display fallback prohibition, full image path, `imageProxy`, objectURL display, saved query, lists, read state, favorite, and download path are out of scope.
- Implementation start requires explicit user instruction.

## 実装確定（coding 前に確定・実装開始指示は implement-items goal objective 2026-11-06 07:32 UTC で受領済み）

- 作業前チェック（AGENTS.md 参照・viewer/download 接触タスクの読了明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を読了。TN display fallback 禁止・TN を reader 表示源にしない・TN バイトを full cache に保存しない・full 判定を弱めない・永続先を scratch/output へ移さない は非接触（本項目は TN request を追加しない）。
- 画面確認基準の正典（ポインタ参照先）: `docs/ai/rules/normal-screenshot-rule.md`（Playwright browser 画面を被写体・OS foreground 不要）、`docs/ai/rules/screen-confirmation-size-rule.md`（actual window size 基準・固定 target size 不合格化しない）。Canonical build / UI confirmation rule の参照先 `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`（search.html served 画面確認必須）と `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`（正典 build command）を適用。cold 確認 hygiene（`docs/ai/rules/cold-confirmation-hygiene-rule.md`）: fresh canonical hub.exe 起動 server・fresh server instance・isolated origin で測定。
- row 追加なしの実装形: button は existing `.meta` row（language/type/files row）の inline control として renderSearchCard の meta string 内へ追加（separate div row を増やさない）。found 時は same row 内 button を link へ置換、message span も same row 内 inline。これにより existing card height contract（`--search-card-height: 130px`）は li 固定 height のまま。button の min-height は existing badge button と同一基準（24px）の inline rule を追加。
- helper 配置: `manualJapaneseSearchTerm` / `manualSearchQueryUrl` / `manualSearchButtonHTML` / click handler は `sameTitleJapaneseAnchor` 直後（`badgeFlag` 前）に置く。既存 `tests/search_prefetch_harness.mjs` の `patchHelperBlock`（applyPrefetchCardKnowledge〜PREFETCH_IDLE_MS）と `schedulerBlock`（PREFETCH_IDLE_MS〜search form addEventListener）の境界外であり、両 block の innerHTML / setStatus / renderSearchCard 排除 assertion を壊さない。renderSearchCard の `sameTitleJapaneseAnchor(r)` と `anchor + ` は Go test（viewer_test.go:2618, 3343）が要求するため維持。
- query builder: `manualSearchQueryUrl(term)` = `/api/search?query=` + encodeURIComponent(term + " language:japanese") + "&lang=all&limit=1"（`lang=japanese` 依存禁止）。term は `card.japanese_title` 优先 → `card.title`、lowercase + `_` → space（`normalizeTitleKey` は非接触）。colon を含む term は click 時 unsupported message（fetch しない）。live で colon 挙動確認後、unsupported を明示的に維持（plan step 2・8）。
- click handler: `searchGeneration` guard（click 時 gen 記録、fetch 応答後 stale なら anchor 更新しない）、既知 Japanese group は fetch せず button を link へ置換、`prefetchCooldownUntil` active 中は fetch しない（error state）、same normalized key の duplicate click は `manualSearchInFlightKeys`（in-flight map）で停止し busy state。result application は `language == japanese` かつ error なし時だけ `setSearchTitleGroup(resultCard)`、clicked card の row だけ更新（card 全体 re-render なし、same key の他 card への immediate DOM 更新なし）。

## Verification

### コマンド結果（合格）

- `node tests/search_prefetch_harness.mjs` → 42 checks passed（既存 labels・`PREFETCH_AHEAD_PAGES = 2`・`PREFETCH_CONCURRENCY = 4`・`PREFETCH_IDLE_MS = 800`・`PREFETCH_COOLDOWN_MS = 300000` 維持）
- `node tests/search_manual_jp_harness.mjs`（本項目の新 harness）→ 20 checks passed（query builder・stale generation・duplicate in-flight・no TN request・no saved query change・colon-title unsupported・busy/idle/found/error states・cooldown 尊重）
- `python -m pytest tests/ -v` → 55 passed（既存 canonical freshness + 新 `tests/test_manual_japanese_search_button.py` 2 test）
- `app` で `go vet . && go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 24.279s`
- Canonical build `go build -o "S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe" .` → 成功、canonical freshness test 合格

### Canonical served UI/network 確認（source string test では不合格基準のため実機）

- Fresh canonical `hub.exe`・fresh server instance（port 8732・runtime root `output\hitomi-hub`）・isolated origin（localStorage clear 後計測）。Playwright browser 画面基準（normal-screenshot-rule）。actual window size 1280x900・content viewport 1280x900。
- 実 search `furry` page 1 → 25 cards・group 未確定 non-Japanese card 14 件に inline button が existing `.meta` row 内表示（new row なし・button min-height 24px・card 内完全表示）。
- Click #4308110 term `witch doctor`: network delta = exactly `/api/search?query=witch%20doctor%20language%3Ajapanese&lang=all&limit=1` (200) + `/api/search-card/3888535` (200)。TN request なし。result card `language == japanese` → `setSearchTitleGroup` + same row の button が link へ置換（`同名の日本語版: #4075289` 相当の same-row link `#3888535`）。
- Click #4308265 term `femboy favs.`: network delta = `/api/search?query=femboy%20favs.%20language%3Ajapanese&lang=all&limit=1` (200) + `/api/search-card/4075289` (200)。TN なし・post なし・saved query 変更なし（query input 値不変更）。same row button→link（`同名の日本語版: #4075289`）。
- Card height confirmation: `.card` computed height = `130px`（contract 維持）。li height 141px（既存 padding 込み）は button あり/なし card で同一。button は card bounds 内（top/bottom 内・24px）。separate row 追加なし。
- Live exact-term confirmation（実在 gallery・sample real work titles）: `witch doctor`（#4308110 english）→ Japanese result #3888535、`femboy favs.`（#4308265 english）→ Japanese result #4075289。exact term + `language:japanese` で Japanese result 返ることを実測。
- Colon title: live sample search（furry・female:yuri page 1）に colon 入り title なし。明示的に unsupported として扱っており（click で message・fetch なし・harness test 合格）。parser 挙動実測: exact `witch doctor` → 200 ids [4308110]、colon `witch:doctor` → 502 `search: status 404 for https://ltn.gold-usergeneratedcontent.net/n/witch/doctor-all.nozomi`（field term dataset へ誤読）→ unsupported 維持の根拠を実測で確定。

### スクリーンショット（Drive 保存）

- 保存先: `I:\マイドライブ\kilo\2026-11-06-manual-japanese-search-button\`
- `manual-jp-card-height-20261106.png`: inline button が existing `.meta` row 内（inline control）で表示され、card height contract（130px・li 141px）保たれ button が card 内完全表示であることの証拠（window size 1280x900）
- `manual-jp-found-card-20261106.png`: clicked card の same `.meta` row 内 button→link 置換（found state・clicked card のみ更新）の証拠

## 採用した判断（段階1）

- button は existing `.meta` row（language/type row）の inline control（plan step 1 の「existing .meta row 内 control」の推奨案どおり・実測で new row なし・height contract 維持を確認）。
- colon title は明示的に unsupported（plan step 2・8 の推奨案）。live 実測で field term 誤読を確認済み（上記）。fuzzy search 化はしない（非接触）。
- `normalizeTitleKey`・prefetch constants・TN lane・saved query・download path 非接触（変更なし）。

## 追加確認（non-Japanese card からの manual click・親実測 2026-11-06 08:05 UTC）

- Fresh canonical `hub.exe`（commit `5d87f4c` からの canonical build）・fresh server instance（port 8733・runtime root `output\hitomi-hub`）・isolated origin（localStorage clear 後で測定）・actual window/viewport 1280x900（dpr 1）。Playwright browser 画面基準。
- search `furry` page 1 → group 未確定 non-Japanese card 14 件に button が existing `.meta` row 内表示。
- click 1（english card #4308110 `Witch Doctor`）→ click 後の network delta 2件のみ：`/api/search?query=witch%20doctor%20language%3Ajapanese&lang=all&limit=1` 200、`/api/search-card/3888535` 200。click 後の `/api/thumb` 発生 0（TN request なし）。same `.meta` row の button → link `同名の日本語版: #3888535`、card computed height `130px`、cover src（bigtn）不変。
- click 2（chinese card #4308228 `Ao no Sekai to Hanasaku Karada`）→ click 後の network delta 2件のみ：`/api/search?query=ao%20no%20sekai%20to%20hanasaku%20karada%20language%3Ajapanese&lang=all&limit=1` 200、`/api/search-card/3710954` 200。same row button → link `同名の日本語版: #3710954`、height `130px`。
- japanese card（#4306976）は button なし（non-Japanese card 限定の rule どおり）。
- スクリーンショット（Playwright browser 画面・normal-screenshot-rule）: `I:\マイドライブ\kilo\2026-11-06-manual-japanese-search-button\manual-jp-nonjapanese-check-20261106.png`（local copy `scratch/manual_jp_nonjapanese_check_20261106.png`）
- 画面で未確認（harness 確認のみ・実サンプルなし）: busy state・error state・colon-title unsupported message の表示。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（task_manager end で実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
