# テストの HTML 文字列部分一致依存（保守性）

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

項目 #16（実行固有ID: assess-20261109-1320）の調査。原文の主張は3点: ①viewer_test.go 大半が HTML 文字列部分一致（`strings.Contains`）で空白・変数名変更で壊れる、②re-anchor が decisions に何度も出ていて変更コストが高い、③harness(Node) 移行は良い方向。実数・decisions 件数・harness 適用範囲を実測し、保守性項目として有効性と優先順位付けへの効き方を確定する。

## 調査結果 / 変更内容

### 1. 実数と割合（実測 2026-11-09）

- app の Go テスト 13 ファイルで `strings.Contains(` 計 786 箇所。うち HTML ソース（embed 済み ui ページ）の substring 照合は 669 箇所（85.2%）。
- `app/viewer_test.go` 単独: 705 箇所（全体の 89.7%）。そのうち HTML anchor 659 箇所（ファイル内 93.5%）。test 関数 133・HTML anchor 関数 58（`TestEmbeddedUI*` が 42 関数）。
  - 注記: 原文の `TestEmbeddedUIFor...` 系は実在しない（該当は `TestEmbeddedUI*` 42 関数）。テスト関数名の語源は C-07/C-09 記録（`TestEmbeddedUIDebounces...` 等）で確認。
- 「大半」は viewer_test.go については事実。ファイル全体の割合（85.2%）も事実。ただし `searchhtml_test.go` の 47 箇所は hitomi 応答 HTML の解析テスト（fixture 照合）で、UI ソース anchor とは性格が異なる（区別して数えるべき）。UI ソース anchor は viewer_test.go 659 + export_test.go 5 + main_test.go 5。
- 構造的原因: Go は JS を実行できないため、Go 側の UI 内容アサートは substring anchor に限定される（`go:embed viewer.html search.html settings.html` app/viewer.go:23）。fragility は Go 方式に内在し、部分一致を「執行検査」に変えるには Node 実行（harness）以外にない。

### 2. re-anchor の実例（decisions 12 entry）

`docs/ai/decisions/` で "re-anchor" を含む entry は 12 件（test・anchor 再配置関連が約 10 件）。「何度も」は事実。実例:

- `2026-11-04-freshness-rule-test-re-anchor-to-pointer.md` — 直接 3 literal 照合からポインタ実態照合へ re-anchor（ユーザー承認済み）
- `2026-11-04-search-viewer-separation-approach-approved.md` — viewer_test.go の 37 assertion を 15〜20 件 re-anchor・7〜9 件 rewrite と見積（当時は 37 assertion → 現行 705 箇所へ増加。成長自体が変更コストの証拠）
- `2026-11-04-search-page-layout-and-vestige-cleanup-approved.md` — re-anchor 6〜8 件
- `2026-11-05-search-form-row-contract-re-anchor.md`（正典 entry）、`2026-11-06-tn-image-lane-60ms-approved.md`（正典値アサート 60 ms へ）、`2026-11-09-disk-cache-cap-count-4000.md`・`2026-11-09-settings-centralization-scope-approved.md`（main_test.go 8000→4000）
- fragility の実損例: `known-issues/archive/2026-11-04-strip-rebuild-harness-crlf-fragile-in-clean-clone.md` — clean clone の CRLF で exact 照合（`\n` 依存）が失敗。line ending 正規化で解決（「文字列マッチの除去ではない」＝意味検証は維持と明記）
- 常設方針: `2026-11-06-ui-polish-jp-copy-and-anchors.md` — 表示文字列変更に合わせて anchor を再配置、意味・合格条件（x-cache・no-store・generation guard・TN display fallback 禁止・actual pixels 分離）は弱めない。つまり「変数名変更で壊れる」は、正典意味の固定という設計上の受け入れコストでもある（guard 意味の literal 固定は意図的）。

### 3. harness(Node) の現行適用範囲

- `tests/harness_lib.mjs` は 2026-11-04 正典化済み（`2026-11-04-confirmation-script-canonicalization-approved.md`）。
- 行為 harness 7 本（strip_rebuild・read_lists_debounce・search_prefetch・search_visible_priority・search_manual_jp・r12_startup_single_run・viewer_objecturl_cap）＋共有 lib。check 計 169 件。7 本すべて `new Function` で関数抽出＋ mock DOM/fetch 上実行する執行検査（pure substring ではない）。ただし read_lists_debounce 等には block 内の exact 文字列 check も残る（混合型）。
- pytest wrapper 10 本経由で `python -m pytest tests/` に統合（60 tests）。
- 「移行が進んでいる」は事実。ただし適用範囲は新規 UI 行為テストのみ。既存 Go の TestEmbeddedUI* 系（659 箇所）は未移行。

### 4. 完全移行の現実性と推奨

- 完全置換は不可。Node harness では Go サーバー側正典意味（imageProxy 順序・gg・shard URL・storage path・DB・x-cache ヘッダ・exe embed freshness）を執行も照合もできない。テストが固定している正典意味の一部分は言語横断（Go/Python 側）でしか固定できない。
- 推奨（增量）:
  1. 新規 UI 行為テストは harness 執行検査（現行正典流儀のまま）
  2. 既存 Go HTML anchor は、執行検査として固定可能なもの（debounce 順序・guard・DOM 反映）を harness へ移し、言語横断・正典値の literal 固定は anchor のまま残す（whitespace 正規化・stable token anchor への寄与は許容範囲）
  3. 意味・合格条件の弱化は禁止（AGENTS.md・2026-11-06 policy のまま）。全面書き換え（659 Assertion の一括移行）は正典ルール変更であり、承認が要る論点として分離できる（判断ルールに従い、推奨として記録し、方針変更の提案側へ分離）

### 有効性

一部有効。数（85.2%・viewer_test.go 93.5%）・re-anchor の頻度（decisions 12 entry）・「移行が良い方向」は実測で事実。ただし①「変数名で壊れる」の相当分は正典意味固定の設計コスト（常設方針で運用確定済み）、②「完全移行」は言語横断の正典意味により不可。新タスクとしては「新設の既存 anchor 一括移行」ではなく「增量方針の追認・確立」として評価するのが適切。

### 優先順位付けに効く要素

- 重要度: 中（保守コスト。実行時リスクなし）
- 緊急度: 低（現行 suite 全緑。コストは UI 変更時の re-anchor 作業として顕在化し、既に手順化済み）
- リスク: 一括書き換えは正典意味弱化リスク（AGENTS.md 禁止）。harness 実行依存の node toolchain・CRLF 回帰（解決済み）
- 依存: `docs/ai/rules/confirmation-test-rule`、harness 正典化 decision（2026-11-04）、anchor 再配置方針（2026-11-06）
- 推定規模: 調査のみで実装なし。移行実施時は viewer_test.go 5880 行 / HTML anchor 659 箇所 / TestEmbeddedUI* 42 関数（大。数日規模）

### 画面確認

UI 項目ではない（テスト方式の保守性評価）。画面確認は該当しないため省略。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app）: 2026-11-09 実測 — vet 問題なし、go test `ok hitomi-hub/app 26.290s`
- `python -m pytest tests/ -v`: 2026-11-09 実測 — `60 passed in 28.41s`（node harness 7 本・wrapper 10 本含む全通過）
- 実測スクリプト: `docs/ai/tmp/assess-20261109-1320-contains-scan.py`（関数別 Contains 件数・HTML anchor 分類）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
