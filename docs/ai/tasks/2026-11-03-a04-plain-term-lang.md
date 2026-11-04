# A-04: plain term に lang が効かない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 単独 commit + push（本項目分）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

機構は事実。`app/searchhtml.go:123` の plain term 分岐は `c.IndexIDs("galleries", term)` を呼び、`state.language` を使わない（nozomi 分岐のみ language 反映）。影響: `?query=furry&lang=english` で plain term は言語条件なしの集合になる。実測（client 累積メトリクス）plain term `furry` で index calls 7・gate_wait 6043ms。hitomi の galleriesindex が言語キーを持つかの正典確認が必要なので、判定は「hitomi 準拠確認待ち」。対応方針: 確認後に lang 適用（nozomi 併用）または lang 無視を明示。推定規模: 中。

### hitomi 正典準拠確認（実測 2026-11-04・assess-20261103-1220）

正典コード観察（hitomi サーバ live 応答。ltn.gold-usergeneratedcontent.net・gate 1000ms・呼出しは search.html 1 + JS 3）:

- `search.js`（31226 bytes）`get_galleryids_for_query`: colon term は state を set して nozomi（`nozomi_address_from_state` の `[tag, language]` join で lang 反映）。plain term 分岐は `key = hash_term(query)`・`field = 'galleries'` の B-tree 降下のみで `state.language` を使わない（search.js 674-691行）
- `results.js`（12362 bytes）`do_search`: lang URL parameter は存在せず、`state.language` 既定 'all'。言語反映は `language:` term のみ（その term が自前 dataset `index-<lang>` として nozomi fetch）。`languagesindex` は suggestions 専用（`ns === 'language'` → URL `/index-<tagname>`）で plain term のキー構造には関与しない
- live 応答差分（同一 plain term `furry`・言語キー有無の判定）: galleriesindex version `1793760810`・key `hash_term('furry')` = `a1d72c62`（2026-11-01 記録と一致・key に言語成分なし）→ dataset count **32901**。`n/index-english.nozomi` dataset **170609** 件、intersection **5952**、index-english 外 **26949** 件（sample: 4302745, 4302444, 4302149, 4302014, 4301927）。furry 集合の大部分が非 English を含む → `galleries` dataset は言語キー・言語スコープを持たない。hitomi UI `q=furry`（lang=all 既定）total 32832（2026-11-01 記録）≈ live 32901（増分整合）→ hitomi は plain term に言語 filter を掛けない

判定: hitomi 正典準拠 = plain term は lang を使わない（galleriesindex 言語キーなし）。アプリ現挙動は既に正典と一致。承認方針どおり挙動変更なし、「lang 無視を明示」＝テスト固定＋記録明記を実施。C-04（同一 tag 二重 fetch 排除・commit 4c25b1f）と矛盾なし（fetch 追加なし）。

### 変更内容（最小）

- `app/searchhtml.go` plain term 分岐: 正典確認と「lang 適用は意図しない」の明示コメント追加（挙動不変）
- `app/searchhtml_test.go`: `TestSearchHTMLPlainTermIgnoresLangPerCanonicalHitomi` 追加 — `SearchHTML("furry", "english")` が言語 filter なし（nozomi fetch 禁止・galleriesindex のみ降下・集合そのまま）であることを固定
- スクラッチ: `docs/ai/tmp/assess-20261103-1220/`（fetch_searchlib.py・scratch_searchlib.js・fetch_searchhtml_page.py・scratch_search_html.html・fetch_search_results_js.py・scratch_search.js・scratch_results.js・a04_live_lang_check.py・a04_live_observation.txt）。live 呼出し計 10（version 1・降下 6・data 1・nozomi 1）、gate 1000ms 維持

## Verification

- `app` `go vet .` clean。`go test -count=1 -timeout 300s .`: 1回目 FAIL（`TestSearchCacheSizeCapBoundsMemory` = known-issue 2026-11-04 記録の flaky LRU tie。本項目は cache write path 非触・単独実行 PASS 条件該当。分析済みとして再実行）→ 2回目 ok 18.702s。対象テスト単独: `TestSearchHTMLPlainTermIgnoresLangPerCanonicalHitomi` PASS・既存 `TestSearchHTMLPlainTermUsesIndexExactTerm` PASS（0.162s）
- ルート `python -m pytest tests/ -v` → 40 passed（9.80s。freshness 含む）
- canonical exe 再 build 完了（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）。UI 非関連項目のため画面操作確認は該当なし（canonical server は非起動。UI confirmation procedure の steps 3-5 は本項目に適用外）
- 実測根拠: 呼出し 10・応答差分 32901 vs intersection 5952（上記・`a04_live_observation.txt`）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
