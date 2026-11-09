# N-17: pytest の freshness rule pointer テストが落ちたまま（KI-20261106-04）

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: pytest の `test_canonical_build_freshness_rule_exists` が落ちたまま（KI-20261106-04）。テストが AGENTS.md に探す pointer 文字列が集約後に存在しない。テスト側を新正典 `docs/ai/rules/confirmation-test-rule.md` へ合わせる修正が必要（AGENTS.md は write-protected）。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-17 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

（調査のみ。テストファイル・AGENTS.md は未変更）

### 現在の実装と判断根拠

- 照合対象ファイルは **project AGENTS.md**（`tests/test_canonical_build_freshness.py:19` の `agents = ROOT / "AGENTS.md"`）。global `C:\Users\fujim\.config\kilo\AGENTS.md` は照合対象ではない。
- テストが照合する literal pointer: `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`（`tests/test_canonical_build_freshness.py:31`）。
- 現 project `AGENTS.md:61` の Rules 条は `docs/ai/rules/confirmation-test-rule.md` への pointer のみ。上記 literal pointer は project AGENTS.md に不在（global AGENTS.md にも不在）。よって `tests/test_canonical_build_freshness.py:31-32` で失敗。
- テストは pointer 文字列だけでなく内容も照合: decision 側の needle（`app/viewer.html`・`app/search.html`・`失敗メッセージには両ファイル名`・`served 画面`）と build command literal。これらは現在通過（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md:17-18`、`docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md:13`）。
- `docs/ai/rules/confirmation-test-rule.md` は実在（59 行・単一ルール）し、§1 で「正典ビルドの screen confirmation」（fresh canonical `hub.exe` が served する UI に対して確認、source string テストだけでは不合格、`tests/test_canonical_build_freshness.py` を含む pytest 合格）を扱っている。§2 window size、§3 cold hygiene、§4 screenshot 被写体・取得方法、§5 before/after、§6 Drive 保存と local cleanup も同ファイルに集約済み。

### 有効性

**有効**（機構は実測で確定。判断保留ではない）。破損ではなく正典 pointer とテスト期待値の不一致。

### 正典照合（矛盾と採用文書）

- 古い文書: `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`（Rules 条の参照先 = separation decision。テストの期待値はこれに基づく）。
- 新しい文書: `docs/ai/rules/confirmation-test-rule.md`（Date 2026-11-06、Status active、ユーザー指示 2026-11-06 08:13 UTC。AGENTS.md の Rules pointer は本ファイル1本のみ、テスト系規則は同ファイルへ集約、旧 rule 4 本は supersede）。
- 状態記述の矛盾: 古い decision は「AGENTS.md は decision へ pointer」と要求し、新しい rule は「AGENTS.md Rules pointer は本 rule ファイル1本のみ」と要求。日付の新しい `docs/ai/rules/confirmation-test-rule.md` を採用。テストの期待値が現正典より古い。
- project copy 条は守られている: `docs/ai/rules/` の実ファイルは `confirmation-test-rule.md` 1 本のみ（旧 4 本は commit `e018c1c` で削除済み）。
- AGENTS.md は write-protected。AGENTS.md 側へ literal pointer を復活させる案は、新正典「Rules pointer は1本のみ」と矛盾し、かつ承認を要する。よって**テスト側を新正典へ合わせる修正が最も適切**（正典流儀と矛盾しない: AGENTS.md は参照行のみ、内容正典は参照先、単一ルール・project copy なし）。

### 推奨する修正方法（実施していない）

1. `tests/test_canonical_build_freshness.py:31` の literal を `docs/ai/rules/confirmation-test-rule.md` へ変更（project AGENTS.md に既存の literal、`AGENTS.md:61`）。
2. 照合の意図（pointer の存在＋実質内容が参照先正典にある）を失わないため、rule ファイル側の実質内容 needle を追加: `hub.exe`、`served 画面`、`window size`、`isolated origin`、`before`、`kilo-safe-remove`（いずれも rule 側に存在確認済み）。任意で「AGENTS.md は pointer のみ」（AGENTS.md に window size・cold hygiene 等の直接記載がないこと）も照合可。
3. decision 内容の needle 照合は**rule ファイルへ移してはならない**: `app/viewer.html` と `失敗メッセージには両ファイル名` は rule ファイルに存在しない（rule 側は `app/*.html` と `app/search.html` のみ）。2ファイル照合・失敗メッセージの意図は decision（内容有効のまま、`2026-11-04-agents-md-rule-pointer-ization.md` の supersede 扱い）と `tests/test_canonical_build_freshness.py:69-76` の bytes 照合が持つ。
4. window size は数値（outer `1678x1025`・viewport `1665x932`）を合格条件にする照合は加えない。現正典は固定目標サイズを合格条件にせず「実際に表示している window size」を基準とする（`confirmation-test-rule.md` §2）。数値はタスク記録の実測値であり、テストが数値を要求すると正典と矛盾する。

### 同一条件で 60 passed になるか（テスト code path の読み解き）

- 推奨 variant をスクラatch で模擬（テストファイル・AGENTS.md は未変更）: `docs/ai/tmp/assess-20261108-1332/r17/test_recommended_variant.py` → **1 passed（0.37s）**。
- 判定内訳: project AGENTS.md に `docs/ai/rules/confirmation-test-rule.md` literal あり → pointer 照合通過。rule ファイル実在＋内容 needle 通過。decision・build command 照合は現状通過済み（影響なし）。
- 現Suite 60 件（59 passed + 本件 failed）。 pointer 判定のみ閉じれば同一条件（canonical `output/hitomi-hub/hub.exe` 実在・go toolchain 使用・文件未変更）で **60 passed** が期待値。

### 副作用・影響範囲・代替案

- 影響はテスト1ファイルのみ（`tests/test_canonical_build_freshness.py`）。コード・DB・完成フォルダ・正典 exe は無影響。`docs/ai/rules/` の照合対象テストは本ファイルのみ（literal pointer 照合の箇所は1か所）。
- 担保の弱化: 実施しない。現テストは pointer 文字列のみでなく参照先正典の内容も照合しており、推奨案でも pointer 照合＋内容照合（rule 側）を維持する。`x-cache`・cold hygiene・window size 基準を弱める変更は含まない。
- 失う意図: 失わない。ただし「decision ファイルへの参照」を照合する意図は新正典（単一 rule ファイル1本 pointer）で不要になる。separation decision の内容照合を残すこと（2ファイル照合・失敗メッセージ両ファイル名・served 画面）が担保の維持条件。
- 代替案: (a) AGENTS.md へ literal を復活 → write-protected + 新正典「Rules pointer 1本のみ」と矛盾 + 承認要 → 不適。 (b) rule ファイルへ needle を全部移す → 上記2 needle 不在で不合格 → 不適。 (c) 現テストを「pointer is either decision or rule file」の OR 照合へ → 二重正典化で流儀（単一ルール）と矛盾 → 不推奨。
- 不可逆なデータ変更なし。

### 画面確認

UI 項目でないため該当なし。Drive 保存なし。正典 exe・served 画面確認の実施可否とは無関係。

## Verification

- `python -m pytest tests/ -v`（root・2026-11-08 実測）: **1 failed, 59 passed in 10.09s**。失敗は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（`tests/test_canonical_build_freshness.py:32` の literal pointer 判定）。再現確認済み（KI-20261106-04 と同一機構。前批の「54 passed / 1 failed」は現 Suite 60 件へ増えた件数差）。
- 推奨 variant の模擬: `python -m pytest "docs\ai\tmp\assess-20261108-1332\r17\test_recommended_variant.py" -v` → **1 passed in 0.37s**。同一条件で本項目の判定は閉じ、Suite は 60 passed が期待値（テストファイル・AGENTS.md は未変更）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: 該当なし（Go 変更なし）。node harness 実行不要。
- 照合ファイルの実在 read 確認: `docs/ai/rules/confirmation-test-rule.md`（59 行・§1 で正典ビルド screen confirmation を扱う）。project `AGENTS.md:61` に literal pointer あり、`tests/test_canonical_build_freshness.py:31` の literal は project AGENTS.md・global AGENTS.md ともに不在。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）— `KI-20261106-04` はテスト側修正の実装完了まで未解決のまま（本調査は実施していない）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

実装開始指示待ち（テスト側修正は未実施）。推奨のみ記録。

## 2026-11-09 再調査（assess-20261109-1320）

### 現行実測（2026-11-09）

- `python -m pytest tests/ -v`（ルート）: **60 passed / 0 failed**（17.76s）。pytest 側は KI-20261106-04 の記録どおり解決済み（現 Suite 60 件・`test_canonical_build_freshness_rule_exists` PASSED）。
- `go vet . && go test -count=1 -timeout 300s .`（app）: `go vet` ok、`go test` **ok（38.094s・失敗なし）**。KI-20261106-04 の「Go 側未解決残留」記述（2026-11-08 実測）は**現行では残留していない**。
- 該当 2 テストの個別確認（`go test -v -run` で PASS 実測）:
  - KI `viewer_test.go:1818`（slice end not found after start）→ 現行 `TestEmbeddedUISearchPageLayoutSeparatesResultsFromSideInputs`（`app/viewer_test.go:1815`、assertion `app/viewer_test.go:1818`）。現行構造と合致して合格。
  - KI `viewer_test.go:2536`（initial search restore not found）→ 現行 `TestEmbeddedUIPreservesSearchStateAcrossReload`（`app/viewer_test.go:2504`、Fatalf `app/viewer_test.go:2537`）。現行構造と合致して合格。

### 現行の実装と判断根拠（機構）

- 失敗の機構は**構造不一致ではなく EOL（CRLF）依存**。`core.autocrlf=true`（実測）・`.gitattributes` 不在（実測）でワーキングツリーは CRLF（`app/search.html` 実測: CRLF 1098 行）。`go:embed`（`app/viewer.go:23`）はディスク bytes を埋め込むため、テスト照合 byte は CRLF 内容となる。
- 落ちた 2 assertion は raw `\n` の完全 adjacency を要求する型: `"\n</div>\n<script>"` と `showLists();\nconst initialSearch = parseSearchHash(window.location.hash);`。CRLF 内容では `</div>\r\n<script>` / `showLists();\r\nconst…` となり needle と一致しない。他の `"\nfunction …"` 型 needle は `\r\n` の `\n` 側で一致するため落ちない（= 実測の「ちょうど 2 件 failed」と整合）。
- 構造（LF blob 上）は照合条件と常に一致: `git show f6531c6:app/search.html`・`git show d35faed:app/search.html`（KI 実測 HEAD 直前）で両 literal の存在確認済み（`docs/ai/tmp/assess-20261109-1320/check_literals.py`・`check_eol.py`）。よって「現行構造と合っていない」は正典意味としては**EOL 依存の脆い assertion 期待値**の問題で、コード構造の正しさは保たれていた。
- 解決 commit: `b2641df`（2026-11-09 01:08・UFI-1 dedicated Series/Type filter）が 2 assertion を EOL 非依存形へ re-anchor（`sliceBetween(..., "<script>")` 終端、`showListsAt`/`initialAt` の順序照合）。現行 CRLF ツリーで全件合格。

### 正典照合（矛盾と採用文書・日付優先）

- 矛盾: `KI-20261106-04`（archive）の Status「解決」は pytest 側のみを指し、同 record の「Go 側未解決残留（2026-11-08 実測）」は現行実測（2026-11-09）と矛盾する。
- 採用: 日付優先の正典ルールにより、**現行実測（2026-11-09）と `b2641df`（2026-11-09 01:08）**を採用。古い「未解決残留」状態記述は根拠にしない。KI archive は参照のみで未変更。訂正提案: archive record に「Go 側は b2641df の EOL re-anchor（2026-11-09）で閉」を補記（正典実務としての記録更新）。

### 有効性

**有効（矛盾は実在）だが実測で閉済み**: Go 側の失敗は現行に残留なし。残るアクションは KI archive の状態記述訂正（記録作業）のみ。実装は不要（すでに re-anchor 済み・合格）。

### 推奨する修正方法

1. 推奨: KI archive の「未解決残留」記述の訂正補記（上記）。テスト・コード変更は不要。
2. 代替（承認要・実施しない): `.gitattributes` で `app/*.html` を LF に固定し EOL 依存脆性を根源から除去。現行 assertion は EOL 非依存なので追加措置は不要（将来の raw `\n` needle 追加時に再発リスクあり）。

### 副作用・影響範囲

- 本調査での変更なし（変更は本タスク記録のみ）。実 DB・完成フォルダ・config・正典 exe 無影響。
- `b2641df` の re-anchor は guard 意味を維持: results が `.side` に戻らない照合（2 ファイル分離）と showLists 後の restore 順序照合が保持される。完全 adjacency 照合より弱いが EOL 非依存化に必要な最小形。

### 画面確認

UI 項目でないため該当なし。Drive 保存なし。

### 実測の再掲（この子実行）

- pytest（ルート）: 60 passed in 17.76s（2026-11-09）
- `go vet . && go test -count=1 -timeout 300s .`（app）: vet ok / test ok 38.094s（2026-11-09）
- 個別 `-v -run "TestEmbeddedUISearchPageLayoutSeparatesResultsFromSideInputs|TestEmbeddedUIPreservesSearchStateAcrossReload"`: 2 件 PASS（1.207s）
- 調査 scratch: `docs/ai/tmp/assess-20261109-1320/`（gitignore 対象）
