# R-16: TN 書き込みと download 書き込みが非 atomic（full cache の temp+rename+magic と非対称）

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 実施中（計画確定 2026-11-08）。core writeAtomically 共通化 → TN（thumbProxy）と downloadFilesParallel へ適用・失敗注入テスト。合格条件: write/close/rename 失敗で壊れた target が残らない。記録: R-16

## 意図 / 背景

- 外部指摘 R-16 は TN cache 書き込み（`app/viewer.go:780` の `os.WriteFile`）と CLI download 書き込み（`app/main.go:737` の `os.WriteFile`）が、full cache の atomic write 経路（temp + close + rename・テスト seam・magic 照合・失敗時 temp cleanup・singleflight）と非対称であるという指摘。
- 本アセスメントは読み取り＋スクラッチ実測のみ。ソースコード・既存テストは変更していない。変更したのは本タスク記録とスクラッチ（`docs/ai/tmp/assess-20261106-1056/r16/`）のみ。
- 実測で閉じる論点: 同一 key（同一 TN cache path）への同時書き込みで partial write（断片・混ざり）が発生するか否か、rename の Windows semantics、temp 名生成の差異。

## 調査結果 / 変更内容

### 現在の実装（読み取り確定）

- TN: `thumbProxy` は応答後 `os.WriteFile` 直書き（`app/viewer.go:780`）。temp なし・singleflight なし・書き込み時 magic 照合なし。`serveStoredImage` は読み取り側に magic 照合を持たない（`app/viewer.go:350-371`）ため、壊れた TN は `tn-cache` として永続返答される。TN disk cache には prune・無効化機構がない（R-15 照合）。
- download: `downloadFilesParallel` の worker が `os.WriteFile` 直書き（`app/main.go:737`）。worker 内では entry path が一意（hash+format）で worker 間衝突は通常ないが、クラッシュ時 partial が completion フォルダに残る。再実行時の size 一致 skip 条件（`app/main.go:726-731`）で次回再 download による自己修復はある。scan 側 magic 照合（`magic_mismatch`）で検出可能。
- full cache（正典対照・A-07a）: `writeImageCacheAtomically`（`app/viewer.go:377-408`）+ `writeCacheFileOnce`（`app/viewer.go:423-483`）。existing target skip・key = cachePath の singleflight・webp/avif magic 照合・write/close/rename 失敗時 temp cleanup・`cacheWriteAttemptCount`/`cacheRenameSuccessCount` と failure hook のテスト seam を持つ。

### 実測（スクラッチ `docs/ai/tmp/assess-20261106-1056/r16/`、NTFS・Go、200 rounds × 4 writers × 256 KiB、合計 6.787 s）

- A1 TN 直書き中のクラッシュ（close 前 exit）: **target path に partial が残る**（131072/262144 B）。`serveStoredImage` は magic を確認しない（読み取り確定）ので、この partial は `tn-cache` として永続返答 → 指摘の「クラッシュ時、壊れた TN が tn-cache として返り続ける」は実測で有効。
- A2 TN 同一 key 同時直書き: complete 199 / mixed 1 / partial 0（mixed 率 0.5%）。混ざりは発生し得るが低頻度。主リスクは crash による partial（A1 で 100%）。
- B1 atomic 経路のクラッシュ（close/rename 前 exit）: **target 不存在・temp に partial（131072 B）が残る**。target は壊れた状態にならない。temp は `serveStoredImage` の読み取り対象ではない。正典 helper の temp cleanup は write/close/rename 失敗時走（`app/viewer.go:442-477`）で、クラッシュでは走らない（temp 残りは残存点）。
- B2 atomic 同時 rename（CreateTemp 一意名）: complete **200/200**・mixed 0。rename 衝突は上書きであり内容が混ざらない。rename error 305 件（15%）= Windows の contention 下で `os.Rename` が失敗する例（失敗は error として扱われ content 破損にはならない）。
- C（Windows rename semantics）: C1 既存 target への rename は成功（上書き）。C2 **open 中の target（Go handle）への rename は Access is denied で失敗**。C3 open 中の target への `os.WriteFile` は成功（TN 直書きは読み配信中 target へ書ける＝混ざり可能）。
- D（temp 名生成）: `os.CreateTemp` の名前は一意（同一パターンでも衝突なし）→ 正典 temp は共有 temp であり得ない。固定名共有 temp 案は混ざり 56/200（28%）→ 修正時は CreateTemp（一意名）でなければならない。

### 有効性

**有効**。ただし指摘原文の表現に 1 点訂正: 「TN 側は temp を共有せず、rename 衝突で内容が混ざる」は TN に rename が存在しないため表現として不正確。機構として確定するのは:
1. TN の partial write リスク（crash・close 前中断で target に断片が残る・読み取り側 magic 照合なしで永続化）— 実測確定（A1）。混ざりは低頻度実測 0.5%（A2）。
2. atomic 経路の rename 衝突は上書きであり混ざらない（B2 200/200）。download 側の partial write 自己修復・scan magic 検出は TN より影響が軽い（TN は永続、download は再実行修復・scan 検出可）。

### 正典照合（TN 対象外の旨・方針変更の提案として分離）

- 実測で確認した正典: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope.md`（実在・Superseded by correction）と `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope-correction.md`（実在・正典）。
- 原 decision の user-confirmed proposal scope は atomic 化の対象を「`imageProxy` full image cache の WriteFile のみ」とし、`tn` / `tn-cache` handling・acquisition write behavior を**明示的に除外**。supersede 後の正典（correction）の Write sites も full cache の 2 書込点のみ。両文書は TN を現行正典の atomic write 適用範囲外としている点で一致（状態記述の矛盾なし・原文書は歴史的記録として除外条の所在を明記）。
- さらに原 decision に実装 guard（明示的な実施開始指示が必要）が記録済み。したがって **TN と download への atomic write 適用は記録済み proposal scope の拡張＝方針変更の提案**であり、本記録では実装案ではなく提案として分離する（推奨は下記）。
- 維持すべき不変条件への影響: TN を full cache に保存しない／TN を full actual pixels に数えない／`x-cache`（tn・tn-cache）判定を弱めない／TN display fallback 禁止／reader 表示源化禁止 — 推奨案（thumbs ディレクトリ内 temp+rename、x-cache 応答変更なし）はいずれにも触れない。

### 推奨修正方法（承認後の実装案・正典 guard により実施開始指示が必要）

- `writeImageCacheAtomically`＋`writeCacheFileOnce` を helper として共通化し、TN（temp は `cacheRoot/thumbs` 内・`os.CreateTemp` 一意名）と CLI download（temp は target フォルダ内）へ適用。合格条件は Next Action 記載どおり（write/close/rename 失敗で壊れた target が残らない・失敗時 temp 残りのテスト）。
- TN 実装の注意点（正典条件の追加）:
  - magic 照合は TN path の `filepath.Ext`（variant 接尾 `.bigtn`）では機能しない。format はリクエスト側（webp/avif）から渡す必要がある（`writeCacheFileOnce` 現行の format 導出（`app/viewer.go:424`）は TN 流用時に variant 接尾を format と誤読する）。
  - Windows semantics: 読み配信中（open 中）の target への rename は失敗し得る（C2 実測）。TN 通常流（stat miss → write）では非衝突だが、singleflight・existing-target skip の error handling で吸収できる。
  - temp 名は CreateTemp（一意）を維持（固定名案は 28% 混ざり実測）。
  - `x-cache` 判定（tn / tn-cache）と応答順序（write 前に応答）は変更しない。existing target skip 意味は TN 現行動作（hit 時 serve）と整合。
- TN 書き込みの singleflight は単独では未規定。R-13（TN singleflight・設計承認待ち）と同じ key 設計（cachePath 単位）に揃えるのが流儀との整合上推奨。本項目は R-13 の承認に依存する（単独実施時は write-only dedup の追加として R-13 と同じ承認構造が必要）。

### 既存テスト照合

- TN 書き込みの atomicity を固定するテストは**不存在**。既存テストは TN の「thumbs 下への永続化」「tn-cache 返答」「TN が full cache にならない」のみ固定（`app/viewer_test.go:1184-1211`、`4020-4050`）。失败注入・`.tmp` 残りの検査は full cache helper のテストのみ（`app/viewer_test.go:3829-3850`）。正典 correction の「失敗時に `.tmp` 残りを確認するテスト」要求は full cache 書込点に対するもので、TN 拡張時は新テストが必要（既存テストは temp+rename 化でも通過）。
- UI 項目ではない（disk 書き込み機構）。画面確認・Drive 保存は該当なし。

### 推定規模・優先順位付けに効く要素

- 推定規模: 小（helper 共通化＋2 書込点の配線＋新テスト数件）。ただし TN 適用は正典 guard・proposal scope 拡張の承認を要る。
- 重要度: low〜mid（整合性欠陥・高頻度バグではない）。
- 緊急度: low（crash・中断でのみ発火）。影響の永続性が TN 側で重い（読み取り magic 照合なし・prune なし・無効化なし → 手動削除まで壊れた TN が返り続ける）。download 側は再実行修復・scan magic 検出があり軽い。
- リスク: 修正自体は低。Windows の open-target rename 失敗（C2）と temp 残りの扱い（クラッシュ時）が運用条件。混ざり実測 0.5%（TN 直書き）は低頻度。
- 依存: A-07a 正典 helper（test seam 流儀）、R-13（TN singleflight 承認待ち・key 設計）、R-15（TN disk cache 上限・prune 方針）、正典 guard（実施開始指示）。

### 実装計画（確定 2026-11-08・assess-20261106-1056・R-16 scope 拡張はユーザー承認済み）

- 正典前作業チェック（viewer・download 項目として読了・ファイル名明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`（正典対照: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope-correction.md`）。承認済み R-16 記載: `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（本項目の scope 拡張はユーザー承認済みとして実装）。
- 共通化の形（推奨案の採用・段階1）: core 関数 `writeAtomically(seam, targetPath, format, data)` へ正典 helper の機構を集約（magic 照合は format 明示引数・temp は target 同一 dir 内 `os.CreateTemp` 一意名 `.cache-*.tmp`・write → close → rename・close 完了前に rename なし・write/close/rename 失敗時 temp cleanup）。正典不変（magic 照合・temp+rename・close 前 rename なし・失敗時 temp cleanup）は弱めない。
- seam: `atomicWriteSeam`（cacheWriteMu・cacheWriteAttemptCount・cacheRenameSuccessCount・write/close/rename failure hooks）を viewer に embed（既存テストの `v.cacheWriteMu` 等の参照は promotion で不変）。download 側は seam nil（counter・hook なし）。
- singleflight wrapper `writeImageCacheAtomically(cachePath, format, data)`: key = cachePath・existing target skip・join result 共有（正典 singleflight spec 維持）。TN はこの wrapper を使用（TN write dedup は R-13 と同じ key 設計 cachePath 単位に揃う・推奨案どおり。R-13 案 A は承認済み（`ba0acd7`）なので追加承認不要）。download は worker path 一意（B2 実測 200/200）のため core 直呼び・existing skip なし（rename 上書きで size 不一致 self-repair 維持。worker 内 DB size 一致 skip は非触）。
- TN 配線（`thumbProxy`）: 応答順序（write 前の応答）・x-cache（tn / tn-cache）判定・TN display fallback 禁止・TN バイトを full cache に保存しない・reader 表示源化禁止は変更なし。format はリクエスト側から渡す（`filepath.Ext` の variant 接尾 `.bigtn` は magic 照合に使えない・正典条件の追加どおり）。R-15 の `recordAndPruneCacheEntry` は write 成功時（rename 成功）のみ呼ぶ条件で不変。既存 `os.MkdirAll`（thumbs）は core の target dir MkdirAll に統合（同一 dir・thumbs 内）。
- download 配線（`downloadFilesParallel`）: core 適用（seam 引数追加）。webp/avif の magic 不一致は error として result に記録（scan 側 `magic_mismatch` 検出と整合）。既存テスト mock の非 magic バイト（"x"・"z"）は webp magic payload へ更新が必要（訂正: 記録の「既存テストは temp+rename 化でも通過」は magic 照合の TN・download 適用により不正確。既存テストの fixture 更新を要る）。UI 項目でない（画面確認・Drive 保存該当なし）。
- 新テスト: TN write/close/rename 失敗注入（壊れた target 残らず・thumbs dir に `.tmp` 残らず・応答 200 tn 維持）、TN write singleflight（key = TN path・helper 2 呼・actual write 1・rename 1・residue なし）、download write/close/rename 失敗注入（target 残らず・residue なし）、download magic 不一致（error・target 残らず）。

## Verification

- 変更は本記録とスクラッチのみ（`docs/ai/tmp/assess-20261106-1056/r16/`）。ソースコード・既存テスト・実 cache フォルダ・実 DB は未変更。スクラッチ共有フォルダでの package main 衝突により本項目分は `r16/` サブフォルダへ隔離（go.mod・`r16measure.go`）。
- 実測コマンド: `go run .`（`docs/ai/tmp/assess-20261106-1056/r16/`）→ 合計 6.787 s、結果は上（A1 partial 131072/262144、A2 complete 199/mixed 1、B2 complete 200・mixed 0・rename error 305、C1 success・C2 Access denied・C3 success、D2 mixed 56/200）。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean（出力なし）、`ok hitomi-hub/app 25.797s`（全通過・コード非変更）。
- `python -m pytest tests/ -v` → 54 passed・1 failed・10.96 s。failed は R-10 が報告済みの既存 failure（AGENTS.md Rules 条の pointer 集約、`test_canonical_build_freshness_rule_exists`）で R-16 と無関係（R-16 のコード非変更で再現）。
- 画面確認: 該当なし（UI 項目でない）。Drive 保存なし。

### 実装実施（2026-11-08・計画確定どおり）

- 変更ファイル: `app/viewer.go`（`atomicWriteSeam`・core `writeAtomically`・`writeImageCacheAtomically(cachePath, format, data)`・`writeCacheFileOnce` binding・`thumbProxy` TN 配線）、`app/main.go`（`downloadFilesParallel` seam 引数・worker が core 直呼び）、`app/viewer_test.go`・`app/hitomi_test.go`・`app/metrics_test.go`（fixture webp magic 更新・新テスト 4 件）。
- `go vet .` → clean。`go test -count=1 -timeout 300s .`（app）→ `ok hitomi-hub/app`（新テスト: TN write failure 残留なし・TN singleflight key = TN cachePath・download failure 残留なし・download magic 不一致 error）。
- `python -m pytest tests/ -v` → 57 passed・1 failed（既存 failure `test_canonical_build_freshness_rule_exists`・KI-20261106-04・R-16 と無関係）。`test_canonical_executable_embeds_current_pages` PASSED。
- Canonical build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → 生成済み。
- CLI download 実機（実在 gallery 4294614・scratch p3_canonical_fresh）: attempted=2・written=2・skipped=0（一致）。ファイル: `2d18a9a8...` 210784 B・`76f4e315...` 183038 B。elapsed 2.21 s。
- viewer full 実機（canonical server 8731）: `GET /api/image/9d990fde5de572e0cccd806f3f87258f6a7905d0e99a2ddb13254342?format=webp` → 1st 200 `x-cache: miss` 193724 B、2nd 200 `x-cache: cache` 193724 B（`no_full` でない・cache ファイル実寸 193724 B・rename 由来）。`.tmp` 残留: cache 配下・scratch 全体で none。
- TN 実機: 既存 TN cache hit 確認（`GET /api/thumb/2d18a9a8...?format=webp&gallery=4294614&variant=bigtn` → 200 `x-cache: tn-cache` 55320 B・thumbs 内ファイル 55320 B）。TN 新規 write（tn → tn-cache）は TN upstream が 502 を返せず、確認できなかった
- TN live write 再試行（2026-11-07・1 回のみ・記録のみで code 変更なし）: freshly rebuilt canonical `hub.exe`（freshness test `test_canonical_executable_embeds_current_pages` PASSED・pytest 57 passed・failed 1 は既存 failure `test_canonical_build_freshness_rule_exists`・KI-20261106-04）からの **fresh server instance**（pid 13760・port 8731）・cold root `scratch/r16_tn_retest`（測定時点で cache ディレクトリなし）で TN 新規 fetch→write を測定 → **TN upstream が再び 502**（2026-11-07 17:42 UTC 単発・x-cache ヘッダなし＝TN error path・display fallback 非接触・write 走らず）。cold root 確認: `hitomi.db` (+shm/wal) のみ・`cache/images`（full）・`cache/images/thumbs` 未生成＝TN バイトが full cache に保存されていない・`.tmp` 残留 0。TN display fallback 禁止・reader 表示源化禁止の正典は非接触（error path 確認まで）。→ **TN live write は未確認点として確定**（テスト固定済みの失敗注入機構は維持）
- 画面操作は実施しなかった（TN 新規 write 不合格で screen path 到達不可・network 経路のみ）→ Drive 保存該当なし・local copy なし → **TN live write は未確認**（テストで固定済みの機構・実機確認は tn-cache 読みのみ）。
- 合格条件（write/close/rename 失敗で壊れた target が残らない）: テストで確定・実機では full cache write 成功経路（miss→cache・residue なし）と download 一致を確認。TN live write 失敗注入は実機未実施（upstream 502）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
