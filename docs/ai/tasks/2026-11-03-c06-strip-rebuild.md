# C-06: strip の再構築

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: CRLF harness 修正完了（単独 commit・clean clone 実測通過）。残作業なし

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（軽微）。`app/ui.html:1101-1128`（`renderStrip`）は showPage ごとに strip 全 entry（最大 `MaxStripPages`）を innerHTML で再構築し、`highlightStrip()` を呼ぶ。対応: page 移動時は active class 変更のみ（full rebuild は gallery open 時）。合格条件: page 移動で strip の img が再生成されない（mocked DOM で child 数不変）。推定規模: 小。UI 実操作確認（fresh tab, display load）は実装段階。

作業前チェック（実施）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を読了。TN display source（`stripThumbSource` → `displayViewerSource`）・TN display fallback 禁止・full cache 判定・`x-cache`・no-store fetch・objectURL 表示・generation guard はこの項目では変更しない。

設計（実装前に確定）:
- 新状態 `stripRenderedGallery` / `stripRenderedGroup`（`stripGroup` 宣言の隣）。`renderStrip()` は full rebuild 時にこれを記録。
- 新規 `refreshStrip()`: gallery と group window がunchangedなら `highlightStrip()` のみ（img 再生成なし・`currentGalleryObjectUrlKeys` 保持）。window 変更（group 境界跨ぎ・group 変更・gallery open 直後）のみ full rebuild。
- `showPage()` の `renderStrip(); highlightStrip();` → `refreshStrip();`。page out of range 早期 return 前にも strip 表示維持のため refreshStrip を先頭で呼ぶ。
- `openGallery()`: open 時状態リセット（`stripRenderedGallery = null`）＋ showPage 後の redundant `renderStrip()` 3 箇所削除（open 時の full rebuild は showPage 経由で 1 回だけ）。showPage 内 out-of-range 早帰りでも strip が描画されるよう showPage 冒頭で refreshStrip を呼ぶ。
- `setStripGroup()`（明示的 group 移動）は window 内容が変わるため full rebuild のまま（設計上不可避。合格条件の page 移動テストは同一 group window 内で固定）。
- 既知 Go test `TestEmbeddedUIProtectsAsyncReaderWrites`（`viewer_test.go` showBody の `renderStrip();`/`highlightStrip();` アサート）は page 移動が highlight-only になったため `refreshStrip();` へ更新。回帰テストを新設。

mocked DOM test 方式: `tests/strip_rebuild_dom_harness.mjs`（Node・mock DOM。ui.html から `stripItems`/`stripGroupPage`/`renderStripControls`/`highlightStrip`/`renderStrip`/`refreshStrip` を抽出し、stub 済み helper と mock `#thumbs` element 上で実行。innerHTML 書き込み回数を rebuild 数として数える）。pytest（`tests/test_strip_rebuild.py`）から実行。合格条件: open 後初回 rebuild 1 回、page 移動で innerHTML 書き込み 0（child 数・img nodes の object identity 不変）、active class（`current`）が移動先 img に反映、group window 変更時のみ rebuild。

## Verification

mocked DOM test（`tests/test_strip_rebuild.py`・`tests/strip_rebuild_dom_harness.mjs`・Node）: ui.html から現行関数を抽出し mock DOM 上で実行。18 checks 合格。page 移動で innerHTML 書き込み 0（child 数 50 不変・img nodes object identity 不変）、gallery open で full rebuild 1 回だけ、active class（`current`）が移動先 img に反映、group window 変更時のみ rebuild。

- `go vet .` clean（app）
- `go test -count=1 -timeout 300s .` → ok 66.947s（`TestEmbeddedUIPageMovementKeepsStripNodes` 新規・`TestEmbeddedUIProtectsAsyncReaderWrites` の showPage strip アサート更新を含む）
- `python -m pytest tests/ -v` → 39 passed（新規 `test_strip_rebuild.py`・`test_canonical_build_freshness.py` 含む）
- canonical exe rebuild（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）→ freshness 合格

UI 実操作確認（canonical server `http://127.0.0.1:8731`・fresh tab・Playwright・gallery 4300588・viewMode double）:
- open 後 strip = 8 img nodes（loading placeholder 消滅＝ rebuild 1 回）、`#thumbs` に MutationObserver 設置
- page 移動（1→3→5→7→9）: rebuild 検知 0 回、child 数 8・img nodes 数不変、probe tag の object identity 全 img で保持（再生成なし）、`current` class が移動先 pair（3-4→5-6→7-8→9-10）へ反映
- full/TN rules 変化なし: reader actual pixels `naturalWidth=1740`（objectURL）、status `actual pixels 8/8 | local direct 1/8 | objectURL cache 7/8 | no full 0/8 | TN display fallback disabled`、`/api/image/<hash>?format=avif` 200・`x-cache: cache`（hash 079f07b0…・84360 bytes）
- screenshot（Drive 保存先）: `I:\マイドライブ\kilo\2026-11-03-c06-strip-rebuild-page-move-highlight.png`（fresh tab・strip 8 thumbs・active highlight・status 表示）。Playwright の Drive 直下書き込み不可により workspace 一時ファイル経由でコピー（正典運用の Drive 保存先）・990878 bytes

正典 Common_Operations tests: 120 passed（task_manager validation・正典 tool 変更なし）

CRLF harness 修正（2026-11-04・実行ID assess-20261103-1220・known-issue archive 済み）:
- `tests/strip_rebuild_dom_harness.mjs`: ui.html read 直後に line ending 正規化（`.replace(/\r\n/g, "\n")`）追加。line 161 の exact 照合（TN display source rule）は正規化後内容に対して実行され、check の意味は維持（マッチ除去の弱体化ではない）
- 回帰テスト追加: `tests/test_strip_rebuild.py::test_strip_rebuild_harness_passes_after_crlf_checkout`（CRLF 化 ui.html コピーで harness 実行。全 check 通過＋ TN rule 2 件の PASS を要求。既存 `required` ラベルテストは変更なし）
- 実測（正典 working tree）: harness 単独 18 checks 全通過（修正前後同一 check 数）、`python -m pytest tests/ -v` → 41 passed in 9.40s（回帰テスト 1 件増）。正典 exe SHA256 前後同一（74BE957D11EA96A511C38DEFE1B57CAD899BA58932D78302972EFD6714DD5DFB）。flaky `TestSearchCacheSizeCapBoundsMemory` 単独実行 ok 0.157s（本項目では変更なし）
- clean clone 実測: 単独 commit 後 HEAD・`C:\Users\fujim\AppData\Local\Temp\kilo\c06-crlf-clone` で実施（push 前・結果は実行報告参照）
- `TestEmbeddedUI` 系固定テスト（`app/viewer_test.go`）は本実施で変更・削除なし（意味維持確認: harness 修正のみのスコープ）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
