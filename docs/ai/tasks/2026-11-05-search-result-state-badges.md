# 検索結果画面の表示済み・既読・保存済み表示とフラグ手動リセット（案検討・計画）

**Status**: 進行中（案検討・計画のみ。実装未開始）
**Date**: 2026-11-05
**Next Action**: 判断点1回答（route 入力時点・クリックフラグは不採用案）・2確定。3・4の推奨への承認 → 実装開始指示待ち

## 意図 / 背景

- 検索結果画面の card に次の3状態を表示する:
  - **表示済み**: 一度でも閲覧画面を開いた
  - **既読**: 最後まで見た
  - **保存済み**: 指定フォルダへの保存を完了した
- 表示済み・既読は手動リセット可能にする。
- 本記録は案検討と計画のみ。ソースコードは無変更（実装開始ゲート未通過）。

## 調査結果

### 現状の状態記録

- `read_state` は `last_page` / `fully_read` / `opened_at` / `favorite` の4列のみ（`schema.sql:60-66`）。**表示済みフラグは独立に存在しない**。
- `setReadState` はページ到達時に `last_page` と `opened_at` を更新し、`complete` のとき `fully_read=1`（`app/library.go:25-49`）。`opened_at` は毎回上書き（初回開封時刻ではない）。
- `setFavorite` は `read_state` 行を作らないとお気に入りが記録できず、行を作成する際 `last_page=0`・`fully_read=0` で写入（`app/library.go:51-61`）。**行の有無＝表示済みとは区別できない**。
- 正典メモリでは read_state は「開いた」ではなく「ページ到達」で更新されると明記（`docs/ai/usage-memo.md:49`）。
- 検索結果 badge は `badgeText(id)` が maps から読む（`app/search.html:178-188`）。maps は `showLists()` が `/api/lists` から構築（`app/search.html:413-442`）。
- **badge の gap（実測事実）**: `listRows("unread")` は `WHERE fully_read = 0`（`app/main.go:1226-1236`）。そのため `fully_read=1` の gallery は favorites に無い限り maps に入らず、**既読 badge が表示されない**。現状 badge は `お気に入り` と `p<N>` のみ。
- card API は `read_state` を `readStateRow` で直引して返す（`app/viewer.go:633-652`）。filter がないためこの経路なら既読状態を正しく取得できる。現状 search page は card の `read_state` を badge に使っていない。
- 保存状態は card API が entry ごとに `entryLocalFormats` で disk 確認済み（`app/viewer.go:695-707`、`:640-644`）。`local` 計数は `localCount(id)`（`app/viewer.go:1045-1053`）。保存先は `gallery_dir`（既定 `Galleries`）内の `<id>/<hash>.<format>`（`app/main.go:873`、`app/main.go:606`）。`files.path` は root 相対（`app/main.go:662`）。

### 表示・書き込みの既存規約（変更してはいけない前提）

- 一覧の所有ページは検索ページのみ。閲覧ページは `showLists` を呼ばない。ページ間同期（BroadcastChannel）は第1版なし、表示更新は手動で許容（`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` 決定1）。
- `showPage()` の表示経路は read post を await しない。read 書き込みは generation-guarded debounce helper 内（`app/viewer.html:1056-1080`、test `viewer_test.go:1763`・`1893`）。
- card は固定高さ `--search-card-height: 130px`・`overflow: hidden`（`app/search.html:24-26`）。strict screen confirmation の card height contract（`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 項目4）により、badge 行へ行を追加して card 背景が伸長する設計は不合格。
- schema 迁移は非破壊（列追加のみ・既存行削除しない）。正典 pattern: `ensureScanCacheSizeMtime`（`app/main.go:333-367`）、`schemaVersion = 5`（`app/main.go:225`）。

## 案の検討

### 論点A: badge の状態データ源

| 案 | 内容 | 長所 | 短所 |
| --- | --- | --- | --- |
| A1 | `/api/search` に current page ids 分の `states` join を追加 | 1クエリ・card 読み込み前に badge 確定 | 保存判定に必要な entry 数が search 応答には無い（galleryinfo を引く必要がある） |
| A2 | card API（`searchResult`）が `viewed` / `saved` を返す | 既存の entry local ループで計算可能（追加 I/O なし）。`readStateRow` は filter なしで既読を正しく返す。badge gap 解消 | badge 確定は card load 後（現行 card metadata と同じタイミング） |
| A3 | `/api/lists` の `unread` filter を外して全 read_state を map 化 | 既存 test 形状が近い | 「未読一覧」の意味が変わる。保存判定は別途必要 |

**推奨: A2**（badge は card 由来）。既存 maps は sidebar 一覧用に維持し、card 応答で map を補完する形。

### 論点B: 表示済みの記録時点

- B1（推奨）: 閲覧画面で gallery を開いた時点で `viewed=1`・`viewed_at`（初回のみ）を書く。書き込み口は `POST /api/viewed`（`withPOSTSecurity` を通る）。viewed は表示経路を block しない（generation guard を維持）。
- B2: read post の行有無を viewed と解釈。`setFavorite` が作る `last_page=0` 行と区別できないため**採用不可**。

定義の分岐（要承認）: 「開いた」を route 入力で見るか、reader が gallery data を解決して initial page を描画した時点にするか。失敗開封（galleryinfo fetch error） viewed 扱いにするかが変わる。推奨: **描画到達時点**（失敗開封は viewed にしない）。

### 論点C: 既読の判定

- C1（推奨）: reader が最終 page（scene_indexes の最後、または `files[]` 最後）に到達した時点で `fully_read=1` を auto 記録。手動「読み完了」button は override として維持。
- C2: 現状どおり手動のみ。

C1 の残る定義点（要承認）: 最終 page に「到達」だけで良いか、その page の画像が `no_full` で表示出来なかった場合も含めるか。既存の read_state はページ到達基準（`usage-memo.md:49`）なので、推奨は**到達のみ**。

### 論点D: 保存済みの定義

- D1（推奨）: gallery の全 entry に指定フォルダ内の local ファイルがある（`entryLocalFormats` が1つ以上）。部分保存中は `保存 N/M`（現行 meta の `local N` を拡張）。
- D2: `files` 行数 == entry 数。scan 記録依存（download 時は scan 実行されるので満たす）だが、card ごとにクエリ追加。

推奨 D1：追加 I/O なし、表示源の `local` セマンティクスと一致。D2 は CLI 側の代替として記録。

### 論点E: 手動リセット

- 配置: 推奨は検索 card の badge 行**内**（同一行に inline button、card height contract を越えない）。viewer nav への追加は範囲拡大なのでオプション。
- API: `POST /api/reset`（`{gallery_id, target: "viewed" | "read"}`）。CLI: `read --id --reset-viewed` / `--reset-read`。
- cascade（要承認）: `fully_read` は viewed を含意する。推奨規則:
  - `reset-viewed` → `viewed=0`・`fully_read=0`・`last_page=0`（状態を最初へ）
  - `reset-read` → `fully_read=0` のみ（viewed・last_page は保持）

## 計画（実装手順・未実施）

1. **schema**: `read_state` へ `viewed INTEGER NOT NULL DEFAULT 0`、`viewed_at TEXT` を追加（`schema.sql` 定義 + `migrateSchema` の列追加、`schemaVersion` 5→6）。非破壊（既存行保持・D-03 規約）。
2. **library.go**: `readStateRow` に viewed/viewed_at を追加。`setViewed`（初回のみ `viewed_at` 写入・以降上書きしない）。`resetReadState(db, id, target)`。`setReadState` は viewed を保持、`setFavorite` は viewed を立てない。
3. **viewer.go**: `searchResult` に `viewed` / `saved`（+ `saved_count`）を追加（既存 loop で計算）。`POST /api/viewed`・`POST /api/reset` を `handler()` へ追加。
4. **viewer.html**: `openGallery()` で viewed を記録（display path を await しない・generation guard）。最終 page 到達時に debounced read helper 内で `complete: true` を送る（`showPage` 表示経路には post を入れない）。
5. **search.html**: `badgeText` を 3フラグ + お気に入りに拡張。badge 行 inline に reset button（`data-reset="viewed|read"`）。card 応答の state で maps を補完し `renderReadBadges()` で反映。`lists-refresh` は維持。
6. **main.go CLI**: `read` へ `--reset-viewed` / `--reset-read`。`list` の `viewed` kind 追加は任意（範囲外として保留）。
7. **テスト**:
   - 新規: schema migration（user_version 6・既存行保持）、`setViewed` 初回のみ timestamp、reset cascade、`searchResult` の saved/viewed、`POST /api/viewed`・`/api/reset` handler、最終 page auto complete。
   - re-anchor 必要（現行 assertion が card 由来 badge に動く）: `viewer_test.go:1632-1651`（badge maps）、`:2005-2088`、`:2059-2075`（card read_state）、`:167-213`、`:2986-2993`（complete handler）。`main_test.go:92-118`（read_state 列追加）。
   - `tests/read_lists_debounce_harness.mjs` は search.html の showLists block を参照するため、badge 源変更で anchor 確認が必要。
8. **正典 build / 画面確認**（ポインタ正典 `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md`、`docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`、`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` 準拠）:
   - fresh tab・direct server baseline と UI response の一致
   - 3状態の実データで badge 表示確認: ①viewed・未既読 ②既読 ③保存済み（同一 gallery で3条件を満たす実データ）
   - 部分保存は `保存 N/M`。reset button click → API → DB 行変化を direct SQL で照合
   - card height contract（130px）維持、`.side` 480px、既存回帰（TN cover・generation guard・no-store fetch・full image path・reader nav・favorite/unread）

## 判断保留（ユーザー承認が必要）

1. 表示済みの記録時点: route 入力 or 初回描画到達（失敗開封の扱い）
2. 既読の auto 判定（最終 page 到達のみ）を許容するか、手動「読み完了」のみとするか
3. 保存済みの合格条件: 全 entry 保存完了のみ（部分保存の表示は meta に残す）
4. reset の cascade 規則（viewed リセットは fully_read も消す）と reset 配置（検索 card inline か viewer nav か）

### 回答（2026-11-05 03:26）

- **2 は確定**: 既読の auto 判定（最終 page 到達）を許容。手動「読み完了」button は override として維持。
- 1・3・4 は本計画の推奨を提示（承認待ち）。

#### 1 の推奨（新規タブ条件を受けて更新 2026-11-05 03:28）

**新規タブ開封で問題はない**。search card の link は `viewerLink()` が `/#/g/<id>` へ absolute 向け、`targetAttr()` で新タブが開く（`app/search.html:121-128`）。viewed の書き込みは閲覧側 document（viewer page）が `POST` で行うため、新タブでも同一事件として記録される。search page 側は表示 refresh を待つだけ（ページ間同期機構は第1版なし・正典決定1、手動 refresh 許容）。

採用する基準: **`openGallery()` の route 入力時点**（`app/viewer.html:1113` — await 前に `history.replaceState` で route を入れる）。
- 新規タブ・同一タブ・route 復元 reload のすべてで event が捕まる。
- 描画到達基準だと、galleryinfo fetch 失敗や `page out of range`（`app/viewer.html:1087-1090` — `runBackgroundRead` 前に return）で**開いたのに記録されない**。ユーザー定義「一度でも閲覧画面を開いていれば」に忠実なのは route 入力基準。
- 同一 gallery の reopen は idempotent（`viewed_at` は初回のみ、`viewed=1` の再写入のみ）。
- 書き込み口は `POST /api/viewed`、display path を await しない（fast-nonblocking・generation guard 維持）。

**リンククリック済みフラグ（`clicked`）は新設しない。**
新設が不要な理由: 新タブ開封は viewer document の load 事件として route 入力で捕まるため、click flag の優位性がない。click flag が拾えるのは「クリックしたが viewer page が load されなかった」ケース（popup blocker・背景タブを開いて即閉じる・right-click open）だけで、これは「閲覧画面を開いた」とは言い難い。新設する場合は false positive 源を明示する必要がある:
- `li.card` 内 link 以外（同名日本語版 anchor `app/search.html:175`、sidebar favorites/unread link）も click になる
- 中ボタン・右クリック・keyboard link activation
- search page 側が viewer 定義の状態を書くことになり、一覧所有ページの分離（正典決定1）の ownership が混ざる
`clicked` を採るなら `viewed`（viewer document 事件）と分離し、badge は `viewed` を優先・`clicked` は「開封試行」として別表示に限る。

#### 3 の推奨

badge「保存済み」は全 entry 保存完了のみ。部分保存は badge に出さず meta 側の `local N/M`（現行 `local N` を分子/分母へ拡張）で示す。
理由: ユーザー定義「保存を完了したもの」に忠実。部分保存を完了として badge 表示すると誤判定になる。card height contract（130px）内で badge 行を増やさない形を維持できる。

#### 4 の推奨

- cascade: `fully_read` は `viewed` を含意するため、`reset-viewed` → `viewed=0`・`fully_read=0`・`last_page=0`（状態を最初へ）。`reset-read` → `fully_read=0` のみ（`viewed`・`last_page` 保持）。
- 配置: 検索 card の badge 行 inline（同一行・`data-reset="viewed|read"`）。viewer nav には追加しない（一覧の所有ページは検索ページ、`docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` 決定1）。リセット後の表示反映は既存の手動 refresh（`#lists-refresh`）で許容。

## Verification

（未実施。実装後 `go vet . && go test -count=1 -timeout 300s .`（app）と `python -m pytest tests/ -v` を記録）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
