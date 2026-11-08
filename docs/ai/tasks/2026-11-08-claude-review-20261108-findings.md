# 外部指摘（Claude）2026-11-08 読み取り検証記録

**Status**: 進行中
**Date**: 2026-11-08
**Next Action**: 委譲再開（指示 2026-11-08 08:08Z）。済み 3 件（N-01・N-02・N-03）。残り 14 件を逐次委譲（N-04 から）。優先順位は全件委譲後に確定。決定は委譲時に親へ提示

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

### 委譲状況（2026-11-08 05:08Z 時点で停止）

- 委譲停止の指示: 「調査タスクへの委譲は今ではない」。以後の委譲は停止し、記録のみ継続。
- 委譲済み・結果受領済み（3 件）。子の記録は個別タスク記録のみ。コード・テスト・実 DB・完成フォルダは未変更。
  - N-01 有効（機構確定・実測）。Go `scanSkipStatus` は meta 一致で保存済み status を返すのみ、`scan_source`・VT api_error を参照しない。Python 参照実装は skip 機構なし（差異として記録）。推奨は `scan_meta.retry_needed`（列追加のみ・D-03 非破壊）で失敗行を skip 条件へ。小。重要度 中高・緊急度 中。
  - N-02 有効（実測）。2MB 超の fixture で `/OpenAction`（2,300,035 B）が findings=[]（走査 2,097,152 B）。推奨は `MaxScanBytes` 内分割（head + tail 65,536）と xref/`/Root` 照合。同一ファイルの pass→hit（export skip 波及）は意味変更として承認案件。小〜中。重要度 mid。
  - N-03 有効（mock 実測）。TTL 1h 超過後の最初の取得が 404 起点、throttle 60s 内で `sameGG` true → 有効画像が negative cache TTL 5 分 `no_full`（4m59s hit / 5m01s 失効）。推奨は remote path 先頭（cache hit 判定後）で `EnsureGG(ctx, false)` 1 行。handler 最頭案は cache hit の意味を弱めるため却下（実測）。小。重要度 高・緊急度 中。
- 未委譲（14 件）: N-04〜N-12、N-13〜N-17。確度は委譲前（読み取り）のまま。
- 優先順位は未確定。根拠不足（子の要約がない項目が 14 件）。確定しているのは指摘側の提示（1・3・4・5・8・9 が小規模・効果、2・15 が修正量対価値）と、委譲済み 3 件 within の記載（N-03 が最重要候補）のみ。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
