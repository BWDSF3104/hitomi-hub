# Series/Type 用専用 UI フィルタ検討

**Status**: 完了
**Date**: 2026-11-08
**Next Action**: 追加確認（固定 Type filter 値）の実装・served UI 確認

## 意図 / 背景

- Series/Type で絞り込みを、検索 term だけでなく専用 UI フィルタとして追加する案を検討する。
- 既定は絞り込みなし。
- Type は複数指定可能とする。
- 実装開始指示前は設計案提示までとし、UI コード変更・正典 build・served UI 確認は行わない。

## 調査結果 / 変更内容

- 現行 UI は `query` / `lang` / `page` / status checkbox のみ。Series/Type 専用 control はなし。
- 検索は `SearchHTML()` の term routing。colon term の left が namespace、right が tag。
- `series:` / `type:` は nozomi dataset へ行き、positive term は intersection。
- 現在の positive term は AND 相当。`or` group は群内 union、他の positive と intersection。
- Type は gallery に通常 1 個しかないので、複数 Type 指定は `OR` にすべき。
- 推奨 UI 動作:
  - Series 既定: 絞り込みなし。値があるときだけ `series:<term>` を追加。
  - Type 既定: 絞り込みなし。複数 checkbox / multiselect で選択可能。
  - Type 複数選択時は `type:a or type:b` として追加。既存 query とは「既存 term AND (Type OR ... )」になる。
- State/route は persisted filters を保持し、route hash に filter state を入れるか、effective query に反映するかを検討。
- UI string tests で compose helper を固定。UI changes は canonical build + served UI confirmation required.

## 実装項目 UFI-1

- `app/search.html` に Series/Type dedicated filter controls を追加。
- Default filter: none.
- Type multiple selection: OR group.
- Effective query composer behavior:
  - raw base query が空で filter も空なら raw query をそのまま使う。
  - filter active 時、raw base positive terms と filter group を `and` boundary で AND する。
  - Series filter が active なら `series:<term>` を追加。multi-word input は term 内 `_` を使う。
  - Type filter が複数選択なら `type:a or type:b` 形式の OR group を追加。
- `app/searchhtml.go` の `buildSearchState()` で、UI-generated query boundary として `and` を group separator に扱う。
- Filter selection は localStorage で persisted し、local restore で controls を復元する。Shared hash only では filter controls が復元されない可能性は既知制限として扱う。

承認範囲:
- `app/search.html`
- `app/searchhtml.go`
- relevant test files only
- this task file only

合格条件:
- UI string tests for controls default, compose helper, multiple Type OR group, local filter state restore。
- Go tests for `and` group boundary: raw positive term AND filter group, raw OR group AND filter group。
- Existing tests unaffected: `go vet . && go test -count=1 -timeout 300s .` in `app`。
- Canonical build + served UI confirmation per `docs/ai/rules/confirmation-test-rule.md`。
- Served UI network response must show `/api/search?query=...` expected effective query。
- UI screenshot saved to Google Drive per existing confirmation rules and recorded in task file。

## 判定

- UFI-1 は未解決。理由: `python -m pytest tests/ -v` full pass is blocked by out-of-scope `AGENTS.md` canonical pointer known failure; known failure is stage 3 and requires user approval.
- UFI-1 implementation/test changes have been undone to the delegation-before implementation state. No commit exists.

## Verification

- UFI-1 implementation evidence produced during attempt (files were undone to delegation-before state after the failure re-delegation):
  - `app/search.html`: dedicated `#series-filter` and `.series-type-filter-row` Type checkbox group; helper block with `FILTER_STORAGE_KEY`, `TYPE_FILTER_OPTIONS`, `lastSearchQuery`, `currentFilterState()`, `composeFilterGroup()`, `composeEffectiveQuery()`, `loadFilterState()`, `saveFilterState()`, `restoreFilterControls()`; `runSearch()` composes effective query and persists rawQuery/filter; `runPrefetch()` uses `lastSearchQuery` through guard; `restoreSearchState()` restores filter controls; filter input changes interrupt idle prefetch; `checkedStatusNames()` reads only `#search input[data-status-filter]`.
  - `app/searchhtml_test.go`: added Go tests for `and` as UI-generated effective query group separator.
  - `app/viewer_test.go`: updated existing assertion to the new `runSearch()` save behavior for raw query and filter persistence.
  - `tests/search_ui_filter_harness.mjs` and `tests/test_search_ui_filter.py`: UI string tests passed during attempt; labels include controls existence, default none, Type multiple selection, compose behavior, current filter state, raw base unchanged, OR group, empty raw query, local persistence/restore, and prefetch interruption wiring. These child-created untracked test/harness files were removed with canonical `kilo-safe-remove` after the failure re-delegation.
- Python full test command:
  - `python -m pytest tests/ -v`
  - Result: `1 failed, 60 passed in 12.74s`
  - Passing relevant tests:
    - `tests/test_search_ui_filter.py::test_search_ui_filter_fixed_behaviours`
    - `tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_pages`
    - existing UI harness tests, including `tests/search_prefetch_harness.mjs` via `tests/test_search_prefetch.py`
  - Pre-existing out-of-scope failure:
    - `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists` fails because `AGENTS.md` lacks pointer to `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`.
    - `AGENTS.md` is outside UFI-1 approved scope. This is recorded as contradiction between confirmation rule and current project file state, not as UFI-1 failure.
- Go verification command in `app`:
  - `go vet . && go test -count=1 -timeout 300s .`
  - Result: `ok hitomi-hub/app 28.270s`
- Canonical build:
  - `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
- Served UI confirmation from canonical build:
  - Server: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`
  - Window size actual: `1280 x 900`
  - Cold hygiene: `localStorage.clear()` before measured operation; fresh canonical server instance; initial `furry` load is warm, measured filter query is new and UI status showed `cache=50 miss=25`
  - Before: raw base `female:yuri`, filter none; network request `/api/search?query=female%3Ayuri&lang=all&page=1&limit=25` returned 200; UI status `検索完了・カード表示済み 25件` / `32957 件・ページ 1`; screenshot `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\search-filter-before.png`
  - After: Type `manga` + `doujinshi`; route hash `#/q/female%3Ayuri%20and%20type%3Amanga%20or%20type%3Adoujinshi/page/1`; network request `/api/search?query=female%3Ayuri%20and%20type%3Amanga%20or%20type%3Adoujinshi&lang=all&page=1&limit=25` returned 200; UI status `検索完了・カード表示済み 25件` / `48675 件・ページ 1` / `cache=50 miss=25`; screenshot `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\search-filter-after.png`
  - Drive screenshots copied. Workspace local copies were removed after Drive copy.

## UFI-1 Attempt 2 実装

- Current approved scope used `app/viewer_test.go` for UI string tests and Go `and` group-boundary tests; `app/searchhtml_test.go` was not changed because current approved scope did not include it.
- `app/search.html` dedicated controls: `#series-filter`, `.series-type-filter-row`, Type checkboxes `type-filter-manga`, `type-filter-doujinshi`, `type-filter-manhua`, `type-filter-manhwa`, `type-filter-other`, and `data-type-filter`. Existing status checkbox behavior was separated through `data-status-filter`.
- `app/searchhtml.go`: `buildSearchState()` treats `and` as UI-generated group separator with `if term == "or" || term == "and" { continue }`, preserving existing `or` group behavior.
- Current UI string test assertions cover default none, Type multiple selection, `composeFilterGroup()`, `composeEffectiveQuery()`, `loadFilterState()`, `saveFilterState()`, `restoreFilterControls()`, local storage restore wiring, and prefetch interruption wiring.
- Current Go group-boundary test `TestUFI1BuildSearchStateAndGroupBoundary` asserts raw positive term AND filter group, raw OR group AND filter group, and raw OR ending followed by `and` filter group through composer.
- Existing `app/viewer_test.go` anchors were fixed:
  - `TestEmbeddedUISearchPageLayoutSeparatesResultsFromSideInputs`: `.main` slice end uses `<script>` instead of `\n</div>\n<script>`.
  - `TestEmbeddedUIPreservesSearchStateAcrossReload`: initial restore anchor uses `showLists();` and `const initialSearch = parseSearchHash(window.location.hash);`; save-state assertion now expects `saveSearchState({ query, rawQuery, lang, page, ids, total: data.total, filter });` and `saveFilterState(filter);`.
- Served UI revealed `composeFilterGroup()` initially produced `ortype` because `join(" or")` lacked the space after `or`. This was fixed to `join(" or ")` and the Go assertion was updated.

## Verification Attempt 2

- Go verification command in `app`:
  - `go vet . && go test -count=1 -timeout 300s .`
  - Result: `ok hitomi-hub/app 26.340s`
- Python full test command after canonical build:
  - `python -m pytest tests/ -v`
  - Result: `60 passed in 13.23s`
- Canonical build:
  - `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
- Served UI confirmation from canonical build:
  - Server: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731`
  - Isolated context: `ufi1fresh`
  - Window size actual: `1665 x 932`
  - Cold hygiene: fresh canonical server instance, isolated origin `ufi1fresh`, `localStorage.clear()` before measured operation (`Object.keys(localStorage).length = 0`); initial `furry` load is warm, measured filter query is new
  - Before state: raw base `female:yuri`, filter none; network request `GET /api/search?query=female%3Ayuri&lang=all&page=1&limit=25` returned `200`; UI status `検索完了・カード表示済み 25件`, `58581 件・ページ 1`, `キャッシュ miss=50`; screenshot `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\ufi1-before.png`
  - After state: raw base `female:yuri`, Type `manga` + `doujinshi`; effective query `female:yuri and type:manga or type:doujinshi`; network request `GET /api/search?query=female%3Ayuri%20and%20type%3Amanga%20or%20type%3Adoujinshi&lang=all&page=1&limit=25` returned `200`; UI status `検索完了・カード表示済み 25件`, `48675 件・ページ 1`, `キャッシュ miss=50 cache=25`; screenshot `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\ufi1-after.png`
  - Drive screenshots copied. Workspace local copies were removed after Drive copy using `kilo-safe-remove`.

## 判定 Attempt 2

- UFI-1 is pass-ready. This commit records implementation/test/task evidence and closes UFI-1.
- Known limitation remains: shared hash contains only the effective query; filter controls are restored from local filter state, so shared hash-only links may not restore filter controls.

## 親の機械確認

- UFI-1 commit is absent at HEAD `714d13675e022c4d3d7a945c3cfb87c8083f088b`.
- After failure re-delegation, `git status --short` shows only task-record changes; child report matches task record status `未解決`.

- UFI-1 Attempt 2: commit `b2641df` exists. Child report matches task record: full pytest `60 passed`, Go app `ok hitomi-hub/app 26.340s`, served UI before/after evidence, UFI-1 pass. At parent confirmation time, only the archived known-issue parent confirmation remained uncommitted.
## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 追加確認（固定 Type filter 値）

- 同意した固定 gallery type 値を `app/search.html` に反映:
  - `doujinshi`
  - `artistcg`
  - `manga`
  - `imageset`
  - `gamecg`
  - `anime`
- `manhua`, `manhwa`, `other` は固定 gallery type 値から分離。
  - `manhua` は中国漫画、`manhwa` は韓国漫画、`other` は catch-all。
  - これらは `type:` namespace 検索値として使いうるが、recommended fixed `gallery.type` enum values ではない。
- `app/viewer_test.go` に exact fixed values と no-filter branch を追加:
  - `strings.Count(search, "data-type-filter") != 6`
  - `composeEffectiveQuery` の `if (!filterGroup) return base;`
- Served UI network 確認:
  - fresh isolated browser context `ufi-fixed-type-no-filter-20261108`
  - actual window size: outer `1678x1025`, content viewport `1665x932`
  - raw query: `female:yuri`
  - filter state: `series=""`, `types=[]`
  - no filter network request: `GET /api/search?query=female%3Ayuri&lang=all&page=1&limit=25`
  - response: `200 OK`
  - UI display: `Query: female:yuri`
  - fixed Type filter controls: `doujinshi`, `artist CG`, `manga`, `image set`, `game CG`, `anime`
- Drive artifacts:
  - `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\fixed-type-no-filter-before-20261108.png`
  - `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\fixed-type-no-filter-after-20261108.png`
  - `I:\マイドライブ\kilo\2026-11-08-ui-series-type-filter\fixed-type-no-filter-confirmation-20261108.txt`
  - local copies were sent to Recycle Bin with canonical `kilo-safe-remove`.
- Verification:
  - `go vet . && go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 24.529s`
  - `python -m pytest tests/ -v` after canonical rebuild: `60 passed`
  - canonical rebuild: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
