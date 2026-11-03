# A-10: download が既存ファイルを確認しない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 実装・mock固定テスト合格。live CLI実測は gg.js switch polarity 反転（M()がlive cases内→w2を組み、実測w1=200/w2=404）でblock・未検証。polarity是正の判断（单独項目）後に実在gallery再測→コミット→終了

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/main.go:450-460`（`downloadFilesParallel`）は既存ファイル確認なしで fetch 後 `os.WriteFile`。再実行時に gate 分の quota を消費して上書き。対応: `os.Stat` で既存・size 一致なら skip（または `--overwrite` フラグ）。合格条件: 2 回目 scan/download で remote 呼出 0、written は skipped として報告。推定規模: 小。

実装（2026-11-03・承認範囲内）:
- `downloadFilesParallel` に既存確認を追加: worker 内で `os.Stat` 既存 + size 一致なら remote fetch せず `skipped` 結果として記録。skip は `client.getImage` 前 → image gate 消費 0（A-07b singleflight・C-08 negative cache と無相互作用。テストで fetch 呼出 0 を固定）。
- size 一致の参照は scan 済み DB 記録 `files.size`。`recordedSizes(db, root, galleryID)` helper で path→size を読む。DB に記録のない既存ファイルは size 一致を客観的に確認できないため fetch 対象に残す（skip 条件の誤拡大なし）。
- `downloadEntryResult.skipped` を追加、download コマンドの JSON 報告に `skipped` を追加（skipped 分は recordPage/processFile しない）。
- `--overwrite` フラグは承認範囲外なので未追加。

テスト（合格条件固定・外部 API 呼出なしの mock）:
- `TestDownloadFilesParallelSkipsExistingFileWhenRecordedSizeMatches`: 2 回目 remote fetch 呼出 0、skip 報告、内容保持。
- `TestDownloadFilesParallelRefetchesOnSizeMismatchMissingOrUnrecorded`: size 不一致・欠損・DB 記録なし既存は fetch（回帰、呼出 3）。
- `TestDownloadFilesParallelLiveSecondRunSkipsExistingFiles`: 明示テストモード（`HITOMI_HUB_LIVE=1`）。窓 data が解ける時のみ実呼出し、404 は skip。
- 既存 `TestDownloadFilesParallelWritesResultsInOriginalOrder` を新 signature + `skipped` assertion で更新。

判断点（持ち帰り・実装なし）: 2026-11-03 実測で live `gg.js` switch polarity が decision `2026-11-01-ggset-generation-and-drift.md` line 15（cases 内 → 0 → `w1`）どおり（`var o = 1; switch → o = 0; break`）だが、`GG.M()`（case → 1 → `w2`）とテスト fixture（`var o = 0; case → o = 1`）は 11-01 構造基準で反転。実測: live cases 内 hash（gallery 4301537 file 1、g=2557）は `w1` で 200（320782 bytes）、hub 構築の `w2` URL は 404。この反転により hub CLI download は現状 live で 404 となり「1 回目に attempted と written が一致」の実測が成立しない（shard URL 組み立て変更は承認範囲外。单独項目・decision 更新要）。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app）: ok `37.563s`（新固定テスト含む）。明示モード live test は 404 drift により skip。
- `python -m pytest tests/ -v`（ルート）: 33 passed、`2.37s`（`test_canonical_build_freshness.py` 含む。`ui.html` 変更なし → exe 再 build 不要）。
- Mock 固定テスト実測: 2 run remote fetch 呼出 0（counter）、回帰（不一致・欠損・記録なし）呼出 3。pass。
- CLI 実測（scratch dataDir `docs/ai/tmp/assess-20261103-1220/a10-cli-root/`、scratch exe `a10-hub.exe`。製品 output・永続 cache 非汚染）:
  - run 1（gallery 4301537・live 窓内 hash）: `attempted=1 written=0 skipped=0`、remote fetch 404（hub 構築 `w2`。live actual `w1`）。合格条件「attempted と written 一致」は live 環境側（polarity 反転）で未成立 → live 実測未検証。
  - 現 live rule `w1` で実 bytes 200（`320782 bytes`・epoch 1793702346 前後）seed → `scan` で size 記録 → 2 回目 CLI download: `attempted=1 written=0 skipped=1`（skipped 報告路径実測。image remote 呼出 0・fetch 錯誤なし）。
  - 回帰: 同ファイル +2 bytes（size 320784 vs DB 記録 320782）→ CLI `skipped=0`・remote fetch attempt（404 錯誤表示 = fetch した証拠）。skip 条件誤拡大なし。
  - probe scripts: `docs/ai/tmp/assess-20261103-1220/a10_probe.py`/`a10_probe2.py`/`a10_probe3.py`/`a10_probe4.py`/`a10_seed.py`。live gg `b=1793700002/`・cases 2112・`var o = 1`（cases → `o = 0`）。in-case hash `e3ee8d...`(g=2557): `w1`=200 / `w2`=404。
- 判定: 実装・mock 固定テスト・pytest は合格。live CLI 実測は gg polarity 反転で block → 項目は進行中 maintained（合格扱いせず）。M()/fixture polarity の是正判断後に live 再測して閉じる。
- commit 前再検証（2026-11-03 commit 時・scratch script `docs/ai/tmp/assess-20261103-1220/a10_reverify.ps1`）: `go vet . && go test -count=1 -timeout 300s .`（app）ok `37.941s`（go 節 elapsed `41.669s`）。`python -m pytest tests/ -v`（ルート）33 passed `3.61s`（`test_canonical_build_freshness.py` 含む。`ui.html` 変更なし → exe 再 build 不要）。
- commit: 2026-11-03 ユーザー承認により A-10 分（`app/main.go`・`app/hitomi_test.go`・本記録）のみ単独 commit＋push。live gg polarity 反転で live 実測未検証のまま・項目は進行中 maintained（close せず）。`docs/ai/current-task.tsv`（B-09 行削除混在）は分離 commit へ回すため非含。判断点の polarity 反転は单独項目として別途是正予定、是正後に実在 gallery で live 実測（1回目 attempted==written）を再測して閉じる。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（live 実測成立後）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
