# 検索結果画面の3状態 badge と手動リセットの実装

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 子へフォアグラウンド委譲（schema 迁移・library・viewer.go・viewer.html・search.html・CLI）。正典 build 実機画面確認まで

## 意図 / 背景

- 検索結果画面の card に 表示済み / 既読 / 保存済み を表示し、表示済みと既読を手動リセット可能にする。
- 案検討・計画の正典: `docs/ai/tasks/2026-11-05-search-result-state-badges.md`
- 設計判断の正典（ユーザー承認 2026-11-05 03:29）: `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md`
- 実行固有ID: `search-result-state-badges`（親の統括のみ。実装・検証実作業は子へフォアグラウンド委譲）

## 承認範囲（この項目で変更してよいもの）

- `schema.sql` の `read_state` へ `viewed INTEGER NOT NULL DEFAULT 0`・`viewed_at TEXT` 追加、`app/main.go` の `schemaVersion` 5→6 と非破壊迁移（列追加のみ・既存行削除しない）
- `app/library.go`: `readStateRow` の viewed/viewed_at、`setViewed`（viewed_at は初回のみ）、`resetReadState(db, id, target)`、`setReadState` の viewed 保持、`setFavorite` は viewed を立てない
- `app/viewer.go`: `searchResult` へ `viewed` / `saved` 追加（既存 `entryLocalFormats` ループで計算。追加 I/O なし）、`POST /api/viewed`・`POST /api/reset` ルート追加
- `app/viewer.html`: `openGallery()` の route 入力時点（`app/viewer.html:1113`）で viewed 記録（display path を await しない・generation guard 維持）、最終 page 到達で debounced read helper 内から `complete: true`
- `app/search.html`: `badgeText` を 3フラグ + お気に入りへ拡張、badge 行 inline に reset button（`data-reset="viewed|read"`）、card 応答の state で maps を補完
- `app/main.go` CLI: `read` へ `--reset-viewed` / `--reset-read`
- 該当テストの re-anchor と新規テスト（`app/*_test.go`、`tests/`）
- Memory Bank（本タスクファイル、該当する decision/known-issues）

変更しない: 検索/閲覧のページ分離、TN display policy、full image 経路（`AGENTS.md`「絶対に壊してはいけない条件」）、`listRows("unread")` の「未読」意味、card height contract（130px）、viewer nav への reset 追加、`clicked` フラグ新設。

## 合格条件と検証方法

1. `go vet . && go test -count=1 -timeout 300s .`（app）合格
2. `python -m pytest tests/ -v` 合格（`tests/test_canonical_build_freshness.py` 含む）
3. schema 迁移: user_version 6、既存 read_state 行が保持される（非破壊）。Python 参照実装は schema.sql 正典を共有するため `tests/test_scan.py` がそのまま合格
4. 状態意味のテスト:
   - `setViewed` は初回のみ `viewed_at` を書き、reopen で上書きしない
   - `setFavorite` が作った `last_page=0` 行は viewed にならない
   - `reset-viewed` → viewed/fully_read/last_page クリア、`reset-read` → fully_read のみ
   - card 応答に `viewed` / `saved` があり、部分保存は `saved=0`（meta の `local N/M`）
   - 最終 page 到達で `fully_read=1`（`no_full` でも到達のみで既読）
5. 正典 build（`output/hitomi-hub/hub.exe`）で served 画面確認。`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 準拠:
   - 目標 viewport 1280x900 を測ってから resize、fresh tab、direct server baseline と UI response 一致
   - 実データ3状態（①viewed・未既読 ②既読 ③保存済み）で badge 表示
   - reset button の click で API→DB 行変化を direct SQL で照合
   - card height contract（130px）維持、`.side` 480px、既存回帰（TN cover・generation guard・no-store fetch・full image path・reader nav・favorite/unread）
   - 画面キャプチャは Google Drive の task-specific screenshot フォルダへ保存し、保存先・ファイル名・確認結果を本記録へ記入

## 調査結果 / 変更内容

### 親の機械的確認（委譲サイクル 1）

- 委譲前: HEAD `25cf419`・clean。委譲後の変更は本項目の承認範囲9ファイルのみ（`app/library.go`・`app/main.go`・`app/main_test.go`・`app/search.html`・`app/viewer.go`・`app/viewer.html`・`app/viewer_test.go`・`schema.sql`・`tests/read_lists_debounce_harness.mjs`）。本項目分のコミットはまだ無く、task Status は子の報告（進行中・画面確認 blocker）と一致。

### 画面確認方法の判断（親が段階1/2で処理・子へ差し戻し 2026-11-05 03:33）

- 正典 `bin/kilo-fresh-browser.ps1` の変更・コピーは承認範囲外（段階3）なので不許可。OS click が `#search-submit`（x=55）へ届かない問題は、正典 CDP 経路で実測して閉じる: fresh browser を CDP target（正典 `kilo-cdp-measure.ps1` が使う port `9344`）として起動し、target tab を測れることを最初に実測で確認する。
- `docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 項目2は「`#search-submit` click **または** `form#search` submit must fire」を許容する。OS click が届かない場合は `form#search` の submit 発火を実測で確認する（query input への Enter、または CDP element click）。CDP click は trusted input event として actual UI event を満たす。どちらの機構を使ったかは記録に明記する（fresh tab・resize 前測定・direct server baseline と DOM/network 一致の基準は同じ）。
- actual submit/click event を実測でも確認できない場合は画面確認を未検証として合格にしない。正典 tool 制限は `known-issues/`（KI-20261105-）へ記録し、画面確認の残件だけ段階3として持ち帰る。Go test・pytest・正典 build・DB direct SQL 照合で取れる証拠は先に集める。

（子の報告から記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
