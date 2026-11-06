# R-18: listRows(unread) が無制限で viewed/saved のみの行も含む

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。案: LIMIT、または viewed/saved のみの行を除外。/api/lists と DOM の肥大化に対応。記録: R-18

## 意図 / 背景

- 外部指摘（Claude）の照合: `listRows("unread")`（`app/main.go:1387-1397`）は limit なしで全行を返す。C-05（bounded cache 流儀）との整合、page 化・上限の可否。
- 追記指示の前提「既読・お気に入り表は bounded（C-05）」は正典照合で不成立（下記）。実測で有効性を確定し、意味変更側（全件 vs window）は承認案件として分離する。
- 変更したのは本記録（`docs/ai/`）のみ。ソースコード・正典 DB・完成フォルダは未変更。スクラッチ: `docs/ai/tmp/assess-20261106-1056/`。

## 調査結果 / 変更内容

### 現在の実装（機構・確定）

- `listRows("unread")` = `SELECT ... FROM read_state WHERE fully_read = 0 ORDER BY gallery_id`（`app/main.go:1387-1397`）。LIMIT なし。
- `/api/lists`（`app/viewer.go:514-531`）が全 row を JSON 化（`jsonOut`）。client `showLists`（`app/search.html:689-691`）が全 row を `#unread` に join（DOM node = row 数）。
- 同一 `listRows` を CLI `hub list`（`app/main.go:1234-1240`）も共有 → 意味変更は UI 限定でなく CLI 出力へ波及。
- viewed のみ・saved のみの row: `setViewed`（`app/library.go:96-101`）と `setSavedEvent`（`app/library.go:107-112`）は read 進捗と独立に `read_state` 行を INSERT（`viewed_at` 初回のみ・`saved_at` event）。`fully_read = 0` で unread に含まれる → 指摘どおり（機構確定）。

### 正典照合（前提の妥当性）

- **C-05 は caches 限定**: 承認範囲は memory cache 4 種 + `indexTopCache`・`noFullCache`・disk galleryinfo 層（`docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`）。`read_state` は cache ではなく永続 state 表であり、bounded 化された正典記録はない。原文の「既読・お気に入り表は bounded（C-05）」は誤り → bounded 流儀の lists への適用はルール適用として不成立（state 表の cap = 既存行消失＝不可逆・意味変更、D-03 非破壊規約と衝突）。
- **D-03 非破壊規約**: findings の insert-only・既存行削除禁止・列追加のみパターン（`docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md`）。state 表の「cap + eviction」型 bounded 化はこの規約と矛盾。
- **decision §8 照合**: 「page slice は cache から取る」（`docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md` §8）は search term cache の話で lists ではない。`read_state` は cache でなく、§8 は lists へ拡張適用できない（衝突なし・適用余地なし）。
- **B-01 参照の矛盾**: 追加指示の「B-01（batch）」は正典上 `docs/ai/decisions/2026-11-03-b01-nav-enter-noop.md`（nav Enter no-op・本件と無関係）。verify missing batch の正典記録は grep 0 件（実体なし）。batch 機構の実体は decision §8 と commit `42f1765`（`recordDiscoveredBatch`）。参照名と内容不一致 → 採用文書: decision §8・decision 2026-11-05。
- **一覧の意味の固定**: `listRows("unread")` の「未読」意味は変更しないとして記録済み（`docs/ai/tasks/2026-11-05-search-result-state-badges-implementation.md` line 25）。page 化・上限（全件 → window）はユーザーに見える意味変更 → 承認が必要。

### 実測（scratch 1,000 / 20,000 行・2026-11-06）

- DB 構成（viewed のみ 70%・saved のみ 20%・last_page>0、全行 fully_read=0）。正典 `hitomi.db` read-only 実測: `read_state` total=28（unread=28・user_version=6・saved_at 列なし）→ 現実データは小規模。数万件は想定規模。
- **JSON bytes**: 65,041 B（1,000）→ 1,300,041 B（20,000）（约20倍）。
- **応答生成**: 1,000 行 cold 初回 252 ms → 安定 5.4〜13.6 ms。20,000 行 53 ms（loopback curl・fresh instance）。page 内 fetch+parse 74.6 ms。
- **DOM 生成**: string join 0.48 ms（1,000）/ 9.48 ms（20,000）。「served page #unread innerHTML parse 40.5 ms（1,000）/ 120.3 ms（20,000）。表示 20,000 nodes・要素高 641,875 px（sidebar overflow visible・スクロール表示・固定寸法契約なし）。
- 画面確認（confirmation-test-rule 準拠・正典 exe + fresh instance port 8733 + isolated origin（storage clear 済み）・window size 1678x1025 / viewport 1665x932・人工 resize なし）: `#lists-refresh` click 前後（同一条件）で before 1,000 nodes → after 20,000 nodes（first li `1000001 p3`・last li `1020000 p3`）。保存先: `I:\マイドライブ\kilo\2026-11-06-r18-unread-list-limit\`（before/after png + confirmation txt）。local copy は `kilo-safe-remove` でゴミ箱へ。

### 有効性

**一部有効**: 機構（無制限・viewed/saved のみ行の inclusion）は確定。ただし (1) C-05 整合の前提が正典上不成立（state 表は bounded でない・bounded にできない）、(2) 実測影響は規模依存で 20,000 件でも応答 50〜75 ms・DOM 約120 ms・1.3 MB にとどまり、機能破滅ではない。現実データ 28 行で未発現。

### 推奨

- **案 A（推奨・意味変更なし）**: 現状維持（`listRows("unread")` は全行読み取り）。`read_state` は cache ではなく state 表で、bounded 流儀は適用不可。実測で現規模の応答/DOM は許容内。
- 万一 mitigation を入れる場合は承認案件として分離（いずれも一覧の意味＝全件 vs window の変更）: (a) server 側 limit param（既定 unlimited）+ 総件数表示、(b) client 側 window 表示 + 「全 N 件中 N 件表示」明示。意味変更側・CLI 波及を含むためユーザー承認が必要。

### 副作用・影響範囲

- `listRows` 共有口（CLI `hub list`）へ波及。既存テストは lists の「全件 vs window」意味を固定していない（`viewer_test.go` lists 確認・`tests/read_lists_debounce_harness.mjs` は fetch/debounce/guard 固定のみ）→ LIMIT 追加で既存テストは壊れないが、意味固定は decision 2026-11-05（「未読」意味変更しない）により別項目・別承認。
- `mergeReadState` は lists row に `viewed` を含まない（`listRows` の SELECT 列外）ため badge の viewed 源は card API（decision 2026-11-05 第5項どおり）→ window 化しても card badge は card 応答側で維持。

## Verification

- `go vet .`（app）→ clean。`go test -count=1 -timeout 300s .`（app）→ **ok 25.549s**（全通過・コード非変更）。
- `python -m pytest tests/ -v` → **54 passed / 1 failed（9.21s）**。failure は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来の既存 failure・R-10 報告済み・本項目と無関係）。`test_canonical_executable_embeds_current_pages` は合格（正典 hub.exe が現行 app HTML バイトを埋め込み → cold hygiene condition 1 の根拠）。
- 正典 build（rebuild）は追加指示（完成フォルダを書かない）により実施せず、正典 exe + freshness test 合格で現行一致を確認。server は正典 exe を scratch dataDir（seed DB）+ port 8733 の fresh instance で起動。
- 正典 DB・完成フォルダ未変更。正典 DB read-only（`mode=ro`）照会のみ。スクラッチスクリプトのみ実行（`seed.py`・`r18dom.mjs`・`r18_dbcount.py`）。外部 API 呼出しは #unread 計測には不使用（/search 起動 path により load 1 回につき 1 件の外部 search が page 設計上不可避で発生・記録）。
- 逸脱記録: `go test -overlay` 注入（C-05 正典流儀）は Go 1.27.3 windows で解決されず（4 鍵形式試み・no tests to run）。served 実測 + node harness（harness_lib 流儀）+ curl で代替（実測表の取得方法は本記録に明記）。
- 画面確認: cold hygiene 3条件（freshness test 合格・fresh instance・isolated origin（storage clear 後 fresh navigation））満足。window size 実測 1678x1025 / viewport 1665x932、before/after 同一条件（viewport 画像・同一被写体 #unread）。Drive 保存 + local copy ゴミ箱送信済み。
- 実測数値は上欄（JSON bytes・応答 ms・DOM ms・nodes・正典 DB 28 行）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（正典 `task_manager` で閉じる際に削除。親が管理）
- [ ] 解決した `known-issues/` ファイルを `archive/` へ `mv`（該当する場合）: 該当なし（本項目で known-issues は作成しない。B-01 参照矛盾と overlay 流儀逸脱は本記録にのみ記載）
- [ ] `backlog.md` の完了項目を削除（該当する場合）: 該当なし
- 推奨（案 A・現状維持）が採用された場合は実施項目として閉じ、page 化・上限の案は意味変更の承認案件として分離（決定時に `decisions/` へ記録が必要）

## 対応不要（親の記録・assess-20261106-1056）

- 対応不要として閉じる。理由: 実測で現規模（正典 DB 28 行）は許容内、20,000 件でも JSON 1.3 MB・応答 53 ms・DOM 120 ms にとどまり機能破滅がない。`read_state` は cache ではなく state 表で、bounded 流儀（C-05）は正典上適用不可、D-03 非破壊と衝突。
- page 化・上限（全件 → window）は一覧の意味の変更＋CLI `hub list` 波及を含むため、承認案件として本記録に分離 retained。優先順位（本 record の根拠）は 18 位: `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` 優先順位節。
