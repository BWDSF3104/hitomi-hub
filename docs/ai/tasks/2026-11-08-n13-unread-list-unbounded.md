# N-13: 未読一覧が無制限のまま（前回の指摘が未解決）

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: 未読一覧が無制限のまま（前回の指摘が未解決）。`listRows("unread")` は `fully_read = 0` の全行を返す。開いただけの gallery（`viewed`）も入るため増え続ける。検索と手動更新のたびに全件を取得して描画。LIMIT か `viewed` だけの行の除外が指摘の対処。前批 R-18（`docs/ai/tasks/2026-11-06-r18-unread-list-limit.md`）で対応不要（現状維持）と判定済み。page 化・上限は一覧の意味（全件 vs window）と CLI `hub list` への波及のため承認案件。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-13 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（機構・確定）

- `listRows("unread")` = `SELECT gallery_id, last_page, fully_read, favorite FROM read_state WHERE fully_read = 0 ORDER BY gallery_id`（`app/main.go:1547-1558`）。LIMIT なし・`viewed` のみの行の除外条件なしを read で確定（指摘どおり、前批機構が現行コードで維持）。
- 件数上限は全層で不在: `f.limit` flag（`app/main.go:1043`・`1077`）は download/search 部分と check 用で `listRows` へ渡さない。handler（`app/viewer.go:690-707`）は全 row を JSON 化、client `showLists`（`app/search.html:663-691`）は全 row を `#unread` へ innerHTML join（DOM node = row 数）。
- 検索と手動更新のたびに全件 fetch・再描画: `runSearch` 末尾で `showLists(searchGen)`（`app/search.html:728`）、`#lists-refresh` click で `showLists()`（`app/search.html:890`）。debounce・fetch 1 回 pin は固定済み（`tests/test_read_lists_debounce.py`・`viewer_test.go:1913-1914`）で件数意味は固定していない。
- viewed のみの row の包含: `setViewed`（`app/library.go:96-101`）・`setSavedEvent`（`app/library.go:107-112`）は read 進捗と独立に `read_state` 行を作る（`viewed_at`/`saved_at` 初回のみ）→ `fully_read = 0` で unread に入る（前批と同一経路で維持）。
- CLI `hub list` は同一 `listRows` を共有（`app/main.go:1356`）→ 件数・意味の変更は UI 限定でなく CLI 出力へ波及（維持）。

### 前批実測の照合（追加再測定不要）

- 前批 R-18（`docs/ai/tasks/2026-11-06-r18-unread-list-limit.md`）の正典 DB 28 行・scratch 1,000/20,000 行の JSON・応答 ms・DOM ms は、現行コード（同一機構・同一ファイル群。line 番号移動のみ）で維持 → 前批実測値は有効。同一条件の再実測は不要として記録。

### 現規模（正典 DB read-only・1 回数え・2026-11-08）

- `output/hitomi-hub/hitomi.db`（`mode=ro`・未変更）: `read_state` total=**23**（`fully_read=0`=23）、viewed のみの行=0、saved のみの行=0、`viewed` フラグ行=0、`user_version`=10、cols に `opened_at`/`viewed`/`viewed_at`/`saved_at` あり。前批 28 行 → 23 行（実データ小規模・指摘症状未発現）。
- スクラッチ: `docs/ai/tmp/assess-20261108-1332/n13_dbcount.py`（照合用に read_state を scratch copy へ backup し、served 画面確認で viewed のみ seed 1 件追加 → 24 行）。正典 DB は read-only 照会のみ・非変更。

### 正典照合（前批判定の維持）

- C-05 は memory cache 4 種 + `indexTopCache`・`noFullCache`・disk galleryinfo 層限定（`docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`）。`read_state` は cache ではなく state 表・bounded 正典記録なし → bounded 流儀の lists への適用は不成立（前批判定と現行正典に矛盾なし）。
- D-03 非破壊（`docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md`）: state 表の cap + eviction 型 bounded 化は衝突（前批どおり）。
- 一覧の意味: `listRows("unread")` の「未読」意味は変更しないとして固定（`docs/ai/tasks/2026-11-05-search-result-state-badges-implementation.md` line 25）。`viewed` のみの行の除外＝未読の意味（全件・viewed 含む）の変更＋badge 源（card API・同 decision 第5項）への波及 → 意味を変える側は承認が必要として分離（前批分離を維持）。
- 既存テスト: `app/hitomi_test.go:1452-1455` は `fully_read=0` 行の包含（data 依存）のみ固定。lists の「全件 vs window」意味は未固定（`viewer_test.go:277-284`・showLists pin `1903-1917`・debounce harness は fetch/debounce/guard 固定のみ）→ LIMIT 追加は既存テストを壊さない。viewed のみ除外は定義（`last_page=0 AND viewed=1`）依存で現行テストは壊れないが、意味固定テストの追加が必要。
- 前批判定（対応不要・現状維持＝案 A）を覆す新しい根拠なし: 実データ規模は 28→23 行で小、viewed のみ行 0 件（未発現）、表示影響は前批実測（20,000 件でも JSON 1.3 MB・応答 50〜75 ms・DOM 約120 ms）で許容内、正典変更なし。

### 有効性

**一部有効（前批判定維持・対応不要・現状維持）**: 機構（無制限・viewed のみの行の包含）は確定。ただし (1) C-05 前提は正典上不成立（state 表）、(2) 実データ 23 行で未発現、(3) 実測影響は前批の許容内。判定は「機構確定＋前批判定の維持」。

### 推奨

- **案 A（推奨・意味変更なし）**: 現状維持（前批 R-18 推奨と同一）。現行コードは同一機構で変更なし。
- mitigation を入れる場合は承認案件として分離（いずれも一覧の意味＝全件 vs window・未読の意味変更＋CLI `hub list` 波及）: (a) server 側 limit param（既定 unlimited）＋総件数表示、(b) client 側 window 表示＋「全 N 件中 N 件表示」明示。`viewed` のみの行の除外は decision line 25 の「未読意味変更しない」固定と衝突するため単独の実装不可（意味変更の承認が必要）。
- 親の判定（2026-11-08・根拠は子の要約と本記録のみ）: 対応不要として閉じる（現状維持＝案A・前批 R-18 判定維持）。理由: 機構は確定したが正典 DB 23 行・`viewed` のみの行 0 件で症状未発現、実測影響は前批の許容内、page 化・`viewed` のみの行除外は意味変更＋CLI 波及で承認案件。優先順位（本批 17 項目）で上位に入れない項目として `task_manager` で閉じた。

### 副作用・影響範囲

- `listRows` 共有口（CLI `hub list`）へ波及（前批維持）。
- `viewed` のみの行の除外は未読一覧の意味変更・badge 源（viewed/saved は card 由来・decision 2026-11-05 第5項）と衝突 → 承認なしに実施不可。
- UI の full 判定・TN・debounce/guard・fetch 1 回 pin は非接触。

### 画面確認（served・合格）

- cold hygiene 3条件: (1) freshness test `test_canonical_executable_embeds_current_pages` 合格（正典 hub.exe が現行 app HTML を埋め込み）、(2) fresh instance（pid 3724・port 8741・測定請求のみ応答）、(3) isolated origin `n13-assess-20261108-1332`（新規 tab・state なし）。dataDir 正典 DB の read-only backup コピーへ viewed のみ seed 1 件（正典 DB・完成フォルダ未変更）。
- window size 実測 outer 1678x1025 / viewport 1665x932（人工 resize なし）。`#unread` DOM node: before **24**（= DB 行数 24・上限なし）→ `#lists-refresh` click 後 **24**（操作は状態を変えないため before/after 同一条件で一致・rule §5 の「状態が変わらない項目は前後1組不要」の例だが両画像を保持）。viewed のみの seed 行（9000001）は list 末尾に包含（DOM 計測で確認。画像には sidebar スクロールで先頭 14 行まで表示）。
- 留意（hijack）: 測定中に共有 Playwright tab が並行 session に hijack され hash `#/q/furry/page/1` が適用され、my instance 上 runSearch（外部 search 1 件）が実行された。`#unread` 表示件数の claim は影響なし（listRows 経路に cache なし・DOM 直接計測・port 8741 の instance は pid 3724 のみ）。console error は favicon 404（無関係）。
- 保存先: `I:\マイドライブ\kilo\2026-11-08-n13-unread-list-unbounded\`（before/after png + confirmation txt）。local copy（project root 内 scratch）は正典 `kilo-safe-remove` でゴミ箱へ送った。project root 外の Playwright 出力（snapshot 等の残件）は削除せず残件として記録。

## Verification

- `python -m pytest tests/ -v` → **59 passed / 1 failed（10.55s）**。failure は `test_canonical_build_freshness_rule_exists`（AGENTS.md pointer 集約由来の既存 failure・R-10/N-02 報告済み・本項目非接触として明記）。`test_canonical_executable_embeds_current_pages` 合格（cold hygiene condition 1 の根拠）。
- `go vet .`（app）→ clean。`go test -count=1 -timeout 300s .`（app）→ **ok 25.052s**（全通過・コード非変更）。
- 正典 build（rebuild）は実施せず、正典 exe + freshness test 合格で現行一致を確認。server は正典 exe + scratch dataDir（read-only backup コピー + seed）+ port 8741 の fresh instance。
- 正典 DB・完成フォルダ未変更。正典 DB read-only（`mode=ro`）照会のみ（read_state 行数 1 回数え=23）。スクラッチスクリプトのみ実行（`n13_dbcount.py`・`n13_seed.py`）。画面確認での外部 API 呼出しは hijack による runSearch 1 件のみ（方針 decision 2026-11-01 参照・記録）。
- 実測数値: read_state 23 行（前批 28 行）、served 画面 #unread DOM 24（seed 1 件込み）before/after 同一、window 1678x1025 / viewport 1665x932、Drive 保存 + local copy ゴミ箱送信済み。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（正典 `task_manager` で閉じる際に削除。親が管理）
- [ ] 解決した `known-issues/` ファイルを `archive/` へ `mv`（該当する場合）: 該当なし（本項目で known-issues は作成しない。hijack 留意は本記録にのみ記載）
- [ ] `backlog.md` の完了項目を削除（該当する場合）: 該当なし
- 推奨（案 A・現状維持・前批判定維持）が採用された場合は実施項目として閉じ、`viewed` のみの行の除外（未読の意味変更・badge 源衝突）と page 化・上限（全件 → window）は承認案件として本記録に分離 retained（決定時に `decisions/` へ記録が必要）
