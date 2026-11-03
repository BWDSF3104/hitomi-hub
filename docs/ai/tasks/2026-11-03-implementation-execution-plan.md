# assess-20261103-1220 残項目の実装実行計画（サブエージェント1件ずつ・優先順）

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 第7実行完了（A-13 7144dd1・A-06 c02bf83・C-10 a58e352・D-07 aac4767・D-06 77181d5・委譲上限5件）。次の実行は新セッションで② A-11→A-09、その後③判断項目。ユーザー判断要: AGENTS.md Code Scope「UI未実装」とREADME/実装の矛盾（D-06判断持ち帰り）。D-07残件: clean clone skip実発火未実施（scratch模拟）

## 意図 / 背景

- `2026-11-03-assess-20261103-1220-implementation-targets.md` のとりまとめ（優先順位・設計承認3件の条件）を実行計画として取りまとめる。
- 実施方式: 1項目 = 1回のサブエージェント（general）。サブエージェントは同時に最大1つ（global 正典）。1項目完了後に次へ、優先順どおり。
- 1項目ごとに: 個別タスクファイル・単独検証・単独 commit（commit はユーザー承認前提）。
- フル画像ガード関連項目は AGENTS.md「絶対に壊してはいけない条件」優先。作業前に 2026-11-01 の 2 decision（root-and-tn-display-correction-policy、fast-nonblocking-viewer-policy）と `2026-11-01-ggset-generation-and-drift.md` を読み、タスク記録にファイル名で明記する。
- 完了判定は実施確認に基づく。確認できない項目は「未確認」として記録し、完了と書かない。

## 実施方式（各項目共通）

1. サブエージェントは個別タスクファイルと関連コードを読ってから実施する。
2. 合格条件は個別タスクの判定記録（実測根拠）に基づく。テスト追加・更新を含む。
3. 検証: `app` で `go vet . && go test -count=1 -timeout 300s .`、ルートで `python -m pytest tests/ -v`（`test_canonical_build_freshness.py` 含む）。UI・Go 変更後は canonical exe 再 build、canonical server（既定 `http://127.0.0.1:8731`）からの実機確認（Playwright）。
4. 実機 UI 確認（B-01・B-03・B-09 など実操作必須項目）はブラウザツールを持たないサブエージェントでは私が実施する。
5. 個別タスクファイルの「調査結果 / 変更内容」と Verification を即更新し、項目完了時に正典 `task_manager.py end` で閉じ、`current-task.tsv` 参照行を削除する（plan タスクの実施 progress に1行追記）。
- 実施順の取りまとめ完了（本タスクの範囲はここまで）。**実施開始はユーザーの明示的な指示後**（2026-11-03 指示: 開始してはならない。B-08 のサブエージェント起動は取消、code change なし）。commit は項目ごとに単独。global 正典どおり、変更の詳細についてユーザーの事前承認を前提とする。
7. 使い捨てスクリプト・スクショは `docs/ai/tmp/assess-20261103-1220/`。インラインコード実行禁止、`Get-Content`/`Set-Content` パイプ禁止。

## 実施順

### 実施済み（優先1–3・解消済み）

- 実装順 1–3（設計承認3件・単独 commit）: A-08 `683ff47` → C-08 `1b8bb30` → A-07b `fa10536`
- 優先1 A-05/C-01/C-02 `c77f92a`、優先2 B-02 `622fadc`、優先3 A-01/B-11 `55b9f83`
- D-05 解消済み close `621056f`

### 残項目（1項目ずつ順次）

| # | ID | 実施対象 | 備考 | 状況 |
|---|---|---|---|---|
| 4 | B-08 | zip path traversal を path segment `..` 判定へ修正。YARA規則と構造判定を揃える | Go（scan.go）・YARA規則（Python 参照実装に同款があれば揃える） | 未実施（サブエージェント起動は取消。開始指示待ち） |
| 5 | D-03 | findings unique に `path` 追加。Go作成DBとcanonical DBで unique index を揃える | 既存DB重複確認・マイグレーション検証 | 未実施 |
| 6 | D-04 | `IsPE` を `e_lfanew` 参照へ修正 | Go | 未実施 |
| 7 | B-01 | `form#search`/`form#nav` の prev・next・complete・fav に `type="button"`。Enter 暗黙 submit を検索固定 | UI実操作必須 | 未実施 |
| 8 | B-03 | fav/complete cache state を toggle ごとに更新。cache 巻き戻し防止 | UI実操作必須 | 未実施 |
| 9 | B-09 | `newTabMode`・修飾キー click で `preventDefault` しない | UI実操作必須・方針確認必要 | 未実施 |
| 10 | B-05 | `hitomi` キーなし config で `defaultHitomiConfig()`。snapshot fallback を黙って起こさない | フル画像ガード関連 | 未実施 |
| 11 | A-15 | `isLoopbackHost` の `[::1]:port` 判定修正 | Go | 未実施 |
| 12 | A-10 | `downloadFilesParallel` 既存ファイル確認。skip と remote fetch 回数修正 | CLI download 実在 gallery 確認必須 | 未実施 |
| 13 | A-07a | cache write 前に image magic/format 検証 | フル画像ガード関連 | 未実施 |
| 14 | B-06 | VT 空 `last_analysis_results` + positives 0 を `not_evaluated` | 実API確認は key 未保有で未実施（テストで固定） | 未実施 |
| 15 | B-07 | YARA failure を構造 pass に優先させず別 finding/status 分離。export/pdf skip 条件維持 | Python | 未実施 |
| 16 | B-10 | Python 参照実装 VT の `HTTPError` handling、404 `unknown_to_vt` 実経路 | Python | 未実施 |
| 17 | A-02/C-03 | `recordDiscovered` per-call commit を batch/transaction 化。実測根拠をテスト固定 | Go | 未実施 |
| 18 | A-03 | `json_ms` 計測修正、metrics 二重返却修正。テスト更新 | Go | 合格・close済み（単独commit） |
| 19 | A-14/C-05 | `searchCache`・`indexDataCache`・`indexNodeCache`・`infoCache` に上限・削除・TTL | Go | 未実施 |
| 20 | C-04 | `language:` 併用時の同一 tag 二重 fetch 排除 | Go | 未実施 |
| 21 | C-06/C-07/C-09 | strip rebuild・resize/fullscreen・read/lists の debounce と generation guard 整理 | フル画像ガードに触れる可能性 | 未実施 |
| 22 | A-11 | PDF `rawRGB` 全ページ分保持の解消（上限 or streaming 評価） | Go | 未実施 |
| 23 | A-04/D-09/D-10 | hitomi index key / sort convention 確認後に判定（判定保留） | hitomi 準拠確認（調査） | 未実施 |
| 24 | A-06 | `time.Sleep` を ctx-aware 化 | Go | 未実施 |
| 25 | A-09 | `serveStoredImage` 全読み・Range・304・Content-Range 評価（必要なら部分的 stream/conditional） | Go | 未実施 |
| 26 | A-13 | `ScanPDFKeys` の `string(data)` key allocation 回避 | Go | 未実施 |
| 27 | C-10 | `go test` gate sleep をテスト専用 config で 0 にする案の評価のみ（本番ゲート間隔を変えない） | 案評価のみ | 未実施 |
| 28 | D-01/D-02/D-08 | `.gitignore`、config.example 秘密扱い、production 未使用コード整理 | trivial | D-02実施済み（単独commit）。D-01/D-08未実施 |
| 29 | D-06 | gg case 方向、TN fallback 古記述、check 説明、README UI 記述修正 | docs 修正は承認後 | 未実施 |
| 30 | D-07 | `test_canonical_build_freshness.py` の clean clone / CI 前提固定 | tests | 未実施 |
| 31 | A-12 | SHA256 全件毎回計算（原優先表外・個別タスクあり） | 低優先 | 未実施 |
| 32 | B-04 | acquisition フォルダ名と確認処理の不一致（原優先表外・D-02 関連） | 低優先 | 未実施 |

## 実施 progress

（1項目実施ごとに1行追記: ID・commit・検証結果・未確認点）

- B-08 合格 commit `865c325`（push 済み）。基準: entry 名を `/` `\` で分割し segment 完全一致の `..` のみ hit。実測 `Vol...01.webp` → pass（Go/Python/YARA 非 hit）、`../evil.exe` → hit。go vet+test ok 36.919s、pytest 26 passed。個別記録は完了。残件（判断要）: 記録記載の `containsTraversal(name)` は実際 `app/export.go:174`（部分一致のまま・承認範囲外・未変更）→ **解決（2026-11-03 ユーザー承認）**: segment 判定の共有ユーティリティへ統一することを承認し、追加項目として実施（単独 commit）。
- D-03 合格 commit `8962fcb`（push 済み）。unique key を `(path, sha256, rule, detail)` へ、Go `ensureSchema` に unique index 追加（schemaVersion 4、迁移は定義置換のみ・行削除禁止）。実測: 新規 Go DB で scan 2回 → findings rows 2→2 不変（INSERT OR IGNORE skip）、Go 作成 DB・canonical DB で key 一致。go test ok 37.201s、pytest 29 passed。個別記録は完了。残件（判断要）: ① 調査用 `repro.db`（rows=20・新 key 重複 group=5）は行削除なしで index 作成不可（copy で失敗実測、original 未変更）→ 重複除去か再生成の判断。② root `hitomi.db`（rows=8・旧 key）はスコープ外で未変更。③ 非破壊迁移の設計判断を `docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md` に記録（旧暗黙 dedup を supersede）。→ **解決（2026-11-03 ユーザー承認）**: 拒否方針（c）で確定、暗黙 dedup へは戻さない。`repro.db` は `kilo-safe-remove` でゴミ箱へ（掃除）。root `hitomi.db` は次回 `ensureSchema` 実行で揃う（今すぐ手入れ不要、次回 canonical server 起動時に確認）。
- D-04 合格 commit `38fb12b`（push 済み）。`is_pe` を `e_lfanew` 参照（seek）へ。実測: `e_lfanew=0x80` fixture（PE signature at 0x80）で `mz_without_pe_header` 非 hit（修正前は hit をスクラッチ `docs/ai/tmp/assess-20261103-1220/d04_fixture_check.py` で再現）、回帰 `0x38` pass・`0x80` signature なし hit・`e_lfanew=0` hit 維持。pytest 33 passed 3.35s（freshness 含む）。個別記録は完了。YARA 規則変更なし（規則は `MZ_Header_At_Offset0` のみ、実測上齟齬なし）。実測根拠 `Galleries\102\b.bin` は現マシン不在 → fixture 生成で代替。
- B-01 合格 commit `c1d2c07`（push 済み）。`form#search` の暗黙 submit を `#search-submit` へ固定、prev/next・nav button に `type="button"`。実測（1280x900 fresh tab・canonical exe 再 build + server 8731）: page 2 で `female:catgirl` + Enter → `GET /api/search?...page=2` 200、`#page` 2 維持、prev/next クリックは `setSearchPage` で page 3/2 実測。go vet clean、go test ok 36.930s（固定テスト `TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit`）、pytest 33 passed。スクショ Drive `I:\マイドライブ\kilo\b01-confirm-1280x900-enter-page2.png`。個別記録は完了。残件（判断要）: nav（`#pagenum`）上 Enter の挙動が prev 暗黙発火 → submit event（no-op）へ変化。→ **解決（2026-11-03 ユーザー承認）**: no-op（検索固定）で確定。nav の page 跳转は別項目。`docs/ai/decisions/2026-11-03-b01-nav-enter-noop.md`。

## 実行打ち切り（2026-11-03）→ 再開

- 委譲は実施順4（B-08）〜実施順7（B-01）の4件で停止。B-01 の nav Enter 挙動変更が判断要であり、以後の B-03 は同一 `app/ui.html`（fav/complete handler・form submit セマンティクス）に依存するため、記録から無関係と確認できず打ち切り。実施順8 以降（B-03・B-09・B-05・A-15・A-10・A-07a …）は未実施。
- **2026-11-03 判断解決・再開**: B-08 の export 判定統一・D-03 の拒否方針・B-01 の nav Enter no-op をユーザー承認（推奨案どおり）。B-08 追加項目（`app/export.go` の segment 判定統一）を実施し、以後は実施順8（B-03）から順次委譲。B-09（`newTabMode`・修飾キー click）は方針確認必要のまま保留。

- B-08追加 合格 commit `e1489e0`（push 済み）。`app/export.go:174` の `containsTraversal` を scan 側共有ユーティリティ `containsDotDotSegment`（segment 完全一致）へ統一（`/` 除外分岐と既存挙動は維持）。実測: `Vol...01.webp` false、`..evil.webp` false、`../evil.exe` true、`a/../b.webp` true、`a\..\b.webp` true。e2e 回帰: `Vol...01.webp` を除外せず export（path_traversal skip=0）。go test ok 37.066s、pytest 33 passed（freshness 含む）。注意点: export 生成名（`hashOf`=Base 由来）では単独 `..` segment になり得ないため `../evil.exe` hit の固定は関数レベル（e2e 経路なし）。中断（誤停止）で未コミットだった変更を同一サブエージェントで継続し、検証・記録・コミットまで完了。機械的確認: コミット存在 ✓／ツリー clean ✓／個別記録 Status 完了・Next Action 一致 ✓。
- B-03 合格 commit `d010178`（push 済み）。`#fav`/`#complete` toggle で `searchCardCache`・`galleryCache` の `read_state` を更新。実測（正典 exe を scratch dataDir `docs/ai/tmp/assess-20261103-1220/scratch-run` で 8731 起動・Playwright 実操作）: gallery 4300588 card → 1 click `on:true` → `current.favorite`=1・cache=1・`/api/lists` favorites=[4300588]、同一 card 再 click で巻き戻りなし、2 click `on:false`（DB=1 と方向一致）→ DB=0・cache=0、3 click `on:true` → DB=1・cache=1。スクショ Drive `I:\マイドライブ\kilo\b03-ui-check-2026-11-03.png`（760,610 bytes）。正典 DB にテストデータなし（scratch DB のみ）、canonical server は正典 runtime へ復旧。go vet pass・go test ok 36.862s（固定テスト `TestEmbeddedUIFavoriteToggleSyncsReadStateCaches`）、pytest 33 passed（freshness 含む）。機械的確認: コミット存在 ✓／ツリー clean ✓／個別記録 Status 完了 ✓。判断持ち帰りなし。親の判断で観察記録 `docs/ai/known-issues/2026-11-03-search-api-response-field-ids.md` を追加（検索応答は `ids` を読む。code 変更なし）。
- B-05 合格 commit `574a659`（code・単独）＋ `559e2a8`（記録処理の docs commit 分離）、push 済み。`loadConfig` に `Hitomi: defaultHitomiConfig()` で `UseLiveGG` default true を明示。mock 固定テスト `TestLoadConfigWithoutHitomiKeyUsesLiveGGViaMock`（修正前 revert で FAIL → 条件固定の証拠）、明示 `use_live_gg:false` は snapshot fallback 維持（fetch 0 回）。実機（canonical exe 再 build・scratch dataDir）: CLI download gallery 4301441 attempted=2/written=2 一致、`snapshot fallback` 警告なし、metrics に `gg` group（calls=1）、`/api/image/35fd0c…?format=webp` → 200 `x-cache: miss` 139238 bytes（no_full でない）。go test ok 36.861s、pytest 33 passed。機械的確認: コミット存在 ✓／ツリー clean（記録は親の progress 追記のみ）✓／個別記録 Status 完了 ✓。判断持ち帰りなし（実機確認で canonical exe を修正込みで再 build した旨のみ）。
- B-09（方針確認必要）は判断未解決のため未実施（保留）。実施順9 を飛ばして実施順10・11 を実施した。
- A-15 合格 commit `3fecdf8`（push 済み）。`isLoopbackHost` の bracket 形式（`[::1]:8731`）を剥がして name 比較。実測（canonical server・修正込み exe）: `POST /api/fav`（`{"gallery_id":0}`）で `Host: [::1]:8731` → 400（same-origin 扱い）、`Host: localhost` control → 400、`Host: evil.example` → 403（回帰）。`::1`・`[::1]`・`localhost:` の既存判定意味は変更前と同一。テスト固定 `TestViewerPOSTSecurityBracketIPv6HostIsSameOrigin`。go vet クリーン、go test ok 37.525s、pytest 33 passed（freshness 含む）。機械的確認: コミット存在 ✓／ツリー clean ✓／個別記録 Status 完了 ✓。残件: 記録の行番号 122-135 は現ファイル 133-153（同一関数）。実測用 server は停止して開始時状態（未稼働）へ復元（現況: 8731 未稼働）。`backlog.md` の A-15 行削除は親の Memory Bank 処理として未実施。
- A-10 部分合格（未検証）→ 進行中 maintained、コミットなし。`downloadFilesParallel` に既存・size 一致 skip を追加（`recordedSizes` helper、download JSON に skipped 報告、`--overwrite` は承認範囲外で未追加）。mock 固定テスト `TestDownloadFilesParallelSkipsExistingFileWhenRecordedSizeMatches`（fetch 呼出 0・内容保持）、CLI 実測（scratch root・seed bytes）: 2 回目 `attempted=1 written=0 skipped=1`（image 呼出 0）、回帰（size 不一致・欠損・記録なし）は fetch。go vet ok、go test ok 37.563s、pytest 33 passed（freshness 含む）。未検証: 1 回目の attempted==written が実在 gallery 4301537 で未成立 — live gg.js の switch polarity（cases 内 hash g=2557 → `w1`=200）と `GG.M()`/テスト fixture（case→`w2`）が反転し CLI download が live で全 hash 404。polarity 是正（单独項目）の判断が必要。機械的確認: コミットなし ✓（進行中）、未コミットは本項目分のみ（`app/main.go`、`app/hitomi_test.go`、個別記録、tsv）。gate/quota・A-07b singleflight・C-08 negative cache への影響なし（skip は `getImage` 前、呼出 0 をテストで固定）。

## 第2実行（2026-11-03・推奨案承認後、委譲5件）

- B-09 合格 commit `f8ee53d`（push 済み）。方針確定（修飾キー click は native へ回し `preventDefault` しない、plain click・newTabMode は absolute URL で `window.open`）。実操作（fresh tab・server 提供 UI）: Scenario A plain click `?newtab=1` → tab 1→2、新 tab `http://127.0.0.1:8731/?newtab=1#/g/4301574/page/1`、reader naturalWidth=1713。Scenario B Ctrl+click → `defaultPrevented=false`、tab 2→3。Scenario C（newtab なし回帰）tab 増えず same-tab hash、target 属性なし。テスト `TestEmbeddedUIHasNewTabQueryPolicy` 追加。go test ok 38.424s、pytest 33 passed。Drive: `I:\マイドライブ\kilo\2026-11-03\b09-scenarioA-newtab-mode-search-tab-20261103.png`、`b09-scenarioA-newtab-opened-by-plain-click-20261103.png`、`b09-scenarioB-ctrl-click-newtab-20261103.png`、`b09-scenarioC-same-tab-regression-20261103.png`、`b09-newtab-click-handler-confirmation-20261103.txt`。機械的確認: コミット存在 ✓／tsv 行削除は同一ファイルの A-10 差分と混在するため commit 分離（記録 commit で親が反映）✓／個別記録 Status 完了 ✓。
- gg polarity（A-10 の blocker）: premise 不成立 → 実装せず記録のみ commit `b0a58ad`。fresh live `gg.js`（11:38Z・bucket `1793703601/`・cases 1984）は `var o = 0; case → o = 1`（現行 code と一致・decision line 15 と矛盾）。窓 data（gallery 4301695）: in-case hash `w2`=200/`w1`=404、out-of-case hash `w1`=200/`w2`=404。10:00窓 `var o=1`/case→0、11:00窓 `var o=0`/case→1、12:00窓 `var o=1`/case→0 → 構造は窓ごとに rotation、片側固定は半分の窓で失敗。記録: `docs/ai/decisions/2026-11-03-gg-case-polarity-correction.md`、`docs/ai/known-issues/2026-11-03-gg-polarity-inverted-vs-live-ggjs.md`。
- gg faithful parse 合格 commit `f76dd7a`（code）＋ `63688df`（記録）。`parseGG`/`GG.M()` が初期 `o`・case 代入を保持、`tools/gen_ggset.py` は live 観測 polarity を snapshot へ記録、`app/ggset.go` 再生成。両構造 fixture で `w1`/`w2`・`atn`/`btn` 反転を固定テスト化。現窓（bucket `1793707201/`・cases 2112・`var o=1`/case→0）実測: cases 内 hash `c05d79ed…e37d8c63`(g=966) → app 構築 `w1`=200（300986 bytes）／`w2`=404、cases 外 hash `4a3c2ca2…f40854`(g=1157) → app 構築 `w2`=200（246926 bytes）／`w1`=404。canonical exe 実機（scratch dataDir）CLI download gallery 4301695 `attempted=3 written=3`、viewer `/api/image/<hash>?format=webp` → 200 `x-cache: miss`（300986・246926 bytes）、`no_full` なし。go vet ok、pytest 33 passed。decision 新 entry `2026-11-03-gg-faithful-parse-not-fixed-polarity.md`（line 15 の固定構造主張を supersede、mapping 方向 `m=0→w1/atn`・`m=1→w2/btn` は有効と維持）。機械的確認: コミット存在 ✓／imageProxy 順序・TN 表示源・永続 cache・UI full 判定・B-01/B-02/B-03/B-05/B-09 無変更 ✓。
- A-10 合格（close 済み）commit `5d846b4`（push 済み）。実測（canonical exe・scratch dataDir `docs/ai/tmp/assess-20261103-1220/a10-cli-final/`・gallery 4301695）: run1 `attempted=3 written=3 skipped=0`（sizes 300986/316356/291158）、run2 `attempted=3 written=0 skipped=3`（remote 呼出 0）、回帰（+2 bytes 不一致・`kilo-safe-remove` 欠損）run3 `attempted=3 written=2 skipped=1`。metrics（明示テストモード・gallery 4294614）: run1 download fetch calls 2、run2 delta 0、TN calls 0。テスト修正: live test の呼出計上を shard full-image fetch の kind（`data`）へ（TN kind 参照で 0 と誤読していた）、TN kind 0 assertion 追加。go test ok 37.806s、pytest 33 passed（freshness 含む）。機械的確認: コミット存在 ✓／変更は本項目分のみ ✓／個別記録 Status 完了 ✓（commit 反映は record commit で分離）。窓 rotation では faithful parse が窓ごとに初期 o・case 代入を読むため一致は維持（記録上の注記）。

## 第3実行（2026-11-03・委譲5件・API key 受領後）

- A-07a 合格 commit `5fb758e`＋記録 `31f8342`（push 済み）。`writeCacheFileOnce` 冒頭で format magic 照合（webp=`RIFF`+`WEBP`、avif=`ftyp`+`avif` brand）、不一致は cache しない。実測: 不正 bytes → cache 増えない・attempt 0、valid → cache（attempt 1・rename 1・`x-cache: miss` 意味維持）。canonical exe 実機（scratch dataDir・8731）CLI download gallery 4301695 `attempted=1 written=1`、viewer `200 x-cache: miss 300986 bytes` → req2 `x-cache: cache`、cache ファイル magic 確認。go test ok 37.686s、pytest 33 passed。読了 decision 3件を記録に明記。機械的確認 ✓。記録 drift: 記録のテスト名 `TestViewerImageProxyCachesBytes` は HEAD に存在せず、実体は `TestViewerImageProxyWritesPersistentCacheAndServesCacheHit`。`tools/VERSIONS.md` digest はスコープ外・未更新。
- B-06 合格 commit `4f14d00`（push 済み）。mock fetch で `stats.total>0` + 空 results → `not_evaluated`、`scan_cache` に `pass` を保存しない、findings `api_error:not_evaluated` を固定（テスト追加）。回帰: positives>0 → hit、404 → `unknown_to_vt` + negative cache、TTL 意味不変。pytest 38 passed。code 変更なし（当該分岐は `8288151`＝review 基準 `349778d` の祖先で解消済み・premise stale）。live hash-only 1 回 404 実測（`stats`/`results` の 200 応答実測は未実施＝未検証）。機械的確認 ✓。残件（判断要）: `app/vt.go:150` は key 有無で判定するため、key あり＋空 map の 200 応答では Go 側 `pass` となり Python の `not_evaluated` と分岐（スコープ外）。原記録「VT cache 60 日」は実際 30 日（`scan.py:32`、`main.go:328`）。
- B-07 合格 commit `bfdc83e`（push 済み）。YARA 一時 error で構造 pass の `scan_status` を `scan_error` へ上書きせず `not_evaluated` を維持、YARA 失敗は `scan_source`（`structural+yara_error`）＝retry 用表現、findings 0 維持、export skip 条件（hit/scan_error）不変。実測（scratch root・`yr`=存在しない binary・VT 無効）: scan `{not_evaluated:1, scan_error:0}`、export `{entries:1, skipped:{}}`。go test ok 38.079s、pytest 38 passed。機械的確認 ✓。残件（判断要）: ① Python 参照実装（`src/scan.py` process_file）は YARA error → `scan_error` のまま（Go と対応付が drifted。実行計画の表記は Python）。② `not_evaluated` により YARA error ファイルが VT gate に入り得る（decision 2026-11-02 #3 と整合、VT 呼出増の副作用）。
- B-10 合格 commit `a303a4e`（push 済み）。`vt_lookup` の `fetch=None` path の `urlopen` を try で囲み `HTTPError` を tuple 規約 `(payload, status)` へ変換。mock なし実 HTTP 実測（scratch）: 404 → `unknown_to_vt`（negative cache 行＋finding）、429 → `api_error:rate_limited`、500 → `api_error:not_evaluated`、`main()` 同形 loop で crash なし。live hash-only 1 回 404 exception なし。TTL（実測 30 日/2 時間）意味不変。テスト変更なし。pytest 38 passed。機械的確認 ✓。残件: 非 HTTPError（`URLError`・timeout）は依然 raise し得る（スコープ外）。
- A-02/C-03 合格 commit `42f1765`（push 済み）。`recordDiscovered` を request あたり 1 transaction へ（`recordDiscoveredBatch`、CLI search 1 tx、search handler record phase batch 1 呼び出し、external call は tx 外）。実測: request1 = 1 commit（修正前 25）、empty ids = 0、2回目行数不変（D-03 の unique key・`INSERT OR IGNORE` 意味維持）。`X-Search-Timing` record 935 → 45ms/38ms（canonical server・rebuilt exe・scratch root）。go test ok 39.958s、pytest 38 passed。機械的確認 ✓。報告した解釈: 承認スコープは `app/main.go` だが合格条件成立のため `app/viewer.go` の call site 1 行を変更。`recordDiscovered` 引数 `dbExec` → `execer` interface 群（テスト fake 用最小構成）。
- 委譲上限5件（A-07a・B-06・B-07・B-10・A-02/C-03）で終了。実施順18以降（A-03、A-14/C-05、C-04、C-06/C-07/C-09、A-11、A-04/D-09/D-10、A-06、A-09、A-13、C-10、D-01/D-02/D-08、D-06、D-07、A-12、B-04）は未実施。
- API key 保管: `scratch/virustotal-api.key`（`.gitignore:8` の `scratch/` で非追跡）。アプリは config の `api_key` → env `VIRUSTOTAL_API_KEY` の順で読む。key はコミット対象ファイル・記録・報告へ書かない（grep で確認済み）。`output/`（別端末へのコピー用）へ key を入れない。未追跡 artifact: `app/app.exe`。

## 第4実行（2026-11-03・判断承認後・D-02単独実施）

- D-02 合格（ユーザー直接指示で実施順28のD-02部分のみ先行実施）。単独commit `ef2d038`（push済み）は `.gitignore`、`app/main.go`、`app/main_test.go`、D-02記録、decision、`current-task.tsv`参照行削除。
- 決定: `config.json` を非追跡の正典config pathとし、`config.example.json` はkeyなしで追跡維持。既定は `<data-dir>/config.json`、明示 `--config` 優先、`config.json` 不在時のみ `config.example.json` fallback。`.gitignore` に `config.json` / `secrets/` 追加。keyはcommit・記録・報告へ書かない。
- 検証: `app`で `go vet .` clean・`go test -count=1 -timeout 300s .` ok 41.711s（追加テスト含む）。canonical exe再build。ルートで `python -m pytest tests/ -v` 38 passed 3.64s（freshness含む）。UI未変更のためserver実機確認なし。`config.json`はgit statusで非追跡、`config.example.json`はkeyなしで追跡維持。
- 判断解決: D-02を除く残判断項目はユーザー推奨承認で確定。確定方針は `docs/ai/decisions/2026-11-03-implementation-judgment-approval.md`。D-01のignore追加、D-06 docs修正、A-04/D-09/D-10 hitomi準拠確認、C-10 test gate inject、B-06/B-07 parity/Python側対応は未実施。
- 次の実施順18項目以降はplanの承認範囲と合格条件に従う。最も近い独立項目は実施順18 A-03。

- A-03 合格（単独commit）。`app/viewer.go`のsearch JSON responseをEncoderで実測し、`json_ms` positiveへ。`timing.metrics` duplicate削除、top-level `metrics` 維持。fixed test更新: `json_ms > 0`、`timing.metrics`不存在、top-level metrics存在、header positive。実測: `go vet` clean、`go test` ok 40.058s、canonical exe再build、pytest 38 passed 4.16s（freshness含む）。UI未変更のためserver実機確認なし。commit `eaf1461`（push済み）。

## Memory Bank TSV fragment 対策（2026-11-03）

- 既存 irregular fragment をユーザー指示により `edit` ツールで削除。
- 正典 `task_manager.py` に `--status` / `--next-action` の単一行 validation と data row 3-field validation を追加（正典 commit `645b89b`・push済み）。
- `tests/test_task_manager.py` に multiline next-action、tab status、existing malformed TSV row、`_write_tsv` malformed row 拒否テストを追加。
- 検証: `python -m pytest tests/ -v` 120 passed `6.19s`、`pwsh -NoProfile -File tests\smoke_safe_rmdir.ps1` 0 failure(s)。
- 記録: `docs/ai/decisions/2026-11-03-current-task-tsv-single-line-guard.md`。

## 第5実行（2026-11-03・判断待ち削減方針後・委譲1件）

- C-05/A-14 合格・close commit `52ffde2`（push済み）。機械確認: HEAD `52ffde2`、ツリーは `app/app.exe` と `app/docs/` untrackedのみ、個別Status完了と子報告一致。
- 残件記録: `indexTopCache`/`noFullCache`/`versions` は承認範囲外で unbounded のまま。known-issues記録のみ。本項目はclose済みで、この残件は実施順20以降の項目と依存なし。
- 次の実施順20項目は C-04: `docs/ai/tasks/2026-11-03-c04-language-double-fetch.md`。
- C-04 合格・close commit `4c25b1f`（push済み）。機械確認: HEAD `4c25b1f`、ツリーは `app/app.exe` と `app/docs/` untrackedのみ、個別Status完了と子報告一致。残件として remote実測（nozomi calls 1→1）未確認、multiple positive language path first-winsは未テスト。本項目はmock testで合格条件固定済みで、実施順21以降と依存なし。
- 次の実施順21項目群のうち C-06 を単独項目として委譲: `docs/ai/tasks/2026-11-03-c06-strip-rebuild.md`。
- C-06 合格・close commit `d2c18dc`（push済み）。機械確認: HEAD `d2c18dc`、ツリーは `app/app.exe` と `app/docs/` untrackedのみ、個別Status完了と子報告一致。group window境界跨ぎの rebuildは設計上意図的でmocked DOMで固定済み。Drive screenshot: `I:\マイドライブ\kilo\2026-11-03-c06-strip-rebuild-page-move-highlight.png`。
- 次の実施順21項目群の C-07 を単独項目として委譲: `docs/ai/tasks/2026-11-03-c07-resize-generation-debounce.md`。
- C-07 合格・close commit `a5fc691`（push済み）。機械確認: HEAD `a5fc691`、個別Status完了と子報告一致。ただし実操作後の `git status` に `hitomi.db-shm`, `hitomi.db-wal` の untracked が残った。これは正典 ignore の不足であり、実施順28 D-01の単独先行実施で解消する。C-09へは dependencyなしとして継続できるが、まずはD-01を実施する。
- 実施順28のD-01を status確認上の blocker解消として単独先行委譲: `docs/ai/tasks/2026-11-03-d01-gitignore-missing.md`。
- D-01 合格・close commit `f8f5895`（push済み）。機械確認: HEAD `f8f5895`, `git status --short` clean, 個別Status完了と子報告一致。`.gitignore`追加のみ。生成物自体は commit/削除なし。
- 第5実行の委譲上限5件（C-05、C-04、C-06、C-07、D-01）に到達。以後は new sessionで `docs/ai/tasks/2026-11-03-c09-read-lists-debounce.md`から再開する。

## 第6実行（2026-11-03・C-09単独）

- C-09 合格・close commit `55b848f`（push済み）。機械確認: HEAD `55b848f`、ツリー clean、個別Status完了と子報告一致（mocked fetch harness 16 checks・固定テスト `TestEmbeddedUIDebouncesReadAndListsToSingleCall`・実機 page移動5回→post/list各1回・Drive `I:\マイドライブ\kilo\2026-11-03-c09-page-move-debounce.png`）。観察（判断なし）: showLists debounce化でopen/searchのlist表示が最大250ms遅延（設計通り・display path不変）。次の実行は実施順22（A-11）から1項目ずつ順次。

## 残項目判断解決（2026-11-03・ユーザー推奨承認）

- 実施順22以降の判定推奨をユーザー承認。正典記録は `docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md`。承認は方針確定であり実施開始指示ではない。
- 確定方針: A-11 streaming（pageごと encode+sequential write）、A-09 stream+304/206、A-13 `bytes.Contains`、A-06 ctx-aware select、C-10 test 専用 gate inject（本番1000ms不変）、D-07 exe 不在時 skip（test 内 build 不採用）、D-08 production 未使用削除・テスト参照は移動・`pageSource` 変更なし・`app/ui.html:1458` no-op 削除、B-04 check 側を `acquisitionTitleName` へ、A-12 `scan_cache(sha256,size,mtime)` skip（不一致は再ハッシュ安全側）、D-06 docs 側 faithful parse 基準へ（README 具体行は実物確認後）、A-04/D-09/D-10 hitomi 準拠確認後に hitomi 挙動へ、B-06 残件 Go 側 empty results 基準の小修正、B-07 残件① Python 側を Go 表現へ、manual full-image retry は再現条件確認まで保留、P2 #8 AVIF は PDF embed 対象外と明示（decoder 追加なし）、P2 #9 は用途定義後に列削除か placeholder 明示で閉じる。
- 承認済みの実施順（plan 表の旧順に優先して適用）: ① A-13 → A-06 → C-10 → D-07 → D-06、② A-11 → A-09、③ 判断項目（A-04/D-09/D-10 hitomi 準拠確認、B-06/B-07 parity、P2 #8/#9）は実施開始指示後に順次。

## 第7実行（2026-11-03・承認済み実施順 A-13→A-06→C-10→D-07→D-06・1項目ずつ）

- 委譲前状態: HEAD `67cd3d0`（記録commit・push済み）、tsv clean。

- A-13 合格・close commit `7144dd1`（push済み）。機械確認: コミット存在 ✓／ツリー clean ✓／個別記録 Status 完了・tsv 参照行削除と子報告一致 ✓。子解釈に対する親判断（追加作業なし）: 合格条件の「existing test `ScanPDFKeys`」は Go テスト名として存在せず、既存 PDF key カバー（`TestStatusSemantics`・pytest `test_pdf_viewer_action_hit`）で確認した解釈を是とする。実測漂移修正（pdfKeys 7 key・最大14MB・行175）は記録済み。未確認点（exe 内容からの Go 差分検証不能）は項目性格上合格条件に影響なし。
- A-06 合格・close commit `c02bf83`（push済み）。機械確認: コミット存在 ✓／ツリー clean ✓（未コミットは plan の本 progress 追記のみ）／個別記録 Status 完了・tsv 参照行削除と子報告一致 ✓。軽微不一致（報告のみ・追加作業なし）: 個別記録の Next Action 行は「引き継ぎ完了・実施開始指示待ち」のまま（task_manager end は Status と tsv のみ更新）。記録の行番号 139/182… は旧版参照でシフト（実コード gate 293・backoff 336/355/369/387 と内容一致で実施）。
- C-10 合格・close commit `a58e352`（push済み）。機械確認: コミット存在 ✓／ツリー clean ✓（未コミットは plan の本 progress 追記のみ。子 plan ファイルを commit せず ✓）／個別記録 Status 完了・tsv 参照行削除と子報告一致 ✓（Next Action 行の未更新は A-06 と同一パターン）。証拠: go test 66.946s→5.094s、gate semantics tests 全 PASS・本番既定 1000ms 不変、pytest 40 passed。判断持ち帰りなし。
- D-07 合格・close commit `aac4767`（push済み）。機械確認: コミット存在 ✓／ツリー clean ✓（未コミットは plan の本 progress 追記のみ）／個別記録 Status 完了・tsv 参照行削除と子報告一致 ✓。残件（未検証として報告）: isolated clean clone 上の skip 実発火は未実施（scratch 模拟 `docs/ai/tmp/assess-20261103-1220/` の simulate_skip.py・test_skip_sim.py で代替・1 skipped）。正典 exe SHA256 前後同一（無損）✓。
- D-06 合格・close commit `77181d5`（push済み）。機械確認: コミット存在 ✓／ツリー clean ✓（未コミットは plan の本 progress 追記のみ）／個別記録 Status 完了・tsv 参照行削除と子報告一致 ✓（Next Action 行未更新は A-06/C-10 と同一パターン）。docs のみ（歴史的記録保持＋正典決定へ整合注記、pytest 40 passed 8.86s 無損）。判断持ち帰り（ユーザー判断）: プロジェクト AGENTS.md「Code Scope」の「UI…書き込み口は未実装」が write-protected で変更不可、README・実装と矛盾 → 正典更新か維持かの判定が必要。第7実行の委譲上限5件（A-13・A-06・C-10・D-07・D-06）に到達。次の実行（新セッション）は ② A-11 → A-09、その後 ③ 判断項目（A-04/D-09/D-10 hitomi 準拠確認、B-06/B-07 parity、B-04・A-12、P2 #8/#9）。

## Verification

- plan タスク自体は docs/ai 記録のみ。code change なし。
- 項目ごとのコマンド・結果・経過時間は個別タスクの Verification と本実施 progress に記録。
- 判断待ちを減らす今後の運用方針を `docs/ai/decisions/2026-11-03-judgment-waiting-reduction-policy.md` に記録した。実装は default領域の推奨案と実測で単独 commit/pushへ進め、UX意味変更・外部API policy・不可逆 migration・正典rule変更・hitomi convention未確認領域だけ判断待ちへ回す。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
