# 確認テスト正典ブラウザ開閉ルール追記 実施報告

**Date**: 2026-11-09
**Task**: 2026-11-09-hitomi-hub-confirmation-rule-browser-close

## 完了状況

| 手順 | 内容 | 結果 |
|------|------|------|
| 1 | 棚卸し・タスク登録・保全の確認 | 完了 |
| 2 | 追記の実施 | 完了 |
| 3 | 差分の確認と正典検証 | 完了 |
| 4 | コミットと push | 完了 |

**すべて完了**: ✅ はい（4件 / 4件）

## 作業開始時点の状態

- **HEAD**: `de66d65` (perf: boundedCachePut の eviction を access 順 physical LRU へ)
- **git status 要約**:
  - Staged: `docs/ai/current-task.tsv`, 2つのタスクファイル
  - Unstaged: `app/search.html`, `tests/search_visible_priority_harness.mjs`, `tests/test_search_visible_priority.py`
  - Untracked: `reports/2026-11-09-kilo-commands-overwrite-browser-close-report.md`
- **対象ファイルの未コミット変更**: なし（`docs/ai/rules/confirmation-test-rule.md` はクリーンな状態）
- **登録した個別タスク ID**: `2026-11-09-hitomi-hub-confirmation-rule-browser-close`（既存タスクファイルを使用）

## 追記内容

**位置**: `docs/ai/rules/confirmation-test-rule.md` の **§4「スクリーンショットの被写体と取得方法」** 末尾（`## 5.` の直前）

**追加した3箇条**（一字一句そのまま）:
1. ブラウザは、開いたセッションが責任を持って閉じる。画面確認のため Playwright browser tool でブラウザを開いた場合は、スクリーンショットの Drive 保存とタスクへの記録が終わった時点で、他の工程へ進む前に browser tool の終了操作で閉じること。その後に不合格・未解決・判断の持ち帰り等のどの経路で終了する場合でも、結果を返す時点でブラウザは閉じていること。閉じられない場合は、その旨と理由を結果に明記する。
2. 委譲元の統括セッションは、委譲のたびに、結果へ「ブラウザを閉じた（または開いていない）」旨が含まれていることと、Playwright のブラウザ本体プロセス（コマンドラインに専用プロファイル／一時フォルダを含むもの。MCP サーバー本体は除く）の残存が 0 であることを機械的に確認する。残存がある場合は、その識別条件に一致するブラウザ本体プロセスだけを終了させ、終了させた件数を記録する。
3. 委譲を使わない単独実行でも同じ規律とする。タスク終了時にブラウザを閉じ、閉じた事実をタスク記録または報告に残す。

**ヘッダー更新**:
- **Date**: `2026-11-06` → `2026-11-09`
- **Status**: `active（ユーザー指示 2026-11-06 08:13 UTC）` → `active（ユーザー指示 2026-11-06 08:13 UTC / 2026-11-09 ユーザー指示でブラウザ開閉ルールを追記）`

## 正典検証結果

| 検証コマンド | 結果 | 詳細 |
|-------------|------|------|
| `go vet .` (app) | ✅ 合格 | 警告・エラーなし |
| `go test -count=1 -timeout 300s .` (app) | ✅ 合格 | ok  hitomi-hub/app  33.411s |
| `python -m pytest tests/ -v` (root) | ✅ 合格 | 64 passed in 25.77s |

## コミットと push

- **コミットハッシュ**: `b854e72`
- **コミットメッセージ**: `feat: 確認テスト正典にブラウザ開閉ルールを追記（§4末尾に3箇条・ヘッダー更新）`
- **push**: ✅ 完了（`main` → `origin/main`）

## 秘密情報

報告に API キー、トークン等の秘密情報は含めていない。