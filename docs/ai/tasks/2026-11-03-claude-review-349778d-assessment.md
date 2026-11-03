# 外部指摘 349778d 調査フェーズ（assess-20261103-1220）

**Status**: 完了
**Date**: 2026-11-03
**Next Action**: 調査フェーズ完了。残項目は個別タスク側で実施

実行固有 ID: `assess-20261103-1220`

## 意図 / 背景

- remote task `2026-11-03-hitomi-hub-claude-review-349778d.md` の受領による調査フェーズ。
- 元指摘は commit `349778d` 時点の静的読解であり、実行確認されていない。本フェーズは現行コードと実動作で裏取りして有効性を判定する。
- 本フェーズではソースコード・文書を一切変更しない。変更するのはタスク管理側の記録のみ。
- 基準コミット `349778d` は HEAD と同一（`git log --oneline -1` = `349778d`）。したがって「基準時点で有効 / 現 HEAD で解消済み」の分岐は全項目で同一。
- ローカル専用前提のためセキュリティ項目は扱わない。

### 作業開始時点の Git 状態

- HEAD: `349778d fix: keydown の左右矢印キーを spread direction の reading direction に合わせる（RTL では左=前进）`
- `git status --short`（既存・開始時点の未コミット変更。本タスクの成果物に含めない）:
  - `?? Galleries/`
  - `?? app/docs/`
  - `?? hitomi.db-shm`
  - `?? hitomi.db-wal`
- 追跡ファイルの修正はなし。分支: `main`。
- 開始時点で進行中の別タスク 2 件（`2026-11-01-scan-layer-move.md`、`2026-11-02-external-code-review-findings.md`）を並行継続。変更していない。

## 項目一覧（ID 順）

登録先タスクは `docs/ai/tasks/2026-11-03-<slug>.md`。重複対応表で統合した項目は統合先 1 タスクに元の ID をすべて記録する。

| ID | 要点（元指摘） | 登録スラッグ | 重複・関連 |
|---|---|---|---|
| A-01 | PDF page 辞書の閉じ `>>` 不足 | `a01-pdf-page-dict-close` | B-11 と同一件 |
| A-02 | `recordDiscovered` が 1 件ずつ commit | `a02-record-discovered-commit` | C-03 と同一件 |
| A-03 | `json_ms` が常に 0、metrics の二重返却 | `a03-json-ms-and-metrics` | 解消済み注記（`classifyFetchURL` の `ltn.`）と関連 |
| A-04 | plain term に `lang` が効かない | `a04-plain-term-lang` | C-04・D-10 と関連（別件） |
| A-05 | 検索カードの逐次取得 | `a05-search-card-sequential` | C-01 の逐次取得部分と同一件 |
| A-06 | `time.Sleep` が ctx 非対応 | `a06-sleep-ctx-aware` |  |
| A-07a | キャッシュ書き込み前の magic 検査がない | `a07a-cache-write-magic-check` | A-07 の分割 |
| A-07b | fetch の重複排除（singleflight）がない | `a07b-fetch-dedup-singleflight` | A-07 の分割 |
| A-08 | `thumbProxy` が TN 失敗時に full へフォールバック | `a08-tn-fallback-to-full` | フル画像ガード |
| A-09 | `serveStoredImage` の全読み | `a09-serve-stored-image-full-read` |  |
| A-10 | `download` が既存ファイルを確認しない | `a10-download-existing-check` |  |
| A-11 | `rawRGB` が全ページ分保持される | `a11-pdf-raw-rgb-memory` |  |
| A-12 | SHA256 を毎回全件計算 | `a12-sha256-every-scan` |  |
| A-13 | `ScanPDFKeys` が key ごとに `string(data)` | `a13-scan-pdf-keys-string-alloc` |  |
| A-14 | `infoCache` に上限がない | `c05-cache-limits`（統合先） | C-05 と同一件 |
| A-15 | `isLoopbackHost` が `[::1]:port` を弾く | `a15-loopback-host-ipv6` |  |
| B-01 | 検索ボックス Enter で「前ページ」が実行される | `b01-enter-implicit-submit` | UI 実操作必須 |
| B-02 | objectURL 2000 件上限で 2001 個目が壊れる | `b02-objecturl-lru` | フル画像ガード・UI 実操作 |
| B-03 | お気に入り状態がキャッシュで巻き戻る | `b03-fav-state-cache-rollback` | UI 実操作必須 |
| B-04 | acquisition のフォルダ名と確認処理の不一致 | `b04-acquisition-title-sanitize` | D-02 と関連 |
| B-05 | `hitomi` キーのない config で `use_live_gg` が false | `b05-config-hitomi-default` | フル画像ガード・D-02 関連 |
| B-06 | VT の空 `last_analysis_results` が `pass` | `b06-vt-empty-results` |  |
| B-07 | YARA の一時失敗で構造 `pass` が `scan_error` | `b07-yara-failure-overwrites-pass` |  |
| B-08 | zip の `..` 判定が部分一致で誤検知 | `b08-zip-traversal-partial-match` |  |
| B-09 | `newtab=1` が実質的に効かない | `b09-newtab-click-handler` | UI 実操作必須 |
| B-10 | Python 参照実装の VT が 404 で落ちる | `b10-python-vt-httperror` |  |
| B-11 | PDF page 辞書の閉じ不足 | `a01-pdf-page-dict-close` | A-01 と同一件 |
| C-01 | 検索の体感（カード逐次取得） | `a05-search-card-sequential`（逐次取得部分） | A-05 と同一件。内訳を分けて記録 |
| C-02 | カードごとの `currentPriorityDone` 待ち | `c02-card-current-priority-wait` | C-01 の第 2 点 |
| C-03 | `recordDiscovered` が 25 回 commit | `a02-record-discovered-commit` | A-02 と同一件。実測値の根拠を検証 |
| C-04 | `language:` 併用で同じ tag を 2 回取得 | `c04-language-double-fetch` | A-04 と関連（別件） |
| C-05 | キャッシュの上限なし（対象全体） | `c05-cache-limits` | A-14 と同一件 |
| C-06 | strip の再構築 | `c06-strip-rebuild` | フル画像ガード可能性 |
| C-07 | resize と fullscreen の generation | `c07-resize-generation-debounce` | フル画像ガード |
| C-08 | `no_full` の連打 | `c08-no-full-storm` |  |
| C-09 | `/api/read`・`/api/lists` の debounce 未実装 | `c09-read-lists-debounce` |  |
| C-10 | `go test` が 35 秒前後 | `c10-test-gate-sleep` |  |
| D-01 | `.gitignore` の不足 | `d01-gitignore-missing` |  |
| D-02 | 既定 config が追跡対象の `config.example.json` | `d02-config-example-tracked` | B-05 と関連（別件） |
| D-03 | findings の UNIQUE に path がない | `d03-findings-unique-path` |  |
| D-04 | `IsPE` が先頭 64 バイトしか見ない | `d04-ispe-header-window` |  |
| D-05 | キャッシュ書き込みの temp 掃除がない | `d05-cache-tmp-cleanup` |  |
| D-06 | 文書のずれ（gg case、TN fallback、check、README） | `d06-doc-drift` | フル画像ガード（gg 正典） |
| D-07 | `test_canonical_build_freshness.py` の前提 | `d07-canonical-build-test-premise` |  |
| D-08 | 未使用コード | `d08-unused-code` |  |
| D-09 | `orderby:popular` が解析されるが無視 | `d09-orderby-popular-ignored` |  |
| D-10 | `searchHTML` plain term の `_` | `d10-plain-term-underscore` | A-04 と関連（別件） |

関連として相互記録（統合なし）: A-04 / C-04 / D-10、B-05 / D-02、A-08 とフル画像ガード。

## 調査結果 / 変更内容

（項目ごとに個別タスクへ記録。本タスクは ID 順の逐次調査と優先順位決定、チェックポイント commit を持つ）

### 進捗反映（2026-11-03）

- 全項目の調査・判定・実測と個別タスク登録、優先順位決定完了。本タスクの範囲（調査フェーズ）完了。
- 実施済み項目（個別タスクで完了）: A-08（`683ff47`）→ C-08（`1b8bb30`）→ A-07b（`fa10536`）、A-05/C-01/C-02（`c77f92a`）、B-02（`622fadc`）、A-01/B-11（`55b9f83`）、D-05（close `621056f`）。残項目は個別タスクで進行中（実施未開始）。

判定サマリ: 有効 33・機構事実だが判定保留 3（A-04/D-09/D-10 は hitomi 準拠確認が必要）・解消済み 1（D-05）・D-08 の `pageSource` 未使用は誤り（route/handler 現用に付き）、D-05 は現行 code に defect なし（cleanupTemp あり）。

優先順位（影響・証拠強度・修正規模）:

1. A-05/C-01/C-02 — card 逐次 fetch が gate 1000ms で lock。体感 30 秒級（実測 cards 30552ms）。UI 最大。
2. B-02 — objectURL 上限で active 含む全 revoke。表示が空白化（実測）。UI 正確性、中規模。
3. A-01/B-11 — PDF page 辞書未閉。生成物不正。小規模・確実。
4. B-08 — `..` 部分一致で正当名を hit し export から除外。小規模・実測。
5. D-03 — unique index に path なし＋新規 DB に unique index 未作成（重複実測）。小〜中。
6. D-04 — `e_lfanew >= 64` の PE を誤判定（実測）。小。
7. B-01 — Enter が検索でなく prev（page 1 では disabled で無動作、page ≥2 で前ページ）。小。
8. B-03 — favorite cache 巻き戻しで toggle 方向が逆（実測）。小。
9. B-09 — same-document fragment link の target 無効（実測）。小（方針確認必要）。
10. B-05 — `hitomi` キーなし config で live gg default 適用外（実測）。小。
11. A-15 — `[::1]:port` Host を拒否（実測）。小。
12. A-10 — download が既存ファイルを確認せず再 fetch。quota 浪費。小。
13. A-07a — cache write に magic 検査なし。小（ガード関連）。
14. B-06 — VT 空 results を pass と誤判定（静的）。小。
15. B-07 — YARA error で pass を scan_error へ上書き（実測）。小〜中。
16. B-10 — Python 参照実装の VT が 404 で HTTPError（静的）。小。
17. A-02/C-03 — record が call ごと commit（実測 37ms/件）。小。
18. A-03 — json_ms 常に 0・metrics 二重返却（実測）。小。
19. A-14/C-05 — cache 上限なし。中。
20. C-04 — `language:` 併用で同一 tag 二重 fetch（実測補助）。小。
21. C-08 — `no_full` の negative cache なし（bounded）。小〜中（ガード関連・設計判断）。
22. C-06/C-07/C-09 — strip rebuild・resize・read/lists の debounce なし。小〜中。
23. A-11 — rawRGB 全 page memory。中。
24. A-04/D-09/D-10 — hitomi 準拠確認が必要（判定保留）。中。
25. A-07b — fetch dedup の追加は past singleflight scope 決定のため設計承認が必要。
26. A-06 — ctx-aware sleep。小（低優先）。
27. A-09 — full read・Range/304 なし。小。
28. A-13 — `string(data)` 割当。trivial。
29. C-10 — `go test` 35s（情報）。小。
30. D-01/D-02/D-08 — trivial。D-05 解消済み。D-06 は docs 修正（承認後）。D-07 は clean clone 限定。

## Verification

- `task_manager.py add` で本タスクと個別 43 タスクを登録した（`docs/ai/tmp/assess-20261103-1220/register_result.txt`, `rc=` エラーなし。PowerShell 版は UTF-8 parse error、Python 版成功）。`current-task.tsv` は header + 既存 2 + 新規 44 = 47 行。
- 実装ファイル・文書ファイル（`docs/ai/` 記録以外）の変更はしていない。scratch（`docs/ai/tmp/assess-20261103-1220/`）に再現スクリプトとスクショを置いた（gitignore 対象）。

実機確認（正典 exe からの server, fresh tab, viewport 1280x900）:
- 正典 exe 再 build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）→ server 起動（port 8731）→ その server が返す UI/network を使用。
- `go vet .` clean。`go test -count=1 -timeout 300s .`（app）= `ok hitomi-hub/app 35.352s`。`python -m pytest tests/ -v` = 25 passed（canonical build freshness 含む）。
- direct baseline（`/api/search?query=furry&lang=all&page=1&limit=25`）: `X-Search-Timing: search=8477;record=935;json=0`, `cache-control: no-store`, total 32892, ids 25。初期表示（fresh tab, search button 不使用）: `.side` 480px、`#search` height 67.8px、card 25、cover 130x130、card height 141、page status `page 1 / 1316・total 32892`。数値基準は満たすが `#search` controls は 480px 内で 2 行 wrap（criteria 文面の「単一行・wrap なし」と observed app が食い違う → criteria 側 drift として記録）。スクショ `01-initial-display-1280x900.png`。
- `#search-submit` click（`female:yuri`）: DOM card 25、`#count` `58507 件・page 1`、server timing `search=879;record=796;json=0`、`cache-control: no-store`。direct baseline total 58507・ids 25 と一致。スクショ `02-b01-enter-implicit-submit.png`（Enter の機構確認後 state）。
- B-01: page 1（`search-prev` disabled）で Enter → network request なし（検索実行されず）。page 2 で `female:catgirl` 入力 + Enter → `#/q/female%3Acatgirl/page/1`（前ページへ移動）。button の `type` 属性なし、tree order首位 = `search-prev`。
- B-02: `#reader` の objectURL display（真の cached full `001313d7...` format=avif, `x-cache: cache`）で `fullObjectUrls` を 2000 にし、2001 件目を `storeFullCache` → size 0・active 0・objectUrl null・display src ""・naturalWidth 0。スクショ `03-b02-objecturl-revoke-all.png`。
- B-03: gallery 4300588 で `#fav` click → `{"gallery_id":4300588,"on":true}`。card 再 click（cache hit）で `current.favorite` が 0 へ巻き戻り、次の click は unfavorite 期待で `on:true` を送信（DB favorite は 1 のまま、`#favorites` に残る）。テスト後は unfavorite へ戻して state を掃除。
- B-09: `?newtab=1` tab で card anchor click → tab 数増えず same tab の hash 変更（`target="_blank"` 属性は DOM に出るが same-document fragment では無効）。
- A-15: `POST /api/fav` + `Host: [::1]:8731`（invalid body）→ 403、control `Host: localhost` → 400。
- A-09: cached image の `Range`/`If-None-Match` → いずれも 200 全 read（Accept-Ranges/304 なし）。
- A-01/B-11: scratch gallery 106 の export PDF で page object が `>>` なし（`show_pdf_page.py`）。
- B-07/B-08/D-03/D-04/A-12: `docs/ai/tmp/assess-20261103-1220/repro_scan.py`（config `yr` なし/不正、fixtures）で実測。D-03 の (2) は scratch DB に `idx_findings_unique` が無く canonical DB に有ることを `verify_more2.py` で確認。
- B-05: `hitomi` キーなし config（port 8732）で server ログ `gg: snapshot fallback (live gg.js unavailable)`、search metrics に `gg` group なし。

スクショ保存先: `docs/ai/tmp/assess-20261103-1220/` → タスク専用 Drive フォルダ `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\`（01/02/03 をコピー）。OS screen capture（正典 criteria の「normal OS screen capture」）は Playwright page screenshot で代替しており、この代替は記録に明示した。厳密な OS capture が必要なら追加取得する。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
