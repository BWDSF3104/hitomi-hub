# N-17: pytest の freshness rule pointer テストが落ちたまま（KI-20261106-04）

**Status**: 進行中
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
