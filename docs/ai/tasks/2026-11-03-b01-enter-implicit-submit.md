# B-01: 検索ボックス Enter で前ページが実行される

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測・機構 уточ）。`app/ui.html` の `#search` 内 button に `type` 属性がなく 3 つすべて submit（`search-prev`/`search-next`/`search-submit`、tree order 首位 = `search-prev`）。page 1 では `search-prev` が disabled → implicit submission の default button がないため Enter で検索実行されない（実測: Enter 後 network request なし、`#page` 1 のまま）。page ≥2 では `search-prev` が enabled → Enter で `setSearchPage(-1)` が走し、入力した語で「前ページ」へ移動（実測: page 2 で `female:catgirl` 入力 + Enter → `#/q/female%3Acatgirl/page/1`）。スクショ: `docs/ai/tmp/assess-20261103-1220/02-b01-enter-implicit-submit.png`（Drive へコピー済み）。対応: `#search-submit` に `type="submit"`、prev/next に `type="button"`。合格条件: 1280x900 fresh tab で Enter が current page の語で検索を実行し、prev/next/saved-search の既存動作（setSearchPage）が壊れない。推定規模: 小。

実施確定（2026-11-03 実装前): 修正方法は `type` 属性付与のみ（UI ロジック変更なし）。`form#search`: `#search-submit` に `type="submit"`、`search-prev`/`search-next` に `type="button"`。`form#nav`: `prev`/`next`/`complete`/`fav` に `type="button"`。承認範囲括弧書きの「nav の submit button」は実コードに submit button が存在しないため該当なし（新規追加しない。nav は submit button なしで、`#pagenum` の Enter は submit イベント（handler: preventDefault のみ）に固定される）。影響範囲: Enter の暗黙 submit の default button 決定のみ。click ハンドラ（`setSearchPage`、`movePage`、`pagenum` change、fav/complete）は変更なし。合格条件は上と同文。検証方法: 正典 UI confirmation procedure（go vet/test → canonical exe 再 build → server 再起動 → Playwright 1280x900 fresh tab 実操作、network request 実測 → スクリーンショット Drive 保存）+ `app/viewer_test.go` に合格条件固定テスト追加。saved-search の live 書き込みテストは canonical DB にテストデータを書くため実施せず、handler 静的固定と submit イベント発火実測で確認する。

## Verification

静的テスト（合格条件固定）: `app/viewer_test.go` に `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit` 追加。実測: `TestEmbeddedUIHasSearchPrevNextButtons`（既存）PASS・新テスト PASS。`go vet .` clean。`go test -count=1 -timeout 300s .`（app）→ ok 36.930s。

canonical exe 再 build（UI 変更につき必須）: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`。稼働中 server（PID 9160）停止 → 再 build exe の `hub.exe view` で既定 `http://127.0.0.1:8731` 起動（UI 確認後、同一 exe で永続再起動済み・port 8731 listening 確認）。

UI 実操作確認（fresh tab・viewport 1280x900・server 提供 UI・network 実測）:
- DOM 型実測: `#search` = `search-prev`/`search-next` `type="button"`, `#search-submit` `type="submit"`（唯一の submit）。`#nav` = prev/next/complete/fav すべて `type="button"`（submit button なし）。`saved-search` の save は submit のまま（変更なし）。
- `#query` に `female:catgirl`・`#page` に `2`・`#query` 上で Enter → form#search の submit が暗黙 submit（default button = `search-submit`）で発火、`runSearch` 実行。network 実測: `GET /api/search?query=female%3Acatgirl&lang=english&page=2&limit=25` → 200。`#page` は 2 のまま（prev の暗黙発火なし: prev click 0、hash `#/q/female%3Acatgirl/page/2`）。画面: `page 2 / 139・total 3458`、検索完了・カード表示済み 25件、`3458 件・page 2`。
- prev/next 既存動作（`setSearchPage`）非破壊: `#search-next` クリック → network `page=3`（200・hash page/3）、`#search-prev` クリック → network `page=2`（200・hash 2 に復帰）。click 各1回計測。
- saved-search: `#qname` 上 Enter で form submit 発火（handler 実行、空値で early return → `POST /api/query` なし＝DB 書き込みなし）。既存挙動維持（live write は canonical DB にテストデータを残さないためイベント発火実測で確認、静的テストで handler 固定）。
- nav Enter: `#pagenum` 上 Enter は submit イベントのみ発火（handler: preventDefault）。nav に submit button がないため暗黙 default button（従来 prev）の click は発生せず。nav の click ハンドラ（prev/next/complete/fav・pagenum change）は変更なし（静的テスト固定）。nav の Enter 挙動は従来「prev 暗黙発火」→「submit event のみ（no-op）」へ変化（承認範囲の目的どおり。要判断点として報告）。

スクリーンショット: `docs/ai/tmp/assess-20261103-1220/b01-confirm-1280x900-enter-page2.png`（179,477 bytes、検索フォーム female:catgirl・page 2・perf: /api/search 763ms・3458 件・page 2 を表示）→ Drive `I:\マイドライブ\kilo\b01-confirm-1280x900-enter-page2.png` 保存。

ルート pytest: `python -m pytest tests/ -v` → 33 passed（3.86s、`test_canonical_build_freshness.py` 含む＝exe 現行 ui.html バイト一致）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
