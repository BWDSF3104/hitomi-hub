# 保存済みステータスの定義（後で定義が必要）

**Status**: 進行中
**Date**: 2026-11-05
**Next Action**: assess-20261106-0326: 完成後削除の扱いの選択（推奨A=event列 saved_at / 推奨B=現行state再計算維持）はユーザー承認を要る。実装開始指示未受領

## 意図 / 背景

保存済み（saved）ステータスの定義を、後で確定する。ユーザー指示 2026-11-05 08:02 により本件（`docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`）から分離。

現状の正典定義は `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項（指定フォルダ内に全 entry の local ファイルがあること・部分保存は badge に出さない）。本タスクではこの定義が将来何を基準とするかを確定する（正典の変更の有無・部分保存の扱い・算出源の恒久的な決め方を含む）。

本件 jump 機能の側は、本タスクの確定を待たず現行正典定義をそのまま使用することを継続判断として維持（算出は `v.info` の既存源・近似は表示不一致のため不採用）。

## assess-20261106-0326（実行記録・親）

- 実行固有ID: `assess-20261106-0326`（開始 2026-11-06 03:26 JST / 2026-11-05T18:25:46Z）
- 対象項目（1件）: 本ファイル `docs/ai/tasks/2026-11-05-saved-status-definition.md`（保存済みステータスの定義（後で定義が必要））
- 開始時点の既存未コミット変更: なし（`git status --porcelain` 空）
- goal: 開始2回失敗のため未使用（手順直実行）

## 調査結果 / 変更内容

### 定義確定（ユーザー指示 2026-11-05 13:52）

- **saved = 当該 gallery のすべての page（全 entry）を指定フォルダ（config `gallery_dir`、既定 `Galleries`）へコピーし終わった時点**
- 既存正典 `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 3項との関係:
  - 正典の「指定フォルダ内に全 entry の local ファイルがあること」は、通常運用（削除なし）でコピー完了時点と等価 → **正典は無変更**
  - 部分保存の扱い: 無変更。部分保存は badge に出さず meta 側 `local N/M`（この定義と一致 — badge は完成時点のみ）
  - この定義は**時点（event）基準**。現行実装（card API の `entryLocalFormats` ループによる state 再計算）との唯一の分岐は**完成後のファイル削除**: state 再計算では saved が false に戻る、時点基準では true を保持
- 算出源の恒久的な決め方（保留・確認必要）:
  - 逐再計算（現行どおり・追加 I/O なし）→ 完成後削除で badge が戻る
  - 完成時点の記録（sticky）→ `read_state` 等に event 列が必要（schema 変更・迁移は非破壊規約準守）

### 残る確認点

1. 完成後ファイル削除時に saved を保持するか（時点記録）それとも state 再計算で false へ戻すか。これにより算出源（event 列 vs 既存 `entryLocalFormats`）が確定する。

### assess-20261106-0326（子調査実務記録・2026-11-06）

有効性: **一部有効**。定義（時点（event）基準）は有効・確定済み（ユーザー承認 2026-11-05 13:52）。実装は通常運用で定義と等価だが、完成後削除の分岐で実装は state 再計算側（saved=false へ戻る）であり、この分の選択はユーザー承認を要る（UX の意味の変更・schema 変更）→ 選択そのものは判断保留（実測で閉じた事実と推奨は下記）。

現在の実装と判断根拠（実測済み・コード確認）:

- card API（`searchResult`）の `saved` は `entryLocalFormats` ループによる disk 状態の逐再計算（`app/viewer.go:640-663`）。`entryLocalFormats` → `fileLocal` → `os.Stat`（`app/viewer.go:706-718`、`app/viewer.go:913-920`）。per-request で disk 現況から算出するため、完成後削除で saved は false へ戻る（event 保存はない）。
- `read_state` に saved の event 列はない（`schema.sql:60-68` は `viewed`/`viewed_at` のみ。event 列は未実装）。`reset` の "all" 節でも「ファイル由来の saved は削除しない」＝列なしと整合（`app/library.go:106`）。
- jump 側の `savedStrict` は同一の disk ループ（`app/firststatus.go:99-116`）。saved pre-filter は `files` 行の `EXISTS`（record 基準・`app/firststatus.go:144-172`）で、candidate 側は削除後も record が残るが strict 再計算が上書きするため結果は state 再計算と一致。event 記録（sticky）を選ぶ場合はこの pre-filter も event 列源へ切り替えないと candidate（record=true）と flag（event）が分かれる。
- UI は card 応答の `saved` を badge に表示（`app/search.html:263`、`:522`）。部分保存は badge に出さず meta `local N/M`（無変更）。

実測（スクラッチフィクスチャ・実データ無変更・instance port 8741・2026-11-06）:

- 全 entry（3件）local 揃った状態で card API `saved=1` → 1 entry を gallery 配下から移出（削除相当）→ `saved=0` → 復元 → `saved=1`。現行実装は state 再計算で、完成後削除により saved が false へ戻ることを実測で確定（sticky なし・per-request 再計算）。
- 実測は `docs/ai/tmp/assess-20261106-0326/` の複製フィクスチャで実施（実 gallery ファイルの削除・DB 書き込みはしていない）。

推奨（複数案のうち最も適切と判断する案・最終選択は承認要）:

- 推奨A（正典の event 定義への整合）: `read_state` へ `saved_at TEXT` 追加（非破壊迁移・schemaVersion 6→7）、copy 完了時点（download 完了判定）で書き、card API・firststatus の saved 算出源を event 列へ。badge=完成 history・meta `local N/M`=現況 の二重表示は既存設計と両立。実装 hook は app 自身の download 完了時（手动コピーは event にならない旨を明記）。
- 推奨B（現行維持）: state 再計算継続（追加 I/O なし・schema 変更なし）。ただし完成後削除で正典定義（時点）と分岐するため、「正典は通常運用解釈で等価」という運用解釈の側の承認が必要。
- 選択は UX の意味（badge が実在ファイルと食い違う）と schema 変更に影響するため、この論点だけ判断保留。実測・記録確認は尽くした（上記）。

推定規模: 推奨A 小〜中（main.go 迁移・download 完了 hook・viewer.go/firststatus.go の算出源変更・テスト。4〜6 ファイル）、推奨B 変更なし（決定記録のみ）。

優先順位付けに効く要素: 重要度=中（分岐は完成後削除の稀ケースのみ）、緊急度=低（実装開始指示未受領・jump/badge 実装は現行正典定義で稼働中）、リスク=中（UX 意味変更・schema 迁移は非破壊規約準守必須）、依存=badge 実装（search.html card 応答）・firststatus pre-filter・download 完了判定・decision `2026-11-05-saved-status-definition-approved`（正典無変更・保留点の承認待ち）。

画面確認: **未確認**。理由: 対象 instance は起動済みでない（port 8731 は実測で connectable False）かつ既存ルール上に dev server 起動コマンドなし（AGENTS.md Development「dev server・build コマンドは適用なし」）、正典 exe ビルド不在（`tools/` に exe なし）。実データに saved gallery（全 entry disk 揃い）がなく、実 data で saved badge を表示できない。コード経路（badgeText が card.saved を読む）と既存テスト（`app/viewer_test.go:1737`・`1746`・`1765`・`2217-2280`）で確認済み。スクリーンショット保存なし。

正典文書間の矛盾: 検出せず（task 記録と decision `2026-11-05-saved-status-definition-approved` は一致。実装済み badge タスク記録の「実装開始指示未受領」は later 実装タスクで解消済みだが本項目の有効性判定には影響しない）。

## Verification

- `go vet .`（app・2026-11-06）→ 出力なし（成功）
- `go test -count=1 -timeout 300s .`（app・2026-11-06）→ `ok hitomi-hub/app 25.456s`
- 実測（スクラッチフィクスチャ・port 8741・実データ無変更）: `powershell docs\ai\tmp\assess-20261106-0326\measure_saved.ps1`
  - `A_saved_before_deletion=1` / `B_saved_after_deletion=0` / `C_saved_after_restore=1`（saved は entry 基準で正しく 1→0→1。`local` 計数はフィクスチャフォルダの全ファイルを数えるため余分なフィクスチャファイル分を含むが saved 判定に影響なし）
- `Test-NetConnection -ComputerName 127.0.0.1 -Port 8731` → TcpTestSucceeded False（実 instance 未起動）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
