# R-09: search-card ごとに recordDiscovered が autocommit（search で batch 済み）

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。合格条件: card 1 ページ 25 件で commit が増えない（search 側 batch と重複排除）。記録: R-09（A-02/C-03 の残存点）

## 意図 / 背景

- A-02/C-03（`docs/ai/tasks/2026-11-03-a02-record-discovered-commit.md`）で `/api/search` の `recordDiscovered` 1 request 1 tx 化は解消済み。card 経路（`/api/search-card/{id}`）の autocommit 重複が残存点として R-09 に登録（読み取り指摘の照合、実装開始前の調査記録）。
- 正典の修正方法提示（`docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md` item 8）は「`searchCard` の単独 write も既存行 skip か tx 内へ統一」。合格条件「1 batch で 1 commit」は search 側で確認済み（A-02 実測 record 935→45/38ms）。card 側の commit 回数固定は未実施。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- search 側: `recordDiscoveredBatch`（`app/main.go:1351-1385`）を search handler が 1 呼び出し（`app/viewer.go:566`）。1 request（1 -page 25 ids）= 1 tx・1 commit。
- card 側: `searchCard` handler は card 1 リクエストごとに tx を開かず `recordDiscovered` を呼ぶ（`app/viewer.go:622`）。`recordDiscovered` は `execer.Exec` 1 回のみ（`app/main.go:1343-1349`）で、tx 外 Exec = SQLite autocommit = 1 commit。単一接続固定（`app/main.go:273`）・WAL（`app/main.go:266`、DSN `app/main.go:278`）。
- UI は card 1 件ごとの個別リクエスト（`app/search.html:580` の `loadSearchCards` worker、A-05/C-01 並列化済み）。card 1 ページ 25 件 = 25 リクエスト = 25 commit。
- 重複の実体: search handler が同じ page の 25 id を 1 tx で記録済み（`app/viewer.go:566`）。card 経路は既存行に対して同一 upsert（`ON CONFLICT DO UPDATE SET last_seen`）を 1 件ずつ再実行し、commit を追加する。行内容（unique key）は増えないが、commit と `last_seen` 更新の重複が発生する。

### 有効性

**有効（実測で確定）**。commit 回数はスクラッチ実測で card 1 ページ 25 件 = card 25 commit（search 側 1 commit 追加で page 合計 26）。機構確定: card 1 リクエスト = autocommit 1 commit。実測は Verification 参照。

既存テスト照合: commit 回数の固定は search 側のみ（`app/main_test.go:1474-1547`・fake harness・基準 batch=1 / per-id=25 / empty=0）。card 経路の commit 回数を固定するテストは存在しない（`TestViewerSearchCardHandlerReturnsResult` `app/viewer_test.go:1505-1529` は response・x-cache のみ）。よって R-09 の合格条件（card 側 commit 0）は既存テストと衝突せず、新テスト 1 件で固定可能。

### 対応の実現可能性と適切な修正方法（推奨）

- **推奨（案 A・正典 item 8 の「既存行 skip」）**: `searchCard` で書き込み前に `SELECT 1 FROM galleries WHERE gallery_id = ?` を行い、既存行なら read-only（commit 0）、行が無いときのみ 1 件書く。card page（search batch を経た表示）で card 側 commit 0・page 合計 1 commit。card 単独経路（search batch を経ない restore flow の card fetch）は行生成の義務が残るため 1 件 1 commit（初回のみ）。
- 案 B（server 側 batch・debounce 寄せ）: card は 1 リクエスト 1 書込みなので request 間で batch 不能。バッファ化は commit 遅延と shutdown 時の書込み消失リスクを追加するため不推奨。
- 案 C（1 request 内 tx 化）: 1 リクエスト 1 書込みでは tx と autocommit が同じで commit 回数不変。効果なし。

### 副作用・影響範囲・代替案

- `last_seen` 意味: 案 A では card 閲覧が `last_seen` を更新しない（card 経路 read-only）。実測で card 経路は既存行の `last_seen` を実際に更新している（＝read ではなく write）。`last_seen` の読み出しは app 内不存在（INSERT/UPSERT のみ、SELECT 使用箇所なし）→ user-visible な影響なし。データ意味は「write 経路で記録された時点」へ狭まる点のみ caveat。`upsertGallery`（`app/library.go:10-17`）の meta 経路は非接触。
- `read_state`・行数生成の義務: `read_state` の FK は `galleries` を参照（`schema.sql:61`）。実測で行無し insert は FOREIGN KEY constraint failed。案 A は「行が無いときのみ書く」を維持するため義務不変。UI の read/fav/acquire 経路の `recordDiscovered`（`app/viewer.go:951`、`975`、`1008`、`1037`、`app/acquisition.go:157`）は書込み意図そのものなので非接触。
- `check` 新着判定: 現行 code は `bookmark_seen` 基準（`app/check.go:7-14`）で card 経路の書込みと無関係。A-9 記録（`docs/ai/tasks/2026-11-02-external-code-review-findings.md`）の「galleries は discovered 記録として残せる」方針と整合（card 経路の read-only 化は新着判定を変えない）。
- A-02/C-03 正典記録照合: R-09 は A-02/C-03 の残存点であり、決定の supersede ではない。search 側 batch 意味（fetch→tx 順序、empty ids commit 0）は不変。
- UI 意味（card 表示の local 数・TN cover・x-cache）は不変。画面確認は UI 変更がないため該当なし（未確認でなく「該当なし」）。

推定規模: 小（`searchCard` handler 1 箇所の existence check + commit 回数固定テスト 1 件。schema・`recordDiscovered` 本体変更なし）。

### 実施（assess-20261106-1056・2026-11-07）

- 採用: 案 A（正典 item 8 の既存行 skip）。実装:
  - `app/main.go`: `cardStore`（`execer` + `galleryExists`）と `sqlCardStore`（`*sql.DB` からの橋渡し・`sqlTransactor` と同形状）と `recordDiscoveredIfMissing` を追加。`galleryExists` は read 口（commit に数えない）。fake が `*sql.Row` を作れないため、fake が満たせる interface に限定（既存正典 comment `app/main.go:235` の「fake は Exec だけで足りる」を read 口へ拡張する実装細部。`recordDiscovered` 本体・search batch semantics は非変更）。exists 照合失敗時は行生成義務側（1 件 write）へフォールバック。
  - `app/viewer.go:647`（`searchCard` handler）: `recordDiscovered` 直書き → `recordDiscoveredIfMissing(&sqlCardStore{v.db}, id, galleryDir(v.root, id))`。
  - `app/main_test.go`: 既存 fake harness に `fakeExec.galleryExists`（`exists-read` event・commit に数えない）を追加。新テスト `TestSearchCardRecordDedupSkipsExistingRowsMockedDB`（search batch 1 commit 後の card 25 件 = 0 commit・exists-read のみ、cold 単独経路 = 初回 1 件 1 commit・2 回目 0 commit・行数 27）。
  - `app/viewer_test.go`: 新テスト `TestSearchCardHandlerIsReadOnlyForExistingRow`（real DB で handler 配線確認: batch 済み既存行の `last_seen` を sentinel へ更新後 handler → `last_seen` 変化なし＝read-only、cold id は handler が 1 行生成＝FK 義務維持）。
  - 既存テストの変更なし。schema・`recordDiscovered` 本体・read/fav/acquire 経路・`upsertGallery`・UI 非接触。
- 試行 1: 新テスト cold 2 回目 pass 前に `counter.reset()` を省略（テスト側 mistake・2 commits）。試行 2: reset 追加で合格。生産コードの変更は試行 1 以降なし。

## Verification

- 実施後の正典検証（合格証拠):
  - `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app` **28.317s**（試行 2）。試行 1 は新テスト内の `counter.reset()` 省略で 1 failed（テスト側 mistake・原因分析を「実施」に記録）、修正後 PASS。合格条件の実測: search batch 1 commit 後 card 25 件 = **0 commit**（exists-read のみ）、cold 単独経路 初回 **1 件 1 commit**・2 回目 **0 commit**、行数 27。
  - `python -m pytest tests/ -v`（root）: **56 passed・1 failed（14.30s）**。failure は既存 known-issue `KI-20261106-04-canonical-build-freshness-pointer-missing.md`（`test_canonical_build_freshness_rule_exists`）。R-09 は AGENTS.md 非接触・本 failure は本項目不合格ではない（R-09 由来 failure 0 件）。
  - 画面キャプチャ: 該当なし（server 側 commit 回数のみ・UI 変更なし）。保存先なし。
- スクラッチ実測（`docs/ai/tmp/assess-20261106-1056/r09/r09_measure_test.go`・一時 DB・既存テスト同形状 harness。正典 SQL は `app/main.go:1344-1347` の逐語コピー。commit 定義は `app/main_test.go:1407-1417` と同文）。外部 API 呼出しなし（この項目で不要）。
  - 現行 shape: search batch = **1 commit**、card 25 個別リクエスト（tx 外 Exec・autocommit）= **25 commit**、page 合計 **26 commit**。rows 25 永続化、`journal_mode=wal` 確認。card 経路は既存行の `last_seen` を実更新（write であることの確認）。
  - 推奨案 A（既存行 skip）: batch の後 card 25 件 = **0 commit**、page 合計 **1 commit**。行数不変、`last_seen` は batch 時時点のまま。
  - card 単独経路（cold・search batch なし): 初回 **25 commit**（行生成義務）、2 回目 **0 commit**。
  - FK 実測: 行無しで `read_state` insert → `FOREIGN KEY constraint failed`（`schema.sql:61`）。skip 案は行が無いとき書く分岐を維持する必要があることを実測で確認。
  - wall time 参考（in-process modernc・unstable・既存注記 `app/viewer_test.go:1484` と同基準）: current card phase 528.8µs（25 commits）、skip-existing card phase 999.6µs（0 commits）。commit 回数の改善は harness 定义で固定済み。
  - 経過: `go vet . && go test -count=1 -timeout 300s .`（scratch r09）PASS（0.08s + 0.04s、module 計 1.569s）。DB close 追加前の 1 回目の実行は TempDir cleanup 失敗（テストロジックの assert は合格）、close 追加で PASS。
- 正典検証（既存テスト変更なし・読み取りのみ）:
  - `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app` **25.459s**。
  - `python -m pytest tests/ -v`（root）: **54 passed・1 failed（9.95s）**。failure は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md の decision pointer 状態）。R-09 は code・UI を変更しておらず無関係。R-09 由来の failure は 0 件。
- 作業前チェック（viewer 接触項目として読了、AGENTS.md 必須）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`、加えて正典 item 8 の参照先 `docs/ai/decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md`（item 8）と A-02 記録 `docs/ai/tasks/2026-11-03-a02-record-discovered-commit.md`。
- 画面キャプチャ: UI 項目ではない（server 側 commit 回数のみ・UI 意味変更なし）ため該当なし。保存先なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
