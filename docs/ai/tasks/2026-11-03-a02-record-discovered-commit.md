# A-02/C-03: recordDiscovered が 1 件ずつ commit

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/main.go:991` の `recordDiscovered` は `dbExec` に対し 1 Exec（autocommit）で、search handler は ids を 1 件ずつ呼ぶ。canonical server（2026-11-03 03:36）`X-Search-Timing: search=8477;record=935;json=0`（25 ids → 約 37ms/件）。`limit=1` では `record=195`。対応: 1 request（または 1 saved query）を 1 tx にまとめ、external call を tx 外で完了してから書き込む。合格条件: mocked fetch/DB で commit 回数が request あたり 1。推定規模: 小。C-03 と統合済み。

実装（2026-11-03・合格）。作業前チェック読了（viewer 関連·AGENTS.md 必須）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

- `app/main.go`: `recordDiscoveredBatch`（1 request 分 ids を 1 tx。fetch は tx 外完了。ids 空は tx を開かない＝commit 0）追加。commit 回数を mocked DB で測るため `execer`（Exec のみ）/`txExec`（Exec+Commit+Rollback）/`dbTransactor`（Begin）を interface として追加（`dbExec` は `execer` を埋込む。`recordDiscovered` の引数は `dbExec`→`execer`、既存 call site の意味不変）。CLI `search` の loop を 1 tx 化（1 件ごとの `gallery:` 错误 print+continue 意味は維持、commit は request 末尾の 1 回）。
- `app/viewer.go`: search handler の record phase を per-id loop → `recordDiscoveredBatch(v.db, ids, v.root)` 1 呼び出しへ（エラーは無視＝従来どおり `_ =`。timing・metrics・Other 意味不変）。saved query 経路（check.go の既存 tx）は変更なし。
- テスト: `main_test.go` に fake harness（commit 回数 = tx 内 Exec 1 commit + autocommit 1 Exec を計上）+ `TestRecordDiscoveredBatchCommitsOncePerRequestMockedFetch`（fetch→begin→exec-in-tx×25→commit の順序と commit 1、baseline 1件ずつ=25、2回目行数不変、empty ids 0）と `TestRecordDiscoveredBatchPersistsRowsOnRealDB`。`viewer_test.go` に `TestSearchHandlerRecordsAllIDsInOneTransactionAfterFetch`（handler 後行数 25、2 回目 request 行数不変、timing ヘッダ相位意味不変、mock env record 実測を記録）。
- 判断点（スコープ）: 承認スコープ明示は `app/main.go` だが、合格条件「request あたり commit 1」は handler の呼出 1 回化なしで成立しないため `app/viewer.go` の record phase 1 行（call site）のみ変更。CLI search の commit 完了時点は request 末尾へ移動（1件ずつ error print+continue は維持）。mock env は in-process の commit コストが極小で timing 方向は unstable（実測 516.7µs vs 517.5µs）→ commit 回数は fake で固定、timing 改善は canonical server 実測で示す。

## Verification

- mocked（fake harness, 25 ids・request あたり commit 回数）: request1 = **1**（fetch→begin→exec-in-tx×25→commit）、修正前基準（1件ずつ autocommit）= **25**、2回目 = 1・行数 25 のまま不変、empty ids = **0**。テスト: `TestRecordDiscoveredBatchCommitsOncePerRequestMockedFetch` PASS。real DB 永続化: `TestRecordDiscoveredBatchPersistsRowsOnRealDB` PASS（rows 25、2回目も 25）。
- handler（mock env・同 DB 実測）: `header record_ms=1, batch-tx(1 commit)=516.7µs, per-id autocommit baseline(25 commits)=517.5µs`（in-process modernc は commit 1回あたり fsync コストが極小で wall-time 差は出ない。commit 回数の改善は fake で固定済み）。
- canonical server 実測（rebuilt exe・scratch root `docs/ai/tmp/assess-20261103-1220/a02-server`・2026-11-03）: run1 `X-Search-Timing: search=539;record=45;json=0`（25 ids）、run2 `search=0;record=38;json=0`（25 ids・既存行）。基準 record=935ms（約 37ms/件・25 commit）→ **record 935→45/38ms（1 commit）**。server は未稼働（開始前状態）へ復した。CLI `search` live: `count:5` 1 tx・`gallery:` error なし。
- `go vet .` clean、`go test -count=1 -timeout 300s .` ok **39.958s**（回帰テスト群包含。既存 search timing shape テスト PASS）。ルート `python -m pytest tests/ -v` **38 passed in 3.67s**（`test_canonical_build_freshness.py` 含む。UI 未変更・canonical exe は変更 code を反映して rebuild）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（`task_manager.py end` で実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合。assess-20261103-1220 の backlog 反映は親）
