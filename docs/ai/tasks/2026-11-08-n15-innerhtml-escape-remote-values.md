# N-15: remote 由来値を innerHTML へ無エスケープ（local origin でスクリプト実行リスク）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: リモート由来の値を `innerHTML` に無エスケープで入れている。`viewer.html` で galleryinfo の `entry.hash` を `data-hash='…'` や `src` へそのまま連結。上流が不正な値を返すと `POST /api/fav` などを持つローカル origin でスクリプトが実行される。`Gallery()` で `validHash` に通らない entry を落とすか、クライアントでエスケープする。同様に `search.html` の保存検索の `href` は `encodeURIComponent` が `'` を変換しないため、`'` を含む検索語（タグ名など）で属性が壊れる。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-15 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `escapeHTML` は既にある（`app/viewer.html:230-234`・`app/search.html:183-187`・`&<>"'` を処理）。text content と title 属性では使用済み（`viewer.html:266-269`、`search.html:403/407/410/685` の span 側）。
- 無エスケープで連結しているのは remote 由来値を**単引用符属性**へ入れる行:
  - `viewer.html:991`（reader img の `data-hash` / `data-key` / `data-full` / `src`）、`1237`（strip img）、`814`（`entrySource` の URL path）、`1086`（`querySelector("img[data-hash=\"" + entry.hash + "\"]")` = CSS selector 文字列）
  - `search.html:685`（`href='#/q/' + encodeURIComponent(q.query)`。`encodeURIComponent` は `'` を変換しない）
- 上流: `Client.Gallery()`（`app/hitomi.go:859-883`）は hitomi `galleries/<id>.js` を無検証で `Files` へ写し、`viewer.gallery`（`app/viewer.go:866-873`）も `entry.Hash` をそのまま `/api/gallery` JSON へ出す。client 側に hash 検証はない。正典 `validHash`（`app/viewer.go:302-312`）は proxy/cache 経路（`viewer.go:955`・`1069`、`acquisition.go:159`、`safeCachePath:356`、`safeThumbnailCachePath:367`）でしか効かない → UI 注入経路は塞いでいない。

### 有効性: 有効（実測で閉じた）。原文に2点の訂正

1. 「innerHTML に無エスケープ」は**属性 context のみ**正しい。text content（title・tags・status・meta）は escapeHTML 済み。
2. script 実行の成立条件は `<` ではなく **`'`**（単引用符属性の境界破壊）。`<` だけの値は属性値として保持され、境界は壊れない。`"` は `viewer.html:1086` の selector 文字列を壊す（selector injection）。

実測（node harness 14 checks passed・`docs/ai/tmp/assess-20261108-1332/n15/harness_escape.mjs`）:

- `validHash` case 11: pass 2（実在 64hex・`"abc"`）/ fail 9（`'` payload 38B、`<` payload 28B、onclick payload 58B、`"ABC"`、`"abc%27"`、`"abc def"`、`"../etc/passwd"`、len 2）
- 現行 `renderStrip` 出力 1795 bytes / img 5: `'` を含む entry 1件で属性境界が壊れ、`onerror`・`data-y` が注入属性として出現。`data-hash` 値は `abc` へ切り詰め（=意味変更）
- served 画面（fresh canonical・port 8743・isolated origin `n15-assess-20261108`・window outer 1678x1025 / inner 1665x932）: `window.__n15inj = 1`（onerror が**実行された**）。`#thumbs` img 3件中1件、reader img でも注入属性。escape 相当 sim 後は実行 0・注入属性 0・node 数 3（不変）・`data-hash` 全件 round-trip 一致
- `search.html`: queries 4件中2件（`'` を含む）で href 属性が壊れる。出力 100 bytes・href 値 `#/q/galleries` + 余剰属性 `%20corner'`（bytes 51）、`#/q/don` + `t%20steal'`（bytes 45）。`'` を `%27` へ変換すると属性1個のみ・route round-trip 4件全一致（link 意味不变・bytes 53）

### 実現可能性と推奨修正方法

**推奨（単独実施可・意味変更なし）= client 側 escape**

- `viewer.html`: 属性値の連結を既存 `escapeHTML` へ。URL path 段は `encodeURIComponent` + `'` → `%27`（helper）。`1086` の selector は dataset 参照か selector 値 escape へ。
- `search.html:685`: href 生成に `'` を `%27` へ変換する helper（`encodeURIComponentSingleQuote`）を追加。表示（escapeHTML 済み）と route 意味は不変と実測で確認。

**承認案件として分離**: `Gallery()` で `validHash` に通らない entry を落とす案 = 表示枚数・strip 内容が変わる（ユーザーに見える意味変更）。server 側 gate（proxy・cache path）は既に効いており、UI 側 escape とは独立の論点。

### 副作用・影響範囲・代替案

- escape は表示枚数・link 意味を変えない（実測: node count 3 同一、href `#/g/900000001/page/1` 同一、valid hex の src bytes 同一、`encodeURIComponent` は hex に対して identity）。
- 既存テスト照合（変更なし・通過維持）: `viewer_test.go:1699` の `data-hash='` literal pin は `data-hash='` + escapeHTML(...) 形を維持すれば衝突しない。`2753`/`2828` の escapeHTML pins は既存該当箇所のみ。`4727` の `validHash` 参照は disk cache path テストで、display count を固定していない。`tests/strip_rebuild_dom_harness.mjs` は `escapeHTML` が passthrough stub で、node 数を `data-page` で数える → escape 適用で件数不変。`tests/r12_startup_single_run_harness.mjs:49` は `searchRoute` の自前コピー（literal pin なし）。
- 関連（N-16）: `docs/ai/known-issues/KI-20261106-02-get-host-check-dns-rebinding.md`。Host/Origin 検査は POST のみ（`viewer.go:155-172`、`sameOriginPost:174-194`）。注入 script は served 画面の同一 origin で実行されるため `POST /api/fav`・`/api/read`・`/api/reset` へ到達できる（same-origin を通過する）。GET 側の要求元検査は案 A（承認済み 2026-11-06・`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）で未実施。N-15 の修正は N-16 に依存せず、両者は補完関係。

### 画面確認（合否）

- 合格（before / after 同一条件・同一 window・同一取得方法・実測）。
- Drive 保存先: `I:\マイドライブ\kilo\2026-11-08-n15-innerhtml-escape-remote-values\`（`n15-before-strip-injection-20261108.png`、`n15-after-strip-escape-simulated-20261108.png`、`n15-before-search-href-broken-20261108.png`、`n15-after-search-href-fixed-20261108.png`、`n15-innerhtml-escape-remote-values-confirmation-2026-11-08.txt`）
- before / after は視覚差なし（injection は DOM 層。両組の画像は同一）。表示・link 意味の同一性は実測値で示した。
- 正典 exe からの after 画面は**未確認**（正典 source 未変更。after は page 内 sim の結果）。
- local copy（project root の 4 枚）は Drive 保存後正典 `kilo-safe-remove` でゴミ箱へ。project root 外の残件（削除せず記録）: `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\console-2026-11-08T11-43-37-932Z.log`（13 errors = mock 由来の resource 404 と worker の TypeError。mock 由来で本項目の証拠ではない）。

推定規模: 小（`viewer.html` 属性 escape 5行程度 + `search.html` helper 1 + 任意の新テスト）

## Verification

- `python -m pytest tests/ -v` → 59 passed / 1 failed（10.86s）。不合格は既知 `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約）＝本項目非接触。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean・ok 25.974s。既存テスト・コード変更なし。
- node harness: `node docs/ai/tmp/assess-20261108-1332/n15/harness_escape.mjs` → 14 checks passed（~1s）。既存 harness 2件（`strip_rebuild_dom_harness.mjs`・`r12_startup_single_run_harness.mjs`）通過・衝突なし。
- served 実測: fresh canonical build → `output/hitomi-hub/hub.exe`（pid 23276・port 8743・isolated origin・window outer 1678x1025 / inner 1665x932・実 hitomi API への不正リクエストなし・mock のみ）。
- server 側 `validHash` 実測: `curl "http://127.0.0.1:8743/api/image/abc%27%20onerror%3D%22x%22?format=webp"` → `400 hash must be 64 lowercase hex chars`（remote 呼び出しなし）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 2026-11-09 再調査（assess-20261109-1320）

### 現在の実装と判断根拠（現行正典）

- **escape は未導入**（項目は未解決・有効）。現行 line（シフト後）: `viewer.html` の単引用符属性連結 = `991`（renderReader img: data-hash/data-key/data-full/src）、`1237`（renderStrip img）、`812`/`814`（entrySource の URL path 段）、selector 文字列 = `1086`（reader の `img[data-hash="..."]`）と `1277`（`updateThumbCache` の `img[data-key="..."]`＝前批列挙に無かった同 class の追加点）。`search.html` は `766` の `href='#/q/' + encodeURIComponent(q.query)` が未 escape（display 側 414/421/766 span は escapeHTML 済み）。
- 訂正点1（属性 context のみ）成立: text・title・alt は escapeHTML 済み（正典 230-234/194-198 が `&<>"'` を処理・`'` → `&#39;`）。remote 由来文字列が属性境界へ届くのは上記連結点のみ。`data-gallery`/`data-id` は server 側 int（`viewer.go:1395-1404`・searchCard の id）で数値。
- 訂正点2（実行条件は `'`）成立（実測）。正典 `escapeHTML` は既に `'` を処理するため、 attribute 連結へ適用すれば境界保存。
- **search 側の微修正（前批表現の補足）**: `encodeURIComponent` は `=` と `"` を %3D/%22 へ encode するため、href 経路では script 実行（handler 属性化）が成立しない。成立するのは `'` による属性境界破壊（href 値切り詰め＝link 意味変更）のみ。served 実測: queries 4件中 2件（`'` を含む）で href 値が `#/q/galleries`・`#/q/don` へ切り詰め、残りは attribute name 退化（実行 0）。`'`→`%27` helper で anchor 1属性のみ・route round-trip 4/4（link 意味不变）。
- server 側 `validHash` 現行条件: `app/viewer.go:320-330` = len>=3 lowercase hex（error message「64 lowercase hex chars」と不一致は正典 comment 2026-11-01（318-319）どおり）。効経路は proxy/cache path のみ（`955`系・`1599`・`374`・`385`）。galleryinfo 由来 entry.hash の UI 到達経路は前批どおり無検証: `hitomi.go:899-923`（Files 無検証）→ `viewer.go:1395-1404`（JSON へそのまま）→ client（検証なし）。fresh instance 実測: `GET /api/image/abc%27%20onerror%3D%22x%22` → 400・remote 呼び出しなし（gate は効いているが UI 経路は塞がない）。

### 有効性: 有効（実測で閉じた・前批判定維持・未実施）

- served 実測（fresh canonical・port 8743・isolated origin `n15-assess-20261109`・fresh instance・window outer 1679x1025 / inner 1666x932・外部 API 非呼出・mock のみ）: #thumbs img 3件中1件（payload entry）に注入属性 `onerror`・`data-x` 出現・data-hash 値 `abc` へ切り詰め。reader img 2件中1件でも注入属性。注入 handler が同一 origin で実行（`window.__n15r2 = 1`）。selector context（`"`）は SyntaxError（1086/1277 クラス）。escape sim 後は注入 0・node 3（不変）・属性数 per img 12（境界保存）・data-hash round-trip 3/3 一致・実行 0。
- 注記: served 実測の strip は mock の scene_indexes 1件で stripItems が空になったため（mock 形状・アプリ挙動ではない）、正典 `refreshStrip` を再走させて計測。

### 単独実施可の判定と正典照合

- 現行正典に矛盾なし（2026-11-09 の新規 decision を確認。TN/full 判定・display source・generation guard 不変。escape は表示・link 意味を変えない: node 数同一・href `#/g/900000001/page/N` 同一・valid hex bytes identity）。推奨=client 側 escape 単独実施（前批推奨維持）: 属性連結を既存 `escapeHTML` へ、URL path 段は `encodeURIComponent` + `'`→`%27` helper、selector（1086/1277）は dataset 参照か selector 値 escape。`Gallery()` で entry を落とす案は承認案件として分離維持。
- テスト照合（現行）: `viewer_test.go:1699` の `data-key='`/`data-hash='` literal pin は `data-hash='` + escapeHTML(...) 形と両立。search 側 escapeHTML pins（`2893`/`2968`）は既存該当箇所のみ。`tests/strip_rebuild_dom_harness.mjs:74` は escapeHTML passthrough stub・node 数カウントは data-page/DOM → escape 適用で件数不変。`tests/r12_startup_single_run_harness.mjs` の searchRoute 自前コピーは pin なし。
- 文書矛盾処理: 前批の pytest 記録（1 failed = KI-20261106-04 freshness pointer）は現行で合格（60 passed）。日付優先で現行実測を採用。

### 画面確認（合否）

- 合格（before/after 同一 window・同一取得方法・実測。表示差はないため意味同一性は実測値で提示）。
- Drive 保存先: `I:\マイドライブ\kilo\2026-11-09-n15-innerhtml-escape-remote-values\`（`n15-strip-injection-before-20261109.png`、`n15-strip-escape-simulated-after-20261109.png`、`n15-search-href-broken-before-20261109.png`（element #queries）、`n15-search-href-fixed-after-20261109.png`（element #queries）、`n15-innerhtml-escape-remote-values-confirmation-2026-11-09.txt`）。viewer pair の bytes 同一（49979B・視覚差なし＝injection は DOM 層）。search pair は display text の entity 表示差（7718B/8089B）。
- 汚染記録: 共用 browser で sibling agent の検索（`furry`）が同一 tab の #results に反映（URL hash 变化）。#queries と flags は自 mock 由来で帰属確立。viewer pair は汚染なし。local copy（scratch 4枚）は Drive 保存後 `kilo-safe-remove` でゴミ箱へ。project root 外残件（削除せず記録）: `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\console-2026-11-09T08-19-02-085Z.log`（mock 由来 404 と汚染検索由来エラー。本項目の証拠ではない）。正典 exe からの escape 実装後画面は未確認（正典 source 未変更・after は page 内 sim）。

## Verification（assess-20261109-1320）

- `python -m pytest tests/ -v` → 60 passed（13.81s）。既存テスト・コード変更なし。
- app で `go vet . && go test -count=1 -timeout 300s .` → vet clean・ok 26.701s。
- `node docs/ai/tmp/assess-20261109-1320/harness_escape_recheck.mjs` → 16 checks passed（属性境界・round-trip・search encode 特性）。
- `go run docs/ai/tmp/assess-20261109-1320/validhash_cases.go`（正典写し）→ case 11 = pass 2 / fail 9（前批と一致）。
- server gate 実測: `GET /api/image/<payload>` → 400「hash must be 64 lowercase hex chars」・remote 呼び出しなし（fresh instance・port 8743）。

推定規模: 小（`viewer.html` 属性 escape 5–6行（991/1237/812・814/1086/1277）+ `search.html` helper 1 + helper 適用 1行 + 任意の新テスト）

優先順位付け要素: 重要度 mid（viewer 側は served origin 内 script 実行成立。search 側は実行ではなく link 意味変更のみ。local 専用前提の低優先運用は前批どおり）。緊急度 mid（client 側 escape は単独実施可・小）。リスク low（表示・link 意味不変・テスト pin 両立）。依存なし（N-16（GET Host 検査）とは補完・独立）。entry を落とす案のみ承認案件。

