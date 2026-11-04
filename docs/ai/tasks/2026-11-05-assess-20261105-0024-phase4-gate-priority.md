# Remote Task Phase 4 — サーバー側ゲート優先度化（要否測定つき）（調査・評価）

**Status**: 完了
**Date**: 2026-11-05
**Next Action**: 子の調査委譲（フォアグラウンド）

## 意図 / 背景

- 項目ID: `2026-11-05-hitomi-hub-search-phase4-gate-priority`（実行固有ID `assess-20261105-0024` の項目3/3）
- リモートタスク原文（正典）: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase4-gate-priority.md`
- 個別タスク記録（このファイル）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase4-gate-priority.md`。評価記録一覧: `docs/ai/tasks/2026-11-05-assess-20261105-0024-items.md`

## 調査結果 / 変更内容

実行固有ID: `assess-20261105-0024` / 項目ID: `2026-11-05-hitomi-hub-search-phase4-gate-priority`（項目3/3）。有効性: **有効（条件付き）** — 現行単一タイムスタンプゲートの FIFO 挙動は実測で確定。Phase 2 の設計（リクエスト単位で譲る＝バックグラウンド in-flight ≤1）なら全シナリオのゲート起因遅延が 1 枠分以内（→ 実施不要）。ただし Phase 2 記録の推奨パラメータ（card 並行度 2＝in-flight 2）では、フォアグラウンド待機が 2 枠分（データ約 1.9s・画像約 0.47s）に達することを再現性ありで実測確定 → その設計で実施するなら Phase 4 が必要。コードは無変更（ソース変更不可の委譲制約。実装案は以下）。

### 棚卸し

- HEAD: `2871fc8`。基準 `7160dae` との差分は docs コミット 2 件のみ（`app/` 実コードは基準と同一。実コード優先の照合: 差異なし）
- `git status`: 既存の未コミット変更は親の記録ファイル 4 件（items・phase2/3/4）。本タスク成果物に不含。本調査によるコード変更なし（Verification で git status 確認済み）
- 個別タスク ID: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase4-gate-priority.md`（正典 `task_manager.py add` で親登録済み・`current-task.tsv` 参照行確認。子は本ファイルのみ更新）
- Phase 2 実施状況: **未実施**（`app/search.html` に scheduler なし — 次ページは「次」クリック後にのみ取得。Phase 2 評価記録 `2026-11-05-assess-20261105-0024-phase2-next-page-prefetch.md` と一致。実装タスク未登録）。よって測定は**模擬負荷**（マーカー `priority=background` 付き連続リクエスト相当）で行った。限界: 実 UI の先読み機構ではなく、ゲート FIFO の競合算術のみを対象とする
- 正典 server 8731（PID 19316）使用。ただし下の「異常観察」参照。追加の照合用に正典 exe（disk ビルド）fresh インスタンスを 8732（scratch root、`C:\Users\fujim\AppData\Local\Temp\kilo\phase4_fresh`）で起動して測定・画面確認を実施

### 現行実装（実コード確認）

- `Client` は系統ごとに単一タイムスタンプ（`c.last` data 1000ms / `c.imageLast` image 250ms、`app/hitomi.go:136-180`・`defaultHitomiConfig` `app/hitomi.go:202`）
- `gateForInterval`（`app/hitomi.go:342`）は `now < last+wait` のとき `*last = last.Add(wait)` して sleep（sleepCtx・ctx cancel 即 return、A-06 固定済み）。→ **先着順 FIFO であり、予約が将来へ連鎖する**: 同時 in-flight k 件のとき後続リクエストのゲート待機は最大 (k+1) 枠分の窓に収まる（in-flight 1 なら ≤1 枠）。「1 枠想定」は in-flight ≤1 でのみ成立
- metrics（kind 別 gate_wait/network/decode/retry_sleep、`classifyFetchURL` の data call が kind "data" を返すのは URL 分類であり lane ではない — full shard fetch は image lane だが kind data。テスト `metrics_test.go`）。testGateIntervalMS inject（C-10）維持
- マーカー伝達経路の現状 gap: `SearchNozomiParts`（`app/searchhtml.go:131`）・`Client.Gallery`（`app/hitomi.go:615`）は内部で `context.Background()` を使う（ハンドラの ctx を伝えない）。image 系は `r.Context()` を伝達済み（`viewer.go:734`・`770`）。context value 方式を data 系統で使うにはこの 2 箇所の呼出変更が必要

### 手順 1 測定（実測・外部呼出し・ゲート順守・必要最小限）

反復: 各シナリオ baseline（idle）3 件 + 負荷あり 3 反復（A）/ 2+1（B）。遅延 = 応答 wall − baseline wall（wall には自网络の揺らぎを含む。ゲート起因分は FIFO 算術で分離）。全数 scratch `docs/ai/tmp/phase4_measure.py`（8731）・`phase4_data_fresh.py`・`phase4_repro_fresh.py`（8732）で取得。

- **A（data・フォア=新規検索・バック=card 負荷）**: baseline median 212ms（198/219/212）。in-flight ≤1: 遅延 915・907・1161ms（ゲート起因分 ≤960ms=1 枠内、1161 の超過分は network 揺らぎ）。in-flight 2: 1934・1970ms（FIFO 算術 1940ms と一致）→ **1 枠超**。fresh 8732 照合: seq fg gate wait 960ms（wall 1330）、par2 fg gate wait 1940ms（search=2932）
- **B（data・フォア=未キャッシュ gallery・バック=gallery 負荷）**: baseline median 203ms（184/223）。in-flight ≤1: 974・1006ms（ゲート起因 ≤960ms=1 枠内）。in-flight 2: 1720ms（算術 1858〜1940ms）→ **1 枠超**
- **C（image・フォア=フル画像・バック=TN 負荷）**: baseline TN 255ms・full 771/571ms（8731）、8732: TN 616・full 960。in-flight ≤1: 遅延 288・289（8731）/ 219（8732 gate 分）→ **1 枠（250ms）内**。in-flight 2: 571ms（8731 推定）/ gate 分 453ms（8732・算術 469ms と一致）→ **1 枠超**

判定: 前提（in-flight ≤1＝Phase 2 の「リクエスト単位で譲る」）は実測で**成立**（全シナリオのゲート起因遅延 ≤1 枠）。Phase 2 推奨の並行度 2 では**成立しない**（data 約 1.9 枠・image 約 1.8 枠）。判定は実測で閉じた（判断保留なし）。

### UI 確認（シナリオ C・正典 exe fresh インスタンス 8732・fresh tab・strict 条件・resize 不使用）

- server-direct baseline（UI 計測前）: u1 card 141ms（cache）/ TN 452ms 16276B（tn）/ full 856ms 108046B（miss）→ TN と full は別バイト（同一でない）
- baseline（cold 表示）: gallery 4304342 開 → `#reader img` `naturalWidth=1455`（objectURL blob・実 full ピクセル）1567ms
- 負荷（TN 先読み相当 2 件 sequential・マーカー付き）→ cold gallery 4304091 開 → 表示 794ms、fresh blob `naturalWidth=2880`（full 179894B、TN 30034/30432B と異なる）。TN は表示源でない（x-cache 判定・new-blob guard で確認）。ゲート干渉 ≤1 枠（算術 219ms）と整合
- 保存先（タスク別 Google Drive フォルダ・同日文書の正典として `2026-11-02-google-drive-task-specific-screenshot-folder.md` を採用。date folder 条と矛盾 → 日付同日だが future tasks 適用範囲明示のタスク別側を採用。`2026-11-02-google-drive-confirmation-storage-procedure.md` の strict 条件は準拠）: `I:\マイドライブ\kilo\2026-11-05-assess-20261105-0024-phase4-gate-priority\`（`phase4-ui-baseline-reader-20261105.png`・`phase4-ui-loadrun-reader-20261105.png`・`phase4-ui-reader-confirmation-20261105.txt`）

### 異常観察（申し送り・known-issues 候補）

- 正典 server 8731（PID 19316・起動 2026-11-04 23:55 JST）で、C-par2 の `/api/image/<hash>` 応答が `x-cache: tn-cache`（TN バイト 26725）であった。現行 HEAD の `imageProxy` はこの x-cache を返せず（local/cache/miss/no_full のみ・`TestImageProxyCacheStateIsNotTN` で固定）、かつ同一 hash の TN が再照会で tn-cache hit せず（no_tn 502・cache ファイルが正典 output ルートに不在）。disk ビルドの fresh インスタンス（8732）では正しい挙動（imageProxy は miss/no_full・TN は thumbs に永続化）を再現 → **起動中インスタンスの in-memory ビルドは正典（HEAD）と保証できない**（実装段階前に canonical rebuild + 正典 server 再起動後、C-par2 を再測定すべき）。ゲート lane 算術の結論は数値が HEAD 算術と一致するため影響なし
- 異常応答そのものは「TN を full 表示源として扱わない」ガード違反の内容である（表示 pass として無効）。Lane timing 証拠（571ms）は有効のまま

### 実装段階の推奨案（本調査では実施しない・decision 記録は実装段階）

- レーン: 系統ごとに `lastFg`/`lastBg` + pending フォア待機フラグ。フォア = 現行 FIFO（先着順・1 枠間隔維持）。バック = 「フォア待機なし && lastFg + 猶予 300ms <= now && lastBg + 間隔 <= now」でのみ予約（フォア不在時=現行と同等の間隔で進行＝飢餓防止）
- マーカー伝達: query param `priority=background`（P2 実測で no-op・cache key 不変確認済み）。経路は handler → `Client` の明示引数（`background bool`）を推奨。理由: data 系統の `SearchNozomiParts`/`Gallery` が `context.Background()` を内部使用（ctx value は届かない）。context 統一は 2 箇所の呼出変更が必要で、既存テスト（`c.last` 直接参照）の再アンカーを要る
- metrics: kind 区分の追加は後方互換（例: 既存 kind を維持し `lane` フィールド追加、または `gallery.background` のような新 kind）。gate_wait/network の意味・形式を変えない
- 固定テスト: フォア待機中の間バックグラウンド枠を発行しない／フォア不在時バックは現行同等間隔（飢餓なし）／context cancel でゲート待ちから即抜ける（A-06 既存）＋既存 gate semantics（`TestRateGatePerRequest`・`TestImageGateUsesLowerImageIntervalSeparately`・C-10 inject）維持
- 触らない範囲（フル画像ガード）は本調査でも非接触。`imageProxy` 順序・`EnsureGG(force)`・x-cache 判定は再測定のみの確認

優先順位付けに効く要素: 重要度=中（応答性・データ整合なし）。緊急度=低〜中（Phase 4 は任意。判定は Phase 2 実装パラメータの選択に依存）。リスク=低〜中（ゲート内部変更。metrics・ctx sleep・inject・ガードテストの非接触維持が条件）。依存=Phase 2 実装（並行度を 1 に固定すれば Phase 4 不要＝最も安価。並行度 2 を採るなら Phase 4 必要）。推定規模=実装時 Client 内 100〜150 行 + 固定テスト 3〜4 件 + canonical build + 再測定。

### 親控（委譲結果確認）

- 要約と記録有無確認済み: 有効性「有効（条件付き）」・実測で閉じ判断保留なし。前提（in-flight ≤1＝1枠）は成立、P2推奨並行度2では再現性ありで1枠超。記録あり（本ファイル）。

## Verification

- `go vet .`（app）: pass（output なし）。`go test -count=1 -timeout 300s .`: **pass（17.919s）**
- フル画像ガード・ゲート semantics テストの個別 pass（`-run` 個別実行 0.289s）: `TestStorageURLUsesFormatNotNameExt` PASS・`TestCanonicalGGRuleComposition` PASS・`TestThumbnailURLMatchesVerifiedTNPath` PASS・`TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing` PASS・`TestImageProxyCacheStateIsNotTN` PASS・`TestViewerThumbProxyTNOnlyDoesNotServeFullCache` PASS・`TestRateGatePerRequest` PASS・`TestRateGateReturnsImmediatelyOnContextCancellation` PASS・`TestRetryBackoffReturnsImmediatelyOnContextCancellation` PASS・`TestImageGateUsesLowerImageIntervalSeparately` PASS
- コード無変更の確認: `git status` — `app/`・`tests/` 変更なし。変更は本タスク記録のみ（既存の親記録ファイルは成果物に不含）
- viewer 実機確認（正典 exe disk ビルド fresh インスタンス 8732・scratch root）: `/api/image/<hash>?format=webp` → 200・`x-cache: miss`・191096B / 328246B / 282642B / 179894B（no_full でない・TN バイト 21300〜32463B と同一でない）。TN 永続化: 同一 URL 再照会 `tn-cache` hit（7ms）
- 実測値（8731 主・8732 照合）: A baseline median 212ms・seq 915/907/1161ms・par2 1934/1970ms（8732: gate 分 960/1940ms）。B baseline median 203ms・seq 974/1006ms・par2 1720ms（算術 1858〜1940ms）。C TN baseline median 255ms・full 571/771ms（8732 TN 616・full 960ms）・seq gate 分 ≤230ms（219ms）・par2 571ms（gate 分 469〜453ms）。UI: baseline 表示 1567ms（naturalWidth 1455）、負荷時表示 794ms（naturalWidth 2880・full 179894B ≠ TN 30034/30432B）
- 外部呼出し数（必要最小限）: 正典 server 8731 metrics 由来 = nozomi 18・gallery 24・image 10・data(shard) 4 → 56。8732 fresh + UI ≈ 34（search 4・card 6・TN 6・full 3・UI data 9・UI image 8）。正典ゲート間隔（1000/250ms）順守、リトライ 429/5xx 発生なし（1 件の TN 502 は異常観察の transient）
- 正典文書矛盾処理: Google Drive 保存は同日 2 文書（date folder 条・タスク別 folder 条）が矛盾 → 適用範囲を future tasks へ明示するタスク別 folder 条を採用（Phase 2 評価の precedent と一致）。作業前チェック decision 3 点（`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`）+ canonical build ポインタ（`2026-11-04-search-viewer-separation-approach-approved.md`）読了済み（viewer 接触項目のため）
- 未実施点（委譲制約による）: 手順 2（実装）は実施せず推奨案記録のみ。異常観察の known-issues ファイル作成は親の判断へ申し送り（子の変更可能ファイルは本記録のみ）。8731（起動中 server）上での UI 確認は instnce 不一致のため 8732 で実施（正典 exe・scratch root 運用は decision 手順の copied/runtime root 形に沿う）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
