# テスト保守方針の追認（增量方針：新規行為テスト harness・既存 anchor 維持）

**Status**: 決定（ユーザー同意済み 2026-11-09 の推奨案の追認。項目 #16 / assess-20261109-1320。実装開始指示を要さない記録のみの変更）
**Date**: 2026-11-09
**Task**: `docs/ai/tasks/2026-11-09-n23-tests-string-contains-fragile.md`

## 追認する方針（增量）

1. **新規 UI 行為テストは Node harness 執行検査**（正典流儀のまま: `tests/harness_lib.mjs`・`new Function` 関数抽出・mock DOM/fetch 上実行）。現行 harness 7 本・check 169 件・pytest wrapper 10 本経由で `python -m pytest tests/` に統合済み（正典化: `2026-11-04-confirmation-script-canonicalization-approved.md`）。
2. **既存 Go の HTML substring anchor は正典意味固定の設計コストとして維持**。「変数名変更・空白で壊れる」相当分は意図された literal 固定（機構文字列・正典値の assert）であり、fragility ではなくコストとして受け入れ済み（常設方針 `2026-11-06-ui-polish-jp-copy-and-anchors.md`）。
   - 執行検査として固定可能な意味（debounce 順序・guard・DOM 反映）のみ增量的に harness へ移す。
   - 言語横断・正典値の literal 固定（gate 値 90/60/1000/300 ms・x-cache 値・no-store・generation guard・TN display fallback 禁止・actual pixels 分離）は anchor のまま維持。
   - whitespace 正規化・stable token anchor への寄与は許容範囲。
3. **re-anchor は変更時のみ実施**（表示文字列・意味の変更に合わせて再配置。事前の一括刷新は行わない）。実測: decisions で re-anchor を含む entry 12 件。
4. **全面移行は不採用（承認案件のまま）**。Go サーバー側正典意味（imageProxy 取得順序・gg live/snapshot・shard URL format・StorageURL・x-cache・exe embed freshness）は Node で執行も照合もできず、659 anchor の一括移行は正典ルール変更にあたる。実施する場合は「方針変更の提案」として分離し、承認を得てからとする。
5. **意味・合格条件の弱化は禁止**（AGENTS.md・2026-11-06 policy のまま）。正典 suite の合格条件はこの追認により変更しない。

## 分類（実測 2026-11-09 の確定）

- `app` Go テスト 13 ファイルの `strings.Contains(` 計 786 箇所。
- UI ソース（embed 済み viewer/search/settings ページ）anchor: `app/viewer_test.go` 659（ファイル内 93.5%・同ファイル Contains 計 705）＋ `export_test.go` 5 ＋ `main_test.go` 5。
- `searchhtml_test.go` 47 箇所は hitomi 応答 HTML の fixture 照合で、UI ソース anchor とは分類を区別する（移行対象の性格が異なる）。
- 構造的原因: Go は JS を実行できないため UI 内容アサートは substring anchor に限定される（`go:embed` app/viewer.go:23）。執行検査化には Node 実行以外に手段がない。

## 実測の証拠

- `go vet . && go test -count=1 -timeout 300s .`（app）: 2026-11-09 実測 — vet 問題なし・`ok hitomi-hub/app 37.362s`（全緑）。
- `python -m pytest tests/ -v`（root）: 2026-11-09 実測 — `63 passed in 15.97s`（node harness 7 本・wrapper 10 本含む全通過）。
- 画面確認: 該当なし（テスト方式の保守性方針。UI 項目ではない）。

## Supersede 対象

なし（既存 decision の policy の追認・確立であり変更ではない。`2026-11-04-confirmation-script-canonicalization-approved.md` と `2026-11-06-ui-polish-jp-copy-and-anchors.md` のまま維持）。
