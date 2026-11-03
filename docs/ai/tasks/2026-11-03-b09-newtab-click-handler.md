# B-09: newtab=1 が実質的に効かない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/ui.html:201` の `newTabMode` は `targetAttr()`（`app/ui.html:250-252`）で anchor に `target='_blank'` を付けるが、href は同一ドキュメント fragment（`#/g/<id>/page/1`）。HTML spec では same-document fragment link の `target` は無視される。実測（`?newtab=1` で card anchor click）: tab 数 3 のまま（新 tab 増えず、同じ tab の hash が `#/g/4300588/page/1-2` へ変化）。DOM には `target="_blank"` 属性が存在。対応: newTabMode では absolute URL（`window.location.pathname + href`）を使うか、click handler で `window.open`。合格条件: `?newtab=1` で card click が新 tab を開く（tab 数 +1）。推定規模: 小。

実装前確認（assess-20261103-1220・B-09 実施）:
- 記録と現ファイルの対応: `newTabMode` は `app/ui.html:201`（一致）。`targetAttr()` は現ファイル `app/ui.html:255-257`（記録の 250-252 は行番号のずれ。機能内容の食い違いなし）。
- 追加事実（handler）: card click の delegated handler（`#results`・`#thumbs`・現 `app/ui.html:1601-1612`）が修飾キーの有無かわらず `e.preventDefault()` を無条件で実行し、`openGallery`（same-tab 表示）を呼ぶ。これが修飾キー click の native 新 tab 動作を塞いでいる。記録の「same tab の hash へ変化」はこの handler の結果と整合。
- 承認済み方針（2026-11-03）: 修飾キー click は handler で `preventDefault` せず native へ。plain click は newTabMode のアプリ動作（新 tab 起動）維持。newTabMode なし plain click の既存 same-document hash 表示（`#/g/<id>/page/1`）は維持（回帰）。

確定した修正方法（変更は承認範囲内: `newTabMode`、`targetAttr()` 関連、card click handler、固定テスト）:
1. helper `newTabHref(hash)`: newTabMode のとき `window.location.pathname + window.location.search + hash`（absolute URL）、そうでなければ hash をそのまま返す。`targetAttr()` 自体は変更しない。
2. card click handler（`#results`・`#thumbs`）を `handleCardClick(e)` へ統一:
   - `e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey` の early return（`preventDefault` せず native 動作へ）
   - plain click・newTabMode: `window.open(newTabHref(route(id, page)), "_blank")`（tab +1）
   - plain click・non-newTabMode: 既存の `await openGallery(id, page)`（same-document hash 表示）維持
3. 合格条件固定テスト: `app/viewer_test.go` `TestEmbeddedUIHasNewTabQueryPolicy` に追加（handler の修飾キー early return、`window.open(newTabHref(...))`、non-newTabMode plain click の `openGallery` 維持、`newTabHref` の absolute URL）。実操作（Playwright・canonical server）で tab 数・href・`target` 属性を実測し、画面キャプチャを Drive（`I:\マイドライブ\kilo\2026-11-03`）へ保存。

作業前チェック（AGENTS.md 必須・ファイル名明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を読了。本修正は imageProxy 順序・永続先・shard URL・gg・TN 表示源・full 判定（x-cache・no-store fetch・objectURL・generation guard）に触れない（UI の card click ルーティングのみ。`showPage()`・reader・cache 未変更）。

実施（2026-11-03・サブエージェント・assess-20261103-1220）:
- `app/ui.html`: `newTabHref(hash)` 追加（newTabMode のとき `window.location.pathname + window.location.search + hash` を返す absolute URL helper。`targetAttr()` は未変更）。card click handler を共有 `handleCardClick` へ統一（`#results`・`#thumbs`）: `e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey` で early return（preventDefault せず native 動作へ）。plain click・newTabMode: `window.open(newTabHref(route(id, page)), "_blank")`。plain click・non-newTabMode: 既存の `await openGallery(id, page)`（same-document hash 表示）維持。B-01 form submit・B-03 cache read_state・viewer・imageProxy・TN・永続 cache・objectURL LRU・DB 書き込み口・config・download・YARA は未変更。
- 合格条件固定テスト: `app/viewer_test.go` `TestEmbeddedUIHasNewTabQueryPolicy` に追加（`newTabHref` の absolute URL、修飾キー early return が `preventDefault()` より前、newTabMode 分岐の `window.open(newTabHref(route(id, page)), "_blank")`、non-newTabMode plain click の `openGallery` 維持と handler の `#results`・`#thumbs` wiring）。既存断言は維持。

## Verification

1. `app` で `go vet .`（クリーン）・`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 38.424s`。個別 `TestEmbeddedUIHasNewTabQueryPolicy` PASS（0.153s）。
2. canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（ui.html バイト埋込）。ルート `python -m pytest tests/ -v` → 33 passed（3.16s、`test_canonical_build_freshness.py` 含む）。
3. canonical server（再 build exe・既定 `http://127.0.0.1:8731`・runtime root は scratch（`scratch/b09`）・canonical DB 未書込）起動。server baseline 直確認: `GET /api/lists` → 200、`GET /api/search?query=furry&lang=all&page=1&limit=25` → 200・total 32894・ids 25・X-Search-Timing `search=0;record=853`（stale response なし・`id="query"` duplicate なし・resize 前の実測）。Playwright（fresh tab。localStorage の stale search state を clear 後 reload で排除）で実操作:
   - Scenario A（plain click・`?newtab=1`）: fresh tab で実検索（cards 25 = server ids.length 25・first card id 4301574）。1枚目 card click → tab 数 1→2（+1）。card anchor 実測 `target="_blank"`・href `#/g/4301574/page/1`。新 tab URL `http://127.0.0.1:8731/?newtab=1#/g/4301574/page/1`（`?newtab=1` 維持）。元 tab は `?newtab=1#/q/furry/page/1` のまま。新 tab reader actual pixels naturalWidth=1713（blob objectURL）。合格条件（tab +1）満たす。
   - Scenario B（修飾キー click・`?newtab=1`）: 2枚目 card を Ctrl+click。click event 実測 `defaultPrevented=false`（handler 修飾キー early return・preventDefault なし）→ native で tab 数 2→3（+1）。新 tab URL `http://127.0.0.1:8731/?newtab=1#/g/4301527/page/1-2`（native 解決で `?newtab=1` 維持）。元 tab hash 不変。
   - Scenario C（回帰・newTabMode なし）: fresh tab `/` で実検索（cards 25・server と一致・anchor `target` 属性なし）。1枚目 card plain click → tab 数増えず（4 のまま）、same-tab hash `#/g/4301574/page/1`（reader actual pixels naturalWidth=1713）。既存 same-document hash 表示維持。
4. 画面キャプチャ（Playwright・output root から Drive へコピー）:
   - `I:\マイドライブ\kilo\2026-11-03\b09-scenarioA-newtab-mode-search-tab-20261103.png`
   - `I:\マイドライブ\kilo\2026-11-03\b09-scenarioA-newtab-opened-by-plain-click-20261103.png`
   - `I:\マイドライブ\kilo\2026-11-03\b09-scenarioB-ctrl-click-newtab-20261103.png`
   - `I:\マイドライブ\kilo\2026-11-03\b09-scenarioC-same-tab-regression-20261103.png`
   - 手順テキスト: `I:\マイドライブ\kilo\2026-11-03\b09-newtab-click-handler-confirmation-20261103.txt`
   実測後 server 停止（本タスク開始時点で canonical server 未稼働だったため停止して開始時の状態へ復した）。
5. 既合格項目への影響: なし（B-01 `c1d2c07`・B-02 `622fadc`・B-03 `d010178`・B-05 `574a659`・A-15 `3fecdf8`・A-07b `fa10536`・C-08 `1b8bb30`）。card click・`newTabHref`・`targetAttr()` は form submit（B-01）、cache read_state（B-03）、viewer・imageProxy・TN・objectURL LRU・DB 書き込み口と接しない。全 go test（38.424s）・pytest 33 passed で回帰確認。A-10 の未コミット変更（`app/main.go`・`app/hitomi_test.go`）には触れていない。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager.py end` で B-09 行のみ削除実施。同一ファイルに A-10 の未コミット変更があるため、B-09 commit には `current-task.tsv` を含めない — commit は `app/ui.html`・`app/viewer_test.go`・本タスク記録のみ。tsv の行削除は A-10 側の commit 以降で反映）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）— 該当なし（B-09 の known-issues ファイルなし）
- [ ] `backlog.md` の完了項目を削除（該当する場合）— 該当なし（B-09 は assess-20261103-1220 の assessment 項目。backlog の行は親側の記録のため本項目で変更しない）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
