# N-04: 見開きペアで 2 枚目の横長を判定していない

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査完了・実施未。2枚目 aspect 判定は本体単独実施可。page 番号・route・hash・既読 step・固定寸法契約は承認サブ分

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: 見開きのペアで 2 枚目の横長を判定していない（`readerItems`）。`isAlreadyDoublePage` を見るのは 1 枚目だけ。縦ページの次が横長だと横長画像が半幅のセルへ押し込まれる。2 枚目が横長なら 1 枚目を単独表示にすべき。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-04 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `app/viewer.html:502-504` `isAlreadyDoublePage(entry)` = `Number(entry.width) > Number(entry.height)`（strict。square は landscape 扱いしない）。
- `app/viewer.html:524-540` `readerItems()` は first（`current.page`）だけ `isAlreadyDoublePage` で見て、second を
  `currentPairEnd` > `current.page` → `items[firstIdx+1]`（scene mode では次の scene start、list 末尾なら `entryForPage(page+1)`）→
  なければ `entryForPage(page+1)` の順で選ぶ。**second の width/height は見ていない。**
- `app/viewer.html:506-511` `displayModeFor(firstEntry)`、`app/viewer.html:515-520` `readerClass(firstEntry)` は
  1 枚目の entry だけで `single landscape` を決める。`renderReader()`（`app/viewer.html:973`）は `readerClass(items[0])` を渡す。
- CSS（`app/viewer.html:91-109`）: `--reader-page-width: min(45vw, 500px)` / `--reader-page-height: min(78vh, 700px)`、
  `.reader.double` は `grid-template-columns: repeat(2, minmax(0, var(--reader-page-width)))`（半幅セル 2 列）、
  `.reader.landscape` だけ幅が `min(92vw, height*1.6)` の単一列。second が横長でもクラスは `reader double` のまま
  → 横長画像は 500px 幅の半幅セルに `object-fit: contain` で収まる（余白は上下）。
- 実測（served 画面・window 1665x932・gallery 1727103）: `grid-template-columns: 500px 500px`、page 93（2880x1620）の
  描画は **500x281**。同一画像を単独表示（`reader single landscape`）すると **1120x630** → 幅 44.6%、面積 19.9%。
- navigation 実測: RTL既定で ArrowLeft（forward）は `items[1].page + 1` → `92-93` から **94-95** へ飛ぶ。横長 page 93 は
  forward navigation 中に単独 page として一度も表示されず、半幅セルでしか表示されない。

### 有効性

**有効**（読み取り指摘は実測で確認。判断保留ではない）。

- 正典 decision `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、
  `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、
  `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md` を読了（作業前チェック）。
- 既知 issue の解決記録 `docs/ai/known-issues/archive/2026-11-01-landscape-entry-forced-double.md` と
  候補方針（`docs/ai/tasks/archive/2026-11-01-strip-group-and-landscape-double.md`）は「横長 entry は
  already-double/single full-width 扱い・`viewMode=double` は portrait pairing に限定」という方針を持つ。
  実装は 1 枚目だけ反映し、2 枚目は半幅セルへ入れる点で**この方針と不一致**。よって指摘は有効。
- 正典文字列（Glob で実体確認）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、
  `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md` は存在し、TN を reader/thumbs の表示源に
  しない・TN display fallback 禁止を維持。修正案は表示源（objectURL / local direct）を変えないため正典 condition に触れない。
- 矛盾する状態記符は検出せず（採用文書: 上記 decision と archived known-issue。日付の新しい `2026-11-01` の
  decision/known-issue が現行実装の意図と一致）。

### 実現可能性と適切な修正方法（推奨）

推奨（案A・既存方針に沿う最小修正）: `readerItems()` の second 追加条件に 2 枚目の aspect 判定を加える。

```js
if (doubleActive() && first.entry && !isAlreadyDoublePage(first.entry)) {
  ... second を選ぶ ...
  if (second && second.entry && !isAlreadyDoublePage(second.entry)) result.push(second);
}
```

これに伴い `readerClass()`/`displayModeFor()` は「pairable な second が 1 枚もない」場合に単独表示へ変わる必要があり、
現状 `readerClass(items[0])`（1 枚目だけ）では `reader double` のまま 2 列グリッドの 1 列目に単独画像が入り、
右側に空白セルが残る。よって pair 組数（items.length）を反映する判定が必要。

数値（同一 window 1665x932・gallery 1727103・page 92 portrait / page 93 landscape）:

| 状態 | reader class | cell 数 | cell 幅 | page 92 描画 | page 93 描画 | position | route | read lastShown | movePage(+) |
|---|---|---|---|---|---|---|---|---|---|
| 現行 | `reader double` | 2 | 500px | 493x700 | **500x281** | `92-93` | `#/g/1727103/page/92-93` | 93 | 94（93 を単独表示しない） |
| 案A（推奨） | `reader single` | 1 | 500px | 493x700 | （次段で単独 1120x630） | `92` | `#/g/1727103/page/92` | 92 | **93**（単独横長 1120x630） |

回帰（portrait→portrait）は変化なし: `reader double`・cells 2・`4-5`・lastShown 5（harness case3、実測テスト
`readerOk` 2/2 と一致）。

### 副作用・影響範囲・代替案

- 影響ファイル: `app/viewer.html`（`readerItems`、`displayModeFor`/`readerClass`）、`app/viewer_test.go`（アサーション）。
  server 側（`app/viewer.go` の API・cache・`x-cache`）は影響なし。表示源・TN condition・generation guard は不変。
- **ユーザーに見える意味変更（承認が必要・実装案とは分離）**:
  1. ページ番号表示と route: `92-93` 表示が `92` 単独になり、`movePage` の step が 2 → 1（pair 単位でなくなると
     前進が 1 ページ刻みになる）。
  2. 明示 pair hash（`#/g/1727103/page/92-93`）の意味: 現行は `currentPairEnd` を無条件で尊重する（実測 case2）。
     案A は横長 second を pair しないため、明示指定时无効化される（`display` は 92 単独）。既存 decision
     `docs/ai/decisions/2026-11-01-p3-implementation-decisions.md`（#18）と `docs/ai/backlog-items/p3-18-spread-rtl-toggle.md` は
     route/hash の logical order `page-second` を「変更しない」と言う。pair を成立させると横長が半幅セルに
     戻る（指摘の状態）ので、**明示 hash をどこまで尊重するかは承認が必要**。
  3. 既読 progress の `lastShown`（`runBackgroundRead`・`complete` 判定、`app/viewer.html:1137`）が step 単位で変わる
     （92 単独時は lastShown 92）。DB 書き込みの粒度＝ユーザーに見える意味。
- 代替案B（意味変更を避ける案）: pair を維持したまま横長 entry だけ幅を広げる（非対称 grid columns）。
  ただし正典 decision `docs/ai/decisions/2026-11-01-reader-fixed-size-no-gap-placeholder-complete.md` の
  「double page items は固定 `min(45vw, 500px)` x `min(78vh, 700px)`・gap 0・placeholder と同一 box」に反する
  （1120px 幅は double cell 契約外）→ **decision 変更（承認）が必要**。window が狭い場合は
  `100%` で頭打ち（実測 container 1633px）ため、固定寸法契約の再定義になる。
- 代替案C: pair するが second が横長のとき first を単独・second を単独の順で「1 枚ずつ」表示（案A と同じ帰結）。
- 推奨は案A（既存方針 `entry.width > entry.height` を単独 full-width 扱いとする決定の延長で、意味変更は
  ページ番号・明示 hash・既読粒度の 3 点に限られる）。

### 画面確認（UI 項目）

- 正典 exe（fresh build）+ served 画面、cold hygiene 3条件（fresh canonical build・fresh server instance（pid 6500・
  08:19Z 起動）・isolated origin（`isolatedContext: n04-assess-20261108`））、window size 基準（actual window 1665x932、
  人工 resize なし、content viewport `#reader` 1618px）。
- before（現行: pair `92-93`・半幅セルで横長が 500x281）と after（同一画像の単独表示 1120x630）を同一条件で取得。
- 保存先: `I:\マイドライブ\kilo\2026-11-08-n04-reader-second-page-landscape\`
  - `n04-1727103-page92-93-double-before-20261108.png`
  - `n04-1727103-page93-single-landscape-after-20261108.png`
  - `n04-reader-second-page-landscape-confirmation-2026-11-08.txt`
- local copy（project root の PNG 2 枚）は正典 `kilo-safe-remove` でゴミ箱へ送った。Drive 側は保持。
- **修正後表示（1 枚目単独表示）の served 画面は未確認**（正典 source を変更できないため、数値はスクラッチ
  harness の算出で示す）。served 画面で確認できたのは現行状態（before）と同一画像の単独表示（after）まで。
- console error は `favicon.ico` 404 のみ。表示源は objectURL（`naturalWidth > 0`）、`x-cache: cache`（357792 B / 143547 B）、
  TN display fallback 使用なし。外部 gallery API は呼んでいない（既存 `cache/galleryinfo` + `cache/images` の fixture で足りました）。

### 既存テストによる現行意味の固定（照合）

- `app/viewer_test.go:2570` と `:2588`（`TestEmbeddedUIHasReaderSpreadAndFullscreen`）は exact substring
  `if (doubleActive() && first.entry && !isAlreadyDoublePage(first.entry))` を要求 → **2 枚目の aspect 判定を
  加えるとこのアサーションは壊れる**（実装時に 2 箇所更新が必要）。
- `app/viewer_test.go:2567` は `readerClass(items[0])` を要求 → readerClass が pair 組数を見ない実装を固定する。
  `readerClass` 内部で pair 組数を見れば substring は維持できるが、1 枚目の entry だけでクラスを決める現行意味を
  テストが固定している。
- `app/viewer_test.go:2555`（`isAlreadyDoublePage` の `Number(entry.width) > Number(entry.height)`）、
  `:2558`（`return "reader single landscape"`）は方針側と整合（修正でも維持）。
- `app/viewer_test.go:2127`（`lastShown = doubleActive() ? ... : current.page`）、`:2178`
  （`currentPairEnd = doubleActive() && displayed.length > 1 ? displayed[1].page : null`）は form を固定するだけで
  案A で form は変わる必要がない（length 1 のとき null）。**動作（既読 step）は変わる**。
- Python harness（`tests/read_lists_debounce_harness.mjs`、`tests/strip_rebuild_dom_harness.mjs`）は `readerItems()` を
  自前 stub しているだけで、2 枚目の aspect 意味を固定していない。既存テストは変更していない。

## Verification

- `python -m pytest tests/ -v` → **1 failed / 59 passed in 15.47s**。failed は既知の非接触項目
  `test_canonical_build_freshness_rule_exists`（AGENTS.md ポインタ条）。本項目は非接触。
- `python -m pytest tests/test_canonical_build_freshness.py -v` → 1 failed（同上）/ **1 passed**
  （`test_canonical_executable_embeds_current_pages`）in 2.61s。canonical exe が現行 `app/viewer.html` の exact bytes を
  埋め込むことを確認（正典 build command: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → passed）。
- `go vet .`（app）→ output なし（passed）。`go test -count=1 -timeout 300s .`（app）→ `ok hitomi-hub/app 25.855s`。
- node harness（`docs/ai/tmp/assess-20261108-1332/n04_pair_measure.mjs`、`tests/harness_lib.mjs` の `bodyIn` で
  `function doubleActive()`〜`function fullscreenTarget()` を抽出し `new Function` で実行）→ 現行 = `reader double` / cells 2 /
  cellW 500px、修正案 = `reader single` / cells 1。数値は上表（window 1665x932）。経過時間: 実行即（<2s）。
- 実機（served 画面）実測値: before `500x281` / after `1120x630`、grid `500px 500px`、window `1665x932`、
  `x-cache: cache`、`naturalWidth > 0`、TN fallback 禁止維持。
- 実在 gallery 照合: `docs/ai/tmp/assess-20261108-1332/find_adjacency.py`（cache/galleryinfo 400 件で縦→横 adjacency）、
  `find_both_local.py`（both-local 0 件）、`find_cached_pair.py`（persistent full cache 双成立 6 件 → 1727103 を採用）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（実装完了時に。本調査は Status/Next Action を変更していない）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）— 本項目は新しい
  known-issues を作っていない。関連する解決済み記録は
  `docs/ai/known-issues/archive/2026-11-01-landscape-entry-forced-double.md`（1 枚目限定の解決までで、2 枚目は未解決）
- [ ] `backlog.md` の完了項目を削除（該当する場合）— 本項目は backlog 項目ではない（外部指摘 N-04）
