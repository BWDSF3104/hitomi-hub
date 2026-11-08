# N-09: info disk cache の prune が上限到達時に全 JSON を走査

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

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

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
