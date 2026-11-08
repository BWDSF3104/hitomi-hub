# 外部指摘（Claude）2026-11-08 読み取り検証記録

**Status**: 進行中
**Date**: 2026-11-08
**Next Action**: 優先順位確定済み（17/17・N-13 対応不要で閉）。実装開始指示待ち。上位: N-17・N-15・N-16・N-03

## 意図 / 背景

- 外部指摘（Claude）2026-11-08 を正典記録として残す。実行はしておらず、すべて `f9f7bf1` のコードに対する読み取りによる指摘。
- 前批（2026-11-06・assess-20261106-1056）の項目は大半が実装済み。本批は「現コードに残っている問題」と「新たに発見した問題」のみ。
- 用户指示（本批の追加指示）: 「記録のみを実施し、後の作業は調査タスクに委譲」。亲（このセッション）は記録・登録・優先順位付けのみを行い、調査実務は項目ごとに子（`task`）へ委譲する。ソースコードは変更しない。
- 開始時点の Git 状態: clean・HEAD `f9f7bf1`（`docs: assess-20261106-1056 実装完了の記録反映`）。開始時点の未コミット変更なし。
- 実行固有ID: `assess-20261108-1332`
- Memory Bank の既存進行中タスク（2026-11-06 findings）は変更せず並行継続した（goal 実行中はユーザー確認不能・既存行の書き換えなし）。
- 確度は 3 段階: 確定（現コードから機構確定）／機構確定・影響は推定／未検証。子の実測で更新する。

## 対象項目一覧（18 件のうち 17 件・本批）

| # | 項目 | 委譲前確度 | 個別タスク記録 |
|---|---|---|---|
| N-01 | scan skip が失敗状態（VT `rate_limited`・`no_key`、YARA `yara_error`）を固定し再評価しない | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n01-scan-skip-failure-status.md` |
| N-02 | `ScanPDFKeys` が先頭 2MB しか読まず後方の `/OpenAction`・`/AA` を見逃す | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n02-pdf-danger-keys-head-tail.md` |
| N-03 | 稼働中に gg が TTL 更新されず、bucket 回転後の最初の取得が必ず 404 起点。throttle + `sameGG` で 5 分 `no_full` | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n03-gg-ttl-not-refreshed-in-proxy.md` |
| N-04 | `readerItems` が 1 枚目だけ `isAlreadyDoublePage` を見て、2 枚目の横長を半幅セルへ押し込む | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n04-reader-second-page-landscape.md` |
| N-05 | strip クリックが `openGallery` で同一 gallery を開き直し（abort・`/api/viewed` 再送・strip 再構築） | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n05-strip-click-reopens-gallery.md` |
| N-06 | `runPriority` が見開き 2 枚を 1 枚ずつ await（image lane 60ms で並列可。cold 待ち約 2 倍） | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n06-runpriority-serial-display-fetch.md` |
| N-07 | prefetch worker の race で `item` が undefined（KI-20261107-01。catch 側 `item.entry` で TypeError） | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n07-prefetch-worker-undefined-item-race.md` |
| N-08 | `negativeNoFullKey` が呼出ごとに case 値をソート・連結（key 約 12KB・noFullCache 最大 1024 件で約 12MB） | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n08-negative-no-full-key-cost.md` |
| N-09 | `pruneInfoDiskLocked` が上限到達時に上限まで刈り、次の 1 件で再度全 JSON（最大 5000 件）を parse。`infoDiskMu` 保持中 | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n09-info-disk-prune-full-scan.md` |
| N-10 | `setSearchTitleGroup` が毎カード `saveTitleGroups()`（1 ページ最大 75 回・同期 stringify 全量）。map に上限なし | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n10-title-groups-localstorage-every-card.md` |
| N-11 | `renderPrefetchStatus` が呼出ごとに `current.files` 全件走査（1000 ページで O(N²)） | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n11-render-prefetch-status-on2.md` |
| N-12 | singleflight の leader が background lane だと、後から来た foreground 要求が background lane 待ちに巻き込まれ優先度が逆転 | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n12-singleflight-lane-priority-inversion.md` |
| N-13 | 未読一覧が無制限（前批 R-18。`viewed` のみの行も含む） | 前批で機構確定・対応不要判定。本批で再評価 | `docs/ai/tasks/2026-11-08-n13-unread-list-unbounded.md` |
| N-14 | `perfStatus` の `cardsLoaded`・`cardTotalMs` が検索ごとにリセットされず、`searchMs` と数値が食い違う | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n14-perf-status-accumulates-across-search.md` |
| N-15 | `viewer.html` が remote 由来 `entry.hash` を innerHTML・属性へ無エスケープ。`search.html` の `encodeURIComponent` は `'` を変換しない | 読み取り（未検証） | `docs/ai/tasks/2026-11-08-n15-innerhtml-escape-remote-values.md` |
| N-16 | GET の `/api/*`・`/files/*` に Host 検査がない（前批 R-20 承認済み・未実装。DNS rebinding） | 前批で承認（案 A）・未実装 | `docs/ai/tasks/2026-11-08-n16-get-host-check-not-implemented.md` |
| N-17 | pytest `test_canonical_build_freshness_rule_exists` が落ちたまま（KI-20261106-04） | 前批で機構確定 | `docs/ai/tasks/2026-11-08-n17-pytest-freshness-pointer-missing.md` |

- 指摘側の優先度（引用）: 1・3・4・5・8・9 は修正が小さく効果が出やすい。2・15 は検出漏れと安全性で修正量に対して価値が高い。親の優先順位は子の要約と個別タスク記録だけを根拠に委譲後に確定する。

## 調査結果 / 変更内容

（項目ごとに子へ委譲。子の結果は個別タスク記録へ記録し、本記録には要約と優先順位を反映する）

### 委譲状況（委譲完了・17/17）

- 2026-11-08 05:08Z の停止指示（「調査タスクへの委譲は今ではない」）後、指示 08:08Z で委譲を再開し、N-04 から N-17 までフォアグラウンドで1件ずつ委譲した。全 17 項目の委譲と結果受領が完了。
- 子の記録は個別タスク記録のみ。コード・テスト・実 DB・完成フォルダは未変更。Status と Next Action は親が正典 `task_manager` で管理（閉じた項目以外は 進行中・実装開始指示待ち）。

### 委譲結果（親の 1〜2 行記録・逐次）

- N-01 有効（機構確定・実測）。Go `scanSkipStatus` は meta 一致で保存済み status を返すのみ、`scan_source`・VT api_error を参照しない。Python 参照実装は skip 機構なし（差異として記録）。推奨は `scan_meta.retry_needed`（列追加のみ・D-03 非破壊）で失敗行を skip 条件へ。小。重要度 中高・緊急度 中。
- N-02 有効（実測）。2MB 超の fixture で `/OpenAction`（2,300,035 B）が findings=[]（走査 2,097,152 B）。推奨は `MaxScanBytes` 内分割（head 2,031,616 + tail 65,536・gate 合計不変）+ xref/`/Root` 解決・key 照合 object dictionary 限定。FP 実測 64KB 窓 /AA 0.45%・/OpenAction 0.0000%。同一ファイルの pass→hit（export skip 波及）は意味変更として承認案件。小〜中。重要度 mid・緊急度 low〜mid。
- N-03 有効（mock 実測）。TTL 1h 超過後の最初の取得が 404 起点、throttle 60s 内で `sameGG` true → 有効画像が negative cache TTL 5 分 `no_full`（4m59s hit / 5m01s 失効）。推奨は remote path 先頭（cache hit 判定後）で `EnsureGG(ctx, false)` 1 行。handler 最頭案は cache hit の意味を弱めるため却下（実測）。小。重要度 高・緊急度 中。
- N-04 有効（served 実測）。2 枚目が横長でも半幅セル 500px（描画 500x281）。単独表示なら 1120x630（幅 44.6%）。推奨は `readerItems` へ 2 枚目 aspect 判定。page 番号・route・hash 尊重・既読 step 粒度の 3 点と固定寸法契約（案B）は承認分離。Drive 保存済み。
- N-05 有効（harness + served 実測）。現行 click = openGallery 1・abort 3・`/api/viewed` 1・strip full rebuild 1 → 推奨（同一 gallery は `current.page` 更新 + `showPage()`）= 0・0・0・0・highlight + render 1。`viewed_at` は初回維持で状態意味不変（再送は idempotent）。`++openGeneration` guard 維持必須。`reset target=viewed` 直後の click は badge 意味変更として承認分離。小（4 行）。Drive 保存済み。
- N-06 有効（mock 実測 + served cold 確認）。serial median 1359 ms → parallel 739 ms（gate dispatch gap 60 ms 維持）。served cold で 1 枚目 158 ms→2 枚目 1116 ms（直列）を実測。dispatch order（array order）維持が必須。strict first→second display order を残す UX 要件は承認案件。小〜中。
- N-07 有効（実測確定）。race を harness で決定論的に再現：現行 = catch 側 `item.entry` の TypeError 1 件が unhandled rejection（handler なしではプロセス落札・worker 1 死亡）、guard 版（`shift()` 直後 `if (!item) return;`）= 例外 0・worker 正常終了 2。error path は両版一致で catch の正典意味と衝突なし。既存テスト全通過・固定なし。画面確認 未確認（served 再現不能）。小（1 行）。重要度 中・緊急度 低〜中・リスク 低。
- N-08 有効（実測・数値補正）。key 実測 10,070 B（原文 12KB は約20%過大）、cap 1024 で 9.83 MiB、呼出は 1 request 典型 3 回（最大 4）。fingerprint（parse 時 1 回 SHA-256）で呼出あたり 164–178 µs → 144 ns（約1,100x）、memory 9.83 MiB → 0.151 MiB。hit 判定意味維持・衝突は実務上無視。単射保証を要求する場合は memoize（案B・承認不要）。小（S・30–50 行）。重要度 低〜中・緊急度 低。
- N-09 有効（実測）。steady state で書き込みごとに 5001 件 read+parse が恒常化（3 反復確認）。案1（10% 多め・target 4500）= 平均 parse 16.7 件・56.8 ms、案2（mtime・parse なし）= parse 0・7.15 ms・reader 待ち 12 ms、併用 = 798 µs。既存テスト「count must stay at cap after evict」と衝突（テスト更新の承認案件）。disk 層に access order 正典はなく mtime 案は R-15「作成順」流儀と整合（衝突は order 源点のみ）。15〜40 行。重要度 中（cap 到達恒常時）・緊急度 低〜中・リスク 中。
- N-10 有効（機構確定＋実測）。1 page 最大 75 回（cold 全 title 新規・上界 75/75）で 1 呼出 stringify が n=10,000 で 10.0 ms（累计 747 ms）。debounce で 75 → 1 write（747 → 11.1 ms）、cap 1024 で保持 1024/10000。debounce 側は意味変更なしで単独実施可、件数上限・淘汰順（以前見た group 消失）は承認案件。小〜中（30–60 行＋新テスト）。重要度 low〜mid・緊急度 low・画面確認 未確認（共用 browser が 2 回乗っ取り）。
- N-11 有効（実測）。N=1000 で呼出 1037・走査 1,037,000・累计 621.876 ms（N=4000 で 10.7 s）→ O(N²) 確定。差分カウンタ案は累计 7.898 ms（約1/79）で表示一致・境界一致。debounce は即時性変更（lag 最大 250 ms）で承認案件。`viewer_test.go:2850-2860` の文字列 pin により pure delta は re-anchor 要。小。重要度 low〜mid・緊急度 low。served 画面確認合格（50/147 → 176/1921）。Drive 保存済み。
- N-12 有効（機構確定＋mock 実測）。join は gate に入らず joiner 待ち = leader の lane 予約＋network。正典値で joiner 壁 390 ms（data）／1039 ms（image）。案2（group effective lane = 参加者最高優先度）は呼出 1・slot 1 を維持し gate 成分のみ先行（181 ms／800 ms）、案1（joiner 自分の lane）は同一 key 呼出 2 になる（R-13「同一内容1回」喪失）。案2は「background は未来予約しない」への例外＝正典ルール変更の承認要。小〜中。重要度 mid・緊急度 low〜mid・リスク mid。
- N-13 一部有効（前批 R-18 判定維持・対応不要・現状維持）。`app/main.go:1547-1558` に LIMIT・`viewed` のみの行の除外なし（機構確定）。正典 DB（mode=ro）read_state=23 行・viewed のみ 0 件で症状未発現。served 画面 #unread DOM 24 件＝DB 行数（上限なし）を確認・Drive 保存済み。page 化・`viewed` のみの行除外は意味変更（decision line 25）＋CLI 波及＝承認案件として分離。→ 対応不要として `task_manager` で閉じた（理由: 意味変更側のみ承認案件・現規模は実測で許容内）。
- N-14 有効（実測）。`perfStatus` は `app/search.html:180` で1回初期化・リセット文なし。harness 2 runs: search1 40 ms・カード25 → search2 120 ms・カード50・5450 ms（`searchMs` は今回分・累計ではない）。正典に「累計表示が意図」の記録なし → 今回分が意図、リセット案（runSearch 冒頭・約4行）は bug 修正。小。重要度 mid〜low・緊急度 low。Drive 保存済み。
- N-15 有効（実測・原文2点訂正）。無エスケープは属性 context のみ（text・title は escapeHTML 済み）。実行条件は `<` ではなく `'`。served 画面（port 8743・isolated origin）で mock 不正 hash の onerror が実行（`window.__n15inj=1`）、escape sim 後は実行 0・注入属性 0・node 数 3（不変）。`validHash` case 11 = pass 2 / fail 9。`search.html:685` は queries 4件中2件で href 属性が壊れ、`'`→`%27` 案は属性1個のみ・route round-trip 全一致。推奨=client 側 escape（単独実施可）。`Gallery()` で entry を落とす案は表示枚数変更＝承認案件。小。重要度 mid・緊急度 mid。Drive 保存済み。
- N-16 有効（実測）。GET route に Host 検査なし（`app/viewer.go:199-216`）、POST は `sameOriginPost`（218-238）で効く。fresh instance（port 8747）実測: evil Host GET=200・POST=403。sim 表: loopback 系 + `LocalHost`=200、evil=403（案A一致）。原文訂正: rebound Host では POST は 403（開いているのは GET 読み取り）、`EqualFold` は現行実装（`ToLower`・viewer.go:241）と異なるが case 耐性は既に成立。テスト 5 件 re-anchor 要。小（5–8 行）。重要度 mid・緊急度 low〜mid・リスク low（案A承認済み・実装指示待ち）。Drive 保存済み。
- N-17 有効（実測）。`tests/test_canonical_build_freshness.py:19` の照合対象は project `AGENTS.md`、要求 literal（:31）は project・global とも不在、現 pointer は `docs/ai/rules/confirmation-test-rule.md`（`AGENTS.md:61`）。実測 `1 failed, 59 passed in 10.09s`。テスト側を rule pointer へ合わせる推奨適用で 1 passed（同一条件で 60 passed 期待値）。AGENTS.md は write-protected＋「pointer 1本のみ」流儀と矛盾 → テスト側修正が最も適切。小（テスト1ファイル・1〜4行）。重要度 中（pytest 合格が完了基準）・緊急度 中・リスク 低。

- 残件（子報告）: Playwright output 直下の screenshot・console log の local copy（`C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\` の N-05・N-06・N-15 のファイル等）は project root 外のため正典 `kilo-safe-remove` でゴミ箱へ送れず残存。削除には追加承認が必要。project root 内の local copy はゴミ箱へ送済み。
- 訂正（N-06 の Drive 保存）: 子は confirmation txt を Drive 直下で「フォルダ名そのままのファイル」として保存し、画像は Drive へコピーされていなかった（Glob・read で file と確認）。親が Drive へフォルダを作成し txt を移動、Playwright temp の before/after 2 枚と project root に残っていた before 試行 3 枚をコピーし、project root 内 3 枚は `kilo-safe-remove` でゴミ箱へ送った。記録は `docs/ai/tasks/2026-11-08-n06-runpriority-serial-display-fetch.md`。

### 優先順位（親の確定・根拠は子の要約と個別タスク記録のみ）

1. **N-17** — 小・単独で閉じられる・pytest 合格が全項目の完了基準（現 failure をふさいでいる）。AGENTS.md write-protected でテスト側修正のみ。依存なし。
2. **N-15** — client 側 escape は単独実施可（表示・link 意味不変）で小、local origin での script 実行（mutation endpoint 保有）を閉じる。緊急度 mid。entry を落とす案は承認案件として分離。
3. **N-16** — 案 A 承認済み・未実装、小（5–8 行）、リスク low。N-15 の補完（origin 検証）。テスト 5 件 re-anchor。
4. **N-03** — 小（1 行）・重要度 高・緊急度 中。viewer の full 判定と `no_full`（正典 condition）に直結、意味変更なし。
5. **N-02** — 検出漏れ（後方 key）を閉じる価値。小〜中、FP 実測済み。同一ファイル pass→hit の意味変更＋テスト fixture 追加は承認案件。
6. **N-01** — 小、失敗状態の永久固定（再評価なし）を解く。D-03 非破壊（列追加のみ）。Python 参照実装との差異も記録済み。
7. **N-04** — served 実測で表示崩れ確定、推奨は小。page 番号・route・hash・既読 step の意味変更と固定寸法契約は承認分離。
8. **N-05** — 小（4 行）、重複呼出・再構築が消える（実測 1→0）。guard 維持必須。badge 意味変更は承認分離。
9. **N-07** — 小（1 行）、race による unhandled rejection・worker 死亡を閉じる。error path 意味不変・テスト非接触。
10. **N-14** — 小（約4行）、数値の食い違い（observability 誤読）を bug 修正として閉じる。意味変更側の承認論点のみ。
11. **N-08** — 小（S）、常駐 memory 〜10 MiB と CPU 約1,100x の改善、意味等価性実測確認済み。単射保証を要求する案Bは承認不要。
12. **N-11** — 小、O(N²)（621.876 ms）→ 約1/79。文字列 pin の re-anchor が必要。画面確認合格。
13. **N-06** — 小〜中、cold 待ち約半分（1359 → 739 ms）、gate 意味と両立。strict display order の UX 要件は承認案件。
14. **N-12** — 小〜中、joiner 待ちの優先度逆転（390/1039 ms）を gate 意味を変えず改善。正典ルール例外の承認要。案1は R-13 衝突。
15. **N-09** — 15〜40 行、cap 到達恒常時のみ（cold worst 30.1 s）。既存テスト固定と decision（order 源）の衝突が承認案件。
16. **N-10** — 小〜中、debounce 側は単独実施可（747 → 11.1 ms）。件数上限・淘汰順は意味変更（承認案件）で画面確認 未確認。

- 対応不要（閉): **N-13** — 前批 R-18 判定維持（現状維持・案A）。機構は確定だが正典 DB 23 行・viewed のみ 0 件で症状未発現、page 化・`viewed` のみの行除外は未読の意味変更（decision line 25）＋CLI 波及。理由を個別記録へ記録し `task_manager` で閉じた。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
