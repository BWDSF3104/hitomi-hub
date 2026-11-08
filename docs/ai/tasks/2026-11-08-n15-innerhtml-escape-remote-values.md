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
