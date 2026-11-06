# R-16: TN 書き込みと download 書き込みが非 atomic（full cache の temp+rename+magic と非対称）

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 実施開始指示待ち。full cache の writeImageCacheAtomically（A-07a）を共通化し、TN と downloadFilesParallel（main.go:737）へ適用。合格条件: write/close/rename 失敗で壊れた target が残らない。記録: R-16

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

## Verification

- 変更は本記録とスクラッチのみ（`docs/ai/tmp/assess-20261106-1056/r16/`）。ソースコード・既存テスト・実 cache フォルダ・実 DB は未変更。スクラッチ共有フォルダでの package main 衝突により本項目分は `r16/` サブフォルダへ隔離（go.mod・`r16measure.go`）。
- 実測コマンド: `go run .`（`docs/ai/tmp/assess-20261106-1056/r16/`）→ 合計 6.787 s、結果は上（A1 partial 131072/262144、A2 complete 199/mixed 1、B2 complete 200・mixed 0・rename error 305、C1 success・C2 Access denied・C3 success、D2 mixed 56/200）。
- `go vet . && go test -count=1 -timeout 300s .`（app）→ vet clean（出力なし）、`ok hitomi-hub/app 25.797s`（全通過・コード非変更）。
- `python -m pytest tests/ -v` → 54 passed・1 failed・10.96 s。failed は R-10 が報告済みの既存 failure（AGENTS.md Rules 条の pointer 集約、`test_canonical_build_freshness_rule_exists`）で R-16 と無関係（R-16 のコード非変更で再現）。
- 画面確認: 該当なし（UI 項目でない）。Drive 保存なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
