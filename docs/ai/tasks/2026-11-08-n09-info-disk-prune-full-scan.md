# N-09: info disk cache の prune が上限到達時に全 JSON を走査

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 完了（assess-20261109-1320 実装）: 案1 hysteresis（cap 5000・target 4500・scan 1/501）実装・1 assertion 更新・正典検証合格・harness 実測合格

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: info の disk cache は上限に達すると新規書き込みのたびに全 JSON を走査する（`pruneInfoDiskLocked`）。上限まで刈ると次の 1 件で再び超過し、最大 5000 件を読んで parse する。`infoDiskMu` を保持したままなので `readInfoDisk` も止まる。10% 程度多めに刈るか、parse せず mtime で刈る案。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-09 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（正典照合）

- `app/galleryinfocache.go`（Phase 3・decision `2026-11-05-phase3-gallery-info-persistent-cache-file-store.md`）:
  正典は「上限（件数 5000・容量 256MB の先に到達した方）超過時のみ scan・取得時刻（`fetched_at`）古い順に evict・JSON field 正・parse 不可は mtime fallback・実行は書き込み時の間引きのみ・disk 層は access order を持たない」。
- `pruneInfoDiskLocked`（`app/galleryinfocache.go:134-196`）は scan 時に全 `.json` を `os.ReadFile` + `json.Unmarshal` して順を決める。`infoDiskMu` を保持したまま（`writeInfoDisk` 内・`galleryinfocache.go:76`）。`readInfoDisk`（`galleryinfocache.go:31-46`）は同一 mutex を取るため、scan 中(reader)は待たされる。
- steady state（上限ちょうどまで刈る）では、新 entry 書き込み → count=5001 → scan → ちょうど 5000 へ → 次 1 件で再び超過。よって「上限到達後、新規書き込みごとに 5001 件 read+parse」が実質恒常化。ただし 既存 ID への書き込み（TTL 再取得・same ID 更新）は count を増やさないため entries cap scan のトリガにならない（bytes cap 超過のみ）。項目原文の「新規書き込みのたび」は new entry 書き込みの話として有効。

### 有効性: 有効（実測で確定）

スクラッチ実測（`docs/ai/tmp/assess-20261108-1332/r09/`・overlay 注入・t.TempDir・実 cache・実 DB 非接触・外部 API なし・entry 約 760B・12 files・Windows NTFS・Defender 実機）:

- baseline 1 件 write（prune なし）: **351.8 µs**
- 正典（上限まで刈る）: steady state で毎書き込み **5001 件 read+parse**（「上限まで刈る→次 1 件で再超過→scan」3 回反復で確認・evict 連鎖 oldest 順一致）。scan 壁時間: cold（初回・AV 含む）**30.1 s** / warm **約 0.7 s**（S1 copy scan 平均 10.35 s、S1 copy reader warm scan 679.9 ms）。実 `writeInfoDisk`（正典関数）per-write 平均 **7.78 s**・worst **21.98 s**（cold 効果）。
- `readInfoDisk`（reader）待ち: warm **679.9 ms**、cold worst **30.1 s**（prune 保持のほぼ全長を待つ）。→ 項目原文「`readInfoDisk` も止まる」は実測で有効。

### 案比較（スクラッチ側実装 simulated・1 書き込みあたり)

| 案 | scan 頻度（600 書き込み窓） | parse/件 | 壁時間/件 | reader 待ち worst |
|---|---|---|---|---|
| 正典（上限まで刈る） | 毎書き込み（1/1） | 5001 | warm 0.7s・cold 30.1s | 679.9 ms warm・30.1 s cold |
| 案1（10% 多め target=4500・正典意味=超過時 scan） | 2 回（1/501） | 平均 16.7（scan 時 5001） | 56.8 ms（平均・cold scan 含む） | 30.1 s（scan 時 cold） |
| 案2（mtime・parse なし・上限まで刈る） | 毎書き込み | 0 | **7.15 ms** | **12.0 ms** |
| 併用（mtime + 10% watermark） | 2 回（1/501） | 0 | **798 µs** | **102.7 ms**（cold scan） |

### 正典衝突・意味の対比（承認案件の分離）

- 「access order」正典（C-05 参照・Glob で実在確認: `docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`・memory 4 cache の LRU）は disk 層に適用なし。Phase 3 decision と `app/viewer.go:140` 注記が「disk 層は access order を持たない」と明示。よって項目原文の「案2 は LRU 意味を失う」は不正確 — disk 層は正典で LRU ではなく**取得時刻順**。案2 が失うのは「JSON field 正（fetched_at 順）」の order 源と「scan 時の壊れたファイル即時除去」（read 時は `readInfoDisk` が壊れを検出・除去するため正しさは保たれ、即時性が「読み取り時除去」へ移る）。mtime ≈ 書き込み時刻（atomic temp+rename・`fetched_at` と同時設定）なので app の書き込みのみでは順序差はほぼ出ず、差分は外部 touch/copy/clock の edge case。
- R-15（`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`・確定 2026-11-06 10:52）: disk 削除順=**作成順**・last access 記録を作らない・件数上限・filesystem scan で数えない（DB 1 表 `cache_entries`）。この決定の cap 範囲は `cache/images`・`thumbs`（8000 件）であり galleryinfo 直接対象ではない。mtime 案は R-15 の「作成順」流儀と整合。衝突は Phase 3 decision の「JSON field 正」点のみ。日付が新しい R-15 を方向として優先するが、Phase 3 decision の order 源変更（mtime 正）は**decision 変更＝承認案件**。R-15 の「DB 1 表・scan で数えない」を galleryinfo へ拡張する案は実装規模が大きい（file 方式 decision の supersede）。
- 意味対比（現状→変更後・ユーザーに見える）:
  - 案1: 現状=上限到達時ちょうど 5000 まで即刈り→毎 new write で scan。変更後=4500 まで刈り→scan 1/501 回、直後 500 件多く消える（刈る「量」が変わる。刈る順=fetched_at 古い順は不変→「最近見た gallery の info が消える順」は不変）。**「上限到達時の即刈り」の意味変更→既存テスト固定と衝突（承認案件）**。
  - 案2: 現状=order 源は JSON `fetched_at`（mtime fallback）・scan 時に壊れ即時除去。変更後=order 源 mtime のみ・壊れは read 時除去・scan 頻度は毎書き込み（1/1）のまま。意味変更（decision 衝突）→**承認案件**。
- 既存テスト照合（変更していない）: `TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap`（`app/galleryinfocache_test.go:213-266`）が固定 = evict 順（fetched_at 古い順）・「count must stay at cap after evict」（= 上限まで刈る意味）・bounded。案1 はこの「count == cap」assertion と衝突（テスト更新が必要）。案2 は order 源（JSON field 正）と壊れ scan 時除去の固定と衝突。prune の「毎書き込み scan」「stale-only」はテストで固定されていない（stale-only bounded は noFullCache の話）。

### 推奨

- 単独実装推奨は**案1（10% 多めに刈る）**: order 源正典（JSON field 正）を変えず、scan 頻度を 1/501 へ減らす。ただし「上限まで刈る」テスト固定の意味変更（テスト更新）を要る → 承認案件。
-  fastest は**併用案（mtime + watermark）**（798 µs/件・reader 102.7 ms worst）。ただし mtime を order 源とする側が Phase 3 decision 変更・壊れ除去の即時性移動を含むため、承認が取れた場合のみ推奨。mtime 単案（案2 だけ）は scan 頻度を減らさず効果 11.4 倍にとどまり、decision 変更単独で要る。実装規模: `pruneInfoDiskLocked` 内 15〜40 行（トリガ条件・target 分離・mtime 分岐）。テスト更新は案1 で 1 assertion、併用案採用時は order 源・壊れ除去の 1〜2 assertion。
- UI 項目でないため画面確認は不要（保存なし）。

## Verification

- `go vet .`（app）: clean（0.9 s）
- `go test -count=1 -timeout 300s .`（app・既存テスト変更なし): ok 24.953 s（経過 27.6 s）
- `python -m pytest tests/ -v`: 59 passed / 1 failed（既知 failure `test_canonical_build_freshness_rule_exists`・本項目非接触として記録。経過 10.30 s）
- 実測: `go test -overlay "S:/Documents/Kilo_Commons/hitomi-hub/docs/ai/tmp/assess-20261108-1332/r09/overlay.json" -run TestN09 -count=1 -v`（テストファイルは `docs/ai/tmp/assess-20261108-1332/r09/n09_measure_test.go`・正典 source 非変更・overlay 注入）。第 1 回実行 478.1 s（数値は上表の正典意味修正済み実行から: 修正 S2/S4 再実行 45.6 s。第 1 回 S2 は scan トリガ意味誤り（watermark をトリガと同一視）で supersede）。
- 画面確認: 不要記録（UI 項目でない）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

# 2026-11-09 再調査（assess-20261109-1320）

## 現在の実装（現行コード照合）

- `pruneInfoDiskLocked`（`app/galleryinfocache.go:134-196`）は前批（assess-20261108-1332）から無変更。上限超過時のみ scan・scan 内で全 `.json` を `os.ReadFile` + `json.Unmarshal`（壊れは除去）・`fetched_at`（JSON field 正・parse 不可は mtime fallback）古い順で上限まで刈る構造そのまま。`infoDiskMu` 保持中（`writeInfoDisk` 内・`galleryinfocache.go:76`）で `readInfoDisk`（`galleryinfocache.go:31-46`）を待つ。
- `app/galleryinfocache.go` を触った commit は `9a05a6e`（Phase 3 実装）のみ。直近 commit `7cca33a`（cache_dir 形統一）は置き場・path 解決・コメントのみで scan 構造非接触。decision `2026-11-09-info-cache-shape-unified-with-cache-dir`（2026-11-09）の「影響」に「件数表示・間引き・手动 clear は同じ `infoCacheRoot` を見る（code path 不変）」と明記 — 現行でもフルスキャン構造を再確認。
- 置き場は現行 `<cache_dir>/galleryinfo/<id>.json`（既定 `cache/images/galleryinfo`）。scan 構造の判断に影響なし。

## 前批実測数値と現行コードの整合（再実測で閉じた）

前批 scratch（`docs/ai/tmp/assess-20261108-1332/r09/`・overlay 注入のみ・正典 source 非変更・t.TempDir・実 cache・実 DB 非接触・外部 API なし）を現行コードで再実行（経過 99.4 s・Defender cold 含む）。

| 指標 | 前批（2026-11-08） | 再実測（2026-11-09） |
|---|---|---|
| baseline 1 件 write（prune なし） | 351.8 µs | 362.6 µs |
| 正典 steady state | 毎書き込み 5001 件 read+parse（evict 連鎖一致）・scan warm 約 0.7 s・cold worst 30.1 s・実 writeInfoDisk per 7.78 s worst 21.98 s | 同一（5001 件・evict 連鎖一致）・scan warm 651.1 ms・cold worst 26.2 s・実 writeInfoDisk per 6.88 s worst 19.3 s |
| 案1（10% watermark・target=4500） | 600 書き込みで scan 2 回・parse/書き込み平均 16.7・壁時間/件 56.8 ms・reader worst 30.1 s（cold scan 時） | scan 2 回・parse/書き込み平均 16.7（一致）・per 49.5 ms・reader worst 26.2 s |
| 案2（mtime・parse なし） | parse 0・7.15 ms/件・reader worst 12.0 ms | parse 0・7.13 ms/件・reader worst 14.8 ms |
| 併用案（mtime + 10% watermark） | 798 µs/件・reader worst 102.7 ms | 707 µs/件・reader worst 89.0 ms |

→ 前批数値と現行コードの矛盾なし。scan 頻度の構造（正典 1/1・案1と併用案 1/501）は一致。cold 側数値の差は cold/AV 効果の変動範囲（機構は同一）。

## 正典「filesystem scan で数えない」（R-15）と現行 disk prune の整合

- R-15 の「件数・作成順を DB の 1 表 `cache_entries` で持つ・filesystem scan で数えない」は full/thumbs 層では**実装済み**（現行コード): `schema.sql:90`・`ensureCacheEntriesTable`（`app/main.go:562`）・`recordCacheEntry`/`pruneCacheEntries`（`app/viewer.go:1104-1190`・件数判定は DB 行 `COUNT(1)` のみ・`created_at ASC`）。kind は `full`/`thumbs` のみ（`app/cacheclear.go:160`）。cap は 4000 件/kind（decision `2026-11-09-disk-cache-cap-count-4000`・`app/main.go:192`・`app/main_test.go:1666` は 4000 に re-anchor 済み）。
- 状態記述の矛盾を記録: decision `2026-11-09-disk-cache-cap-count-4000` の Status 行は「実装未着手」だが、現行コードは実装済み（コードを証拠として採用。value 決定 4000 はコードと一致し矛盾はない。Status 行のみ stale）。
- galleryinfo 層は `cache_entries` に kind エントリを持たず、現行でも `syncInfoDiskLocked`/`pruneInfoDiskLocked` の filesystem scan で数えている。R-15 の cap 範囲は `cache/images`・`thumbs`（8000→4000）で galleryinfo は直接対象外。Phase 3 decision（`2026-11-05-phase3-gallery-info-persistent-cache-file-store.md`）は DB 新テーブル案を明示的に不採用（file 方式・「filesystem scan は超過時のみ」）。よって整合は「R-15 の範囲外・Phase 3 decision 有効」で成立。R-15 方向（DB 表で数える）を galleryinfo へ拡張は Phase 3 decision の supersede であり**承認案件のまま**。
- R-15「作成順」流儀と mtime 案の整合は前批どおり維持（mtime ≈ 書き込み時刻＝作成順。実装済みの full/thumbs 層は `created_at ASC` で同流儀）。Phase 3 decision の「JSON field 正（fetched_at 順）」を mtime 正へ変える点は decision 変更＝承認案件のまま。disk 層 access order 正典なし（`app/viewer.go:150` 注記・Phase 3 decision）も現行で確認。

## テスト衝突・承認案件の分離（変更なしを再確認）

- `TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap`（`app/galleryinfocache_test.go:213-266`）無変更（テストファイルを触った commit も `9a05a6e` のみ）。「count must stay at cap after evict」（`galleryinfocache_test.go:246-248`）・evict 順（fetched_at 古い順）・bounded・refetch 後 bounded が固定。
- 案1（ヒステリシス）は「count must stay at cap」assertion と衝突（テスト更新 1 assertion）→ 承認案件として分離維持。
- 案2（mtime 正）は order 源（JSON field 正）と壊れ文件的 scan 時即時除去の固定と衝突（1〜2 assertion）→ 承認案件として分離維持。

## 有効性: 有効（再実測で確定）

- 機構（上限到達後、新規 entry 書き込みごとに 5001 件 read+parse・`infoDiskMu` 保持で reader 待ち）は現行コードで有効。実測で閉じた。
- 実データ側の现实状: runtime root `cache\images\galleryinfo` は 429 file（cap 5000・実測 2026-11-09）→ 上限未到達で現状トリガは発火していない。発火は 4571 件追加書き込み後（steady state 進入後）。「恒常化する」は上限到達後の steady state の話として有効（優先順位付け: 緊急度は低い・到達前に修正すれば実データ影响ゼロ）。

## 推奨（前批から変更なし）

- 単独実装推奨: **案1（10% 多めに刈る hysteresis）** — order 源正典（JSON field 正）不変・scan 頻度 1/501・テスト 1 assertion 更新（承認案件）。
- fastest: **併用案（mtime + 10% watermark）**（707 µs/件・reader worst 89 ms・再実測で確認）。採用は order 源変更の承認が取れた場合のみ。
- 実装規模: `pruneInfoDiskLocked` 内 15〜40 行（トリガ条件・target 分離・mtime 分岐）+ テスト 1〜2 assertion。

## Verification

- `go vet .`（app）: clean（0.8 s）
- `go test -count=1 -timeout 300s .`（app・既存テスト変更なし): ok 35.4 s（経過 37.4 s）
- 再実測: `go test -overlay "S:/Documents/Kilo_Commons/hitomi-hub/docs/ai/tmp/assess-20261108-1332/r09/overlay.json" -run TestN09 -count=1 -v -timeout 600s`（scratch 非変更・正典 source 非変更・t.TempDir・実 cache・実 DB 非接触・外部 API なし・経過 99.4 s）。出力: `docs/ai/tmp/assess-20261109-1320/r11_remeasure.txt`
- 実 data file 数確認（read-only）: `output\hitomi-hub\cache\images\galleryinfo` = 429 file
- 画面確認: 不要記録（UI 項目でない）

# 2026-11-09 実装（assess-20261109-1320・子委譲）

## 採用した判断と根拠（段階1）

- **案1（10% hysteresis）を実施**（ユーザー同意済み 2026-11-09）。承認範囲は `pruneInfoDiskLocked` 内のみ
  （追加 8 行 + evict break 条件 1 行）。正典意味は維持: scan トリガは上限超過時のみ・order 源（JSON
  field `fetched_at` 正・parse 不可は mtime fallback）・evict 順（`fetched_at` 古い順）・scan 時の壊れ
  除去・`infoDiskMu` 保持。案2（mtime 正）は decision 変更（承認案件）のまま実施せず。DB 表への拡張も
  実施せず（Phase 3 decision の file 方式有効）。
- target は `cap - cap/10`（切捨て）で分離。正典 cap 5000 → target 4500（推奨どおり・scan 頻度 1/501）。
- 端数の扱い（記録): `cap < 10` では 10% が 0 件となり target == cap（正典意味のまま）。テストの cap 3 は
  この範囲なので、承認済みの 1 assertion 更新は「hysteresis target 照合」の意味になっても値は 3 のまま
  （evict 集合・bounded assertion は無変更で通過）。このため hysteresis の効果（1/501）は正典 cap での
  scratch harness 実測で証明した。テストの cap を 10 以上へ上げる案は複数 assertion 変更（承認範囲超）と
  なるため採用しなかった。

## 変更内容

- `app/galleryinfocache.go`（`pruneInfoDiskLocked`・134-196）: `targetEntries` / `targetBytes`（上限の 90%）
  を分離し、evict の刈り先を target へ変更（トリガは上限のまま）。
- `app/galleryinfocache_test.go`（`TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap`）: 承認済みの
  1 assertion（「count must stay at cap after evict」）を hysteresis target 照合へ更新。他の assertion・
  他のテストは非接触。

## 実測（scratch harness・正典流儀・外部 API なし・t.TempDir・実 cache 実データ・実 DB 非接触）

scratch: `docs/ai/tmp/assess-20261109-1320/n09_hysteresis_test.go`（overlay 注入）。変更後の実装を正典
`writeInfoDisk`（実 `pruneInfoDiskLocked`）経由で呼ぶ。scan 発火は「書き込み後 count が target へ戻る」
ことで決定論的に検出（非 scan 書き込みは count +1）。

- **H1（変更後実装・cap 5000・target 4500・fill 4500 → 1503 書き込み窓）**: scan **3 回 = 1/501**、発火は
  501・1002・1503 書き込み（決定論）。per-write avg **26.77 ms**・scan 時 wall worst **30.12 s**・scan 時
  parse 5001 件。evict 集合は毎回「fetched_at 古い順の 501 件」で実ファイル照合合格（scan1 oldest=1・
  scan2 oldest=502・scan3 oldest=1003）。count は常に cap 以下（bounded）で target 4500 に収まる。
- **H2（control・変更前正典意味 target == cap・同一規模 20 書き込み）**: scan **20 回 = 1/1**・per-write
  avg **2.174 s**（scan worst 30.26 s・parse/書き込み 5001）。→ scan 頻度 1/1 → 1/501、平均負荷
  2.174 s/件 → 26.77 ms/件（約 **81 倍**）。
- **H3（reader 待ち・正典 `readInfoDisk`・mutex 保持中）**: scan なし書き込みは **0 s**（write 1.998 ms のみ）、
  scan 発火時は保持のほぼ全長を待つ（wait **30.64 s** = hold 30.64 s・parse 5001 件・evict 501 件）。
  `infoDiskMu` 保持意味はコード変更なし（正典どおり）。

## 修正・再検証の試行

- 実装 1 回で合格条件を満たした（試行 1 回）。
- harness 側修正 1 回（実装ではなく計測スクリプトのバグ）: H3 の scan なし書き込みで `hFill` が 1 件を
  計上せず count 5002（期待 5001）→ viewer を分離して修正、H3 再実行で合格。

## Verification

- `go vet .`（app）: clean
- `go test -count=1 -timeout 300s .`（app): ok **28.005 s**（更新 1 assertion 通過。単独確認
  `-run TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap -v`: PASS 0.13 s・経過 1.284 s）
- `python -m pytest tests/ -v`: **64 passed**（27.48 s）。UI 文件非変更のため正典 build rebuild 該当なし
- harness 実測: `go test -overlay "docs/ai/tmp/assess-20261109-1320/overlay.json" -run TestN09H -count=1 -v -timeout 600s`
  （経過 95.8 s・H3 再実行 37.4 s）→ 出力 `docs/ai/tmp/assess-20261109-1320/h1_h3_harness.txt`・
  `h3_repair1.txt`。実 cache（`output\hitomi-hub\cache\images\galleryinfo`）は read-only も非実施・非接触
- 画面確認: **該当なし**（UI 項目でない）。Drive 保存なし（local copy なし）
- 文書矛盾（前批記録どおり・コード優先）: decision `2026-11-09-disk-cache-cap-count-4000` の Status 行
  「実装未着手」は stale（現行コード実装済み）。採用文書はコード＋日付新（Phase 3 decision の file 方式・
  R-15 cap 範囲は full/thumbs）。galleryinfo 層の cap（5000）・order 源は本項目で非接触。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager end`）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし・本項目は findings 記録のみ）
- [ ] `backlog.md` の完了項目を削除（該当なし）

実行固有ID: assess-20261109-1320（実装）

