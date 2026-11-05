# KI-20261105-06 旧形式 galleryinfo 数値フィールド parse 失敗（assess-20261105-1817）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 実装承認（推奨: flexString flexible decode + `TestGalleryParse` に数値形式 fixture 追加）

## 意図 / 背景

- 実行固有ID: `assess-20261105-1817`（assess 評価実行。統括記録: `docs/ai/tasks/2026-11-05-assess-20261105-1817-items.md`）
- 項目原文: `docs/ai/known-issues/KI-20261105-06-old-format-galleryinfo-numeric-parse-fail.md`
- 本タスクは assess-items.md 手順による調査・評価専用。ソースコードは変更しない（変更してよいのは本記録のみ）。実装承認は済んでいない（known-issue の「修復は別承認」維持）。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- decode 層（正典名は `rawGallery`。項目原文の `parseGallery` はこの呼出の意図。実体は `Client.Gallery` と `persistedGalleryInfo`）:
  - `app/hitomi.go:658-671` の raw struct は `ID string`・`Tags/Artists/Groups/Characters []map[string]string`。value の number 受入（flexible decode）なし。
  - live 取得→parse は `Client.Gallery`（`app/hitomi.go:690-714`）。`json.Unmarshal` の field 型不一致で失敗。`id` は `strconv.Atoi(raw.ID)` で数値化（`flexString` 化後も維持可）。
  - viewer card 経路: `searchCard`（`app/viewer.go:604-624`）→ `v.info` memory→disk→remote（`app/viewer.go:1075-1106`）→ error 時 `searchResult` は `{"id","error","tags"}` を返す（`app/viewer.go:626-630`）。error card には `read_state` が入らないため `completeCardState`（`app/search.html:457-467`）が map を補完できず badge 空。
  - mechanism 補正（項目原文の微修正）: `renderReadBadges()`（`app/search.html:477-487`）は error card の `li[data-search-id]` も走査対象で、map が補完済みなら error card でも badge は描画される。実際の根因は「error card 反映 skips」ではなく「parse 失敗により map 補完源（card.read_state・lists）が得られない」こと。badge 表示は parse 修復のみで復帰し、UI 側の追加修正は不要（コード経路読みによる判断）。
  - 影響範囲は UI に限定しない: CLI `gallery`・`download` も同一 `client.Gallery`（`app/main.go:1025`・`app/main.go:1042`）で、旧形式 gallery は実測上全体失敗する（badge より重要）。
- disk 永続層（`app/galleryinfocache.go`）は parse 済み string 形（`persistedGalleryInfo`）を保存するため影響なし。壊れたファイルは miss 扱い・除去の既存流儀（同:31-46）で修復と無関係。

### 実測（実測で閉じた）

live galleryinfo 応答（2026-11-05 実測・正典 domain `ltn.gold-usergeneratedcontent.net`。`ltn.hitomi.la` は getaddrinfo 失敗。data call 1.1s 間隔・rate limit 遵守）。scratch: `docs/ai/tmp/ki06-live-galleryinfo.py`（live 応答 JSON を `docs/ai/tmp/ki06-<id>.json` に保存）。

| id | `id` 型 | `tags[0]` | Go decode（同じ形の raw struct） |
|---|---|---|---|
| 296403 | int | `female` が int(1) | error: `rawGallery.tags.0.female of type string`（item と同一文） |
| 293004 | int | `female`/`male` に int 含む | error: `rawGallery.id of type string` |
| 241311 | int | `female` int(1) | error: `rawGallery.id` |
| 238019 | int | `female` int(1) | error: `rawGallery.id` |
| 236142 | int | `female` int(1) | error: `rawGallery.id` |
| 2299308 | str（正常系） | 全 string | ok |

Go 再現は `docs/ai/tmp/ki06-gocheck/`（`go run .`）で項目原文の error 文を一致再現。旧形式（数値）と新形式（string）の分岐は実測で確定。

### 有効性

有効（実測確定）。旧形式 live 応答の number 値が現行 string 型 struct で decode 失敗、card error 描画・badge 空・CLI download 失敗の全症状が根因単一（decode 層）。

### 対応の実現可能性と推奨修正方法

- 推奨: 小規模 flexible decode。
  1. `flexString`（string/number 両受入・number は字面 string 化）を `rawGallery.ID` と tag namespace map の value 型へ適用。`strconv.Atoi(string(raw.ID))` は変更不要で継続可。
  2. `names()`（`app/hitomi.go:673-688`）は意味不変: 数値 `female: 1` → string `"1"` は truthy で `female:<tag>` 出力同一。数値 0 も string 化すれば既存 string 形と同一挙動。
  3. test anchor: `TestGalleryParse`（`app/hitomi_test.go:783-810`）の mock fixture に数値形式バリエーション（id 数値・tags value 数値）を追加。mock 維持方針（decision 2026-11-01）維持・live fixture は scratch で可。
- 代替案（不採用）: `json.RawMessage` 全 field 化・`UseNumber()` post-process。string 型 field で先に失敗するため post-process 案は decode 失败そのものを回避できない。`interface{}` 受入は型意味を弱め既存 convention（string 正典）とズレる。

### 副作用・影響範囲

- decode 層のみ。full image/TN・`imageProxy` 順序・永続先・`x-cache` guard は不変（AGENTS.md「絶対に壊してはいけない条件」に触れない。実装時は viewer/download 接触タスクの作業前チェック 3 decision 参照と `go vet . && go test -count=1 -timeout 300s .` + 実機確認が適用）。
- 永続 disk cache の既存 entry（parse 済み string 形）は再 parse 不要。失敗 gallery の entry は存在しない（失敗時点で書込み前のため）。修復後の live 成功で正常に永続化。
- 正典ルール変更なし。UX の意味変更なし（badge・card 表示は既存仕様の復帰のみ）。

### UI 画面確認

- 未確認。アプリ未起動（`127.0.0.1:8731` へ HTTP 応答なし・2026-11-05 確認）。子制約（ソース変更禁止・評価専用）により正典 build の起動は評価スコープ外の行为（実装受入行為）として実行しない。Google Drive への保存は実施せず。UI 症状（card error 本文・badge 空）は項目原文の runtime 確認時点で観察済みで、上記 mechanism 補正と実測により有効性判断は画面再確認に依存しない。

### 推定規模・優先順位付け要素

- 推定規模: 小（`flexString` 約 10 行 + `names()` 型適応 + fixture 追加 + 検証再実行。テスト含め 1〜2 h 相当）。
- 重要度: 中（badge 表示だけでなく CLI `gallery`/`download` の旧 gallery 全体失敗に影響）。
- 緊急度: 低〜中（旧 gallery 部分集合。jump 機能の合格条件は影響なし）。
- リスク: 低（decode 層のみ・正典不変条件・永続 cache 流儀に触れない）。
- 依存: 実装には別の実装承認が必要（項目原文維持）。正典 decision・AGENTS.md 変更なし。テスト方針（external API 呼出し可）は既存の `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md` を参照し遵守（実測は scratch script で実施）。正典文書間の状態記述矛盾は検出なし（採用正典: decision 2026-11-01・normal-screenshot-rule・screen-confirmation-size-rule）。

## Verification

- 本タスクは記録のみ（ソース無変更）。正典検証（pytest・go vet/test）は適用外（docs/ai のみ変更条の例外）。
- 実測コマンド（2026-11-05）:
  - `python docs\ai\tmp\ki06-live-galleryinfo.py` — live 6 gallery fetch（1.1s 間隔）。結果表は上記。保存: `docs/ai/tmp/ki06-*.json`。
  - `go run .`（workdir `docs\ai\tmp\ki06-gocheck`）— 項目原文の error 文一致再現（正常系 2299308 は ok）。
  - `curl.exe -s -m 3 http://127.0.0.1:8731/api/lists` — 応答なし（UI 未確認の根拠）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
