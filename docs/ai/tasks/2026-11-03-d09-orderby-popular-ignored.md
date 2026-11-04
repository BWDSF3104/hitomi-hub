# D-09: orderby:popular が解析されるが無視される

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: hitomi 正典準拠確認（実測・順序不同確定）→ dataset 選択 mirror 実装・テスト固定・検証完了。単独 commit + push（本項目分）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/searchhtml.go`（`parseTerms`）は `orderby` を `state.orderby`（`popular` 等）へ入れるが、`applySearchOrder`（178-191）は `state.orderbydir`（asc/desc）しか適用しない。`orderby:popular` は集合順序を変えない。hitomi 準拠（results.js の sort）は未確認 → 判定は hitomi 準拠確認後に確定。対応: hitomi 準拠確認後に sort を実装 or parse から除去。推定規模: 小〜中。

### 実施前確定（正典ソース確認 2026-11-04・assess-20261103-1220）

A-04 で取得済みの正典 live JS（`docs/ai/tmp/assess-20261103-1220/scratch_results.js`・`scratch_search.js`）の orderby 機構確認:

- `results.js`（89-95行）: `orderbykey` が undefined の場合既定 key を `orderby === 'popular'` → `year`、else → `added` と確定。`order_results`（97-110行）は方向しか適用しない（asc/ascending → reverse、rand/random → shuffle）。popular 用の client 側 sort は存在しない。
- `search.js`（612-624行）`nozomi_address_from_state`: `state.orderby !== 'date' || state.orderbykey === 'published'` のとき nozomi URL に `orderby`/`orderbykey` 分区を挿入（area 指定: `/n/<area>/<orderby>/<key>/<tag>-<lang>.nozomi`、area 'all': `/n/<orderby>/<key>-<lang>.nozomi`）。集合順序は「key で選択された dataset 自身の順序」で変わる。`compressed_nozomi_prefix = 'n'`（searchlib.js 14行・アプリ既定 `/n/` と一致）。
- `results.js`（114行）plain term 初期分岐: 最初の positive term が plain で `orderbykey !== 'added'` なら初期 order は order dataset（`get_galleryids_from_nozomi(state)`）から。term membership は後の positive filter で intersection（order dataset の順序保持）。

判定: hitomi の `orderby:popular` は集合順序を変える（dataset 選択経由。client sort ではない）。「applySearchOrder に sort 実装」は data なしでは faithful 不可、「parse から除去」は hitomi 挙動と矛盾。→ 最小実装 = `nozomi_address_from_state` の mirror（path 分区 + cache key）＋ orderbykey 既定確定（popular→year/date→added）＋ plain term 初期 order dataset 分岐 mirror。`applySearchOrder`（方向）は不変 → A-3 合格条件（二重 reverse なし・asc 単独 reverse）維持。

合格条件（実装前確定）:
1. 実測: `n/tag/popular/year/female:yuri-all.nozomi` と `n/popular/year-all.nozomi` が 200 で、date-added dataset と集合同一・順序不同（または順序差の実測根拠）
2. `app/searchhtml.go` 内 mirror（SearchNozomiParts path/cache key・termIDs/plain term 初期分岐・orderbykey 既定）を最小実装。A-3・C-04・A-04 既存テスト不変通過
3. 挙動固定テスト追加: `female:yuri orderby:popular` の fetch path（order 分区）と返す順序、plain term + popular の order dataset 起点、asc 方向併用で reverse 1 回
4. 検証: `app` `go vet . && go test -count=1 -timeout 300s .`、root `python -m pytest tests/ -v`（freshness 含む）、canonical exe 再 build。既知 flaky LRU tie（known-issue 2026-11-04）は suite 全体 1 回失敗まで許容（単独実行 PASS 条件）

### hitomi 正典準拠確認（実測 2026-11-04・assess-20261103-1220）

正典 live JS（A-04 取得済み `scratch_results.js`・`scratch_search.js`）で機構確定済み（上記）。live 実測（`docs/ai/tmp/assess-20261103-1220/d09_live_order_check.py`・呼出し 4・gate 1000ms 維持・`d09_live_observation.txt`）:

- `n/tag/female:yuri-all.nozomi`（date/added・既定）: 200・58521 件・first5 `[4302790, 4302689, 4302580, 4302325, 4302227]`（date-added 順）
- `n/tag/popular/year/female:yuri-all.nozomi`（orderby:popular 既定 key 'year'）: 200・58483 件・first5 `[3718021, 3680247, 3786575, 3939320, 4044867]`（date 順ではない）。順序不同 → `orderby:popular` は hitomi で集合順序を変える（dataset 選択経由）
- `n/popular/year-all.nozomi`（results.js 114 plain term 初期分岐の area 'all' 形）: 206・first8 `[3840444, 3758797, 2373966, 3765219, 3830033, 3737417, 3703112, 3757937]` ≠ `n/index-all.nozomi` first8（`4303467, 4303464, ...`・date-added 順）→ 順序不同を確認

判定: hitomi 正典準拠 = `orderby:popular` は結果の順序を変える（client sort ではなく `nozomi_address_from_state` の dataset 選択。`order_results` は方向のみ）。アプリは現挙動（orderby 無視）が正典と不一致 → hitomi 挙動へ寄せる最小実装 = 正典機構の mirror（sort 実装でも parse 除去でもない。データなしで popular 順序を client で生成は不可のため、正典と同じ dataset 選択で実装）。

未確認点（記録）: popular dataset と date dataset の件数が微差（58483 vs 58521）。dataset 間の集合同一性は未確認（順序判定には影響しない）。or-only query の初期 order（正典は area 'all' dataset 起点・アプリは union 順序）は C-06/C-07/C-09 確定済み既存挙動で本項目非触・変更なし。

### 変更内容（最小・正典 mirror）

- `app/searchhtml.go`（D-09・parseTerms/applySearchOrder 関連の最小変更。A-3 の方向処理は不変）
  - `SearchNozomiParts(state *searchState)` へ変更（呼び出し元は `termIDs` のみ・既存 tests 呼び出し形式更新）。`nozomi_address_from_state` mirror: `orderby != 'date' || orderbykey == 'published'` で order 分区（area 指定 `/n/<area>/<orderby>/<key>/<tag>-<lang>.nozomi`、area 'all' `/n/<orderby>/<key>-<lang>.nozomi`）。dataset 選択が変わるため cache key にも order を追加（date dataset と混合しない。既定 path/key は既存挙動そのまま）
  - `resolvedOrder`: results.js 89-95 mirror（orderbykey 既定 popular→'year'・else 'added'）。SearchHTML で fetch 前に確定
  - SearchHTML 初期分岐: results.js 114 mirror — 最初の positive term が plain で `orderbykey != 'added'` なら order dataset を順序起点とし、term membership は後段 intersection で保持（positiveStart 調整）
  - `termIDs` language 分岐は C-04 の自前 dataset（index-<lang>）意味を維持（state copy）。language-first + order 時は正典と同じ `/n/popular/year-<lang>.nozomi` 形になる
- `app/searchhtml_test.go`: `TestSearchHTMLPopularOrderFetchesOrderDatasetPerCanonicalHitomi`・`TestSearchHTMLPlainTermOrderStartsFromOrderDatasetPerCanonicalHitomi`・`TestSearchHTMLPopularOrderAscReversesOrderDatasetOncePerCanonicalHitomi` 追加（A-3 二重 reverse なし条件の固定を含む）。既存 `TestApplySearchOrderDoesNotDoubleReverseAsc`・A-04/C-04 tests 不変通過
- スクラッチ: `docs/ai/tmp/assess-20261103-1220/d09_live_order_check.py`・`d09_live_observation.txt`（gitignore）

矛盾確認: A-3（`applyOrderForInitial`・二重 reverse 修正）維持 — direction は order dataset にも 1 回だけ適用（テスト固定）。`orderbydirection`（asc/desc）現挙動不変。C-04 二重 fetch 排除不変（order dataset は別 key・1 回 fetch）。A-04 plain term lang 無視不変（membership は index hash 降下のまま、順序のみ正典 mirror）。

## Verification

- 実測根拠: 呼出し 4・gate 1000ms 維持（`d09_live_order_check.py`・`d09_live_observation.txt`）。popular dataset 200＋順序不同確認（tag 形 first5 一致せず・area 'all' 形 first8 一致せず）。判定確定
- `app` `go vet .` clean。`go test -count=1 -timeout 300s .` → ok 20.822s（1 回目から全 test 通過。既知 flaky LRU tie 発火なし。追加 3 test・既存 A-3/A-04/C-04 tests 通過）
- root `python -m pytest tests/ -v` → 40 passed（8.90s。freshness 含む）
- canonical exe 再 build 完了（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）。UI 非関連項目のため画面操作確認は該当なし（canonical server 非起動）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
