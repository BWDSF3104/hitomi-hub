# Manual Japanese search button minimal impact implementation plan

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: Implementation-ready plan recorded; exact commands and blockers recorded in task file.

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

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
