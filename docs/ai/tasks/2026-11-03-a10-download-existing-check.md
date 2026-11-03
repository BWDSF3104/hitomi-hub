# A-10: download が既存ファイルを確認しない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: commit 済み（5d846b4・単独 commit+push・live 実測合格）。項目 close 済み（tsv 行削除は分離 commit 扱い）。窓 rotation 注記: faithful parse が窓ごとに初期o・case代入を読むため app 構築 URL と live 200 の一致は窓によらず維持（mapping 方向 0→w1/atn・1→w2/btn 固定）。
ar o=0/case→1 と 10:00/12:00窓 
ar o=1/case→0 を実測）。mapping方向(0→w1/atn,1→w2/btn)は有効。正典修正は faithful parse（初期o・case代入をliveから読む）＝承認判断点・未実施。A-10 live再測は 
ar o=0 窓でのみ成立（窓依存）。項目進行中・tsv行削除しない

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

polarity是正項目結果（2026-11-03・assess-20261103-1220・code 変更なし）: 承認 premise（fresh live 実測 = decision 側）が 11:38Z 実測で不成立。fresh live `gg.js`（bucket `1793703601/`・cases 1984）は `m: function(g) { var o = 0; switch (g) { case 1284: ... case 1736: o = 1; break; } return o; }`（cases 内 → 1 → `w2`/`btn`）で現行 code・fixture と一致し decision line 15 と矛盾。窓 data 実測（gallery 4301695 全 50 files・11:00窓）: in-case hash `w2`=200/`w1`=404、out-of-case hash `w1`=200/`w2`=404（p8）。同一 hash `e3ee8d44...735fd9`（g=2557）は 10:00窓（`var o = 1`/case→`o = 0`）で `w1`=200、12:00窓（同構造）で live 側 `w1`=200 → 構造（初期 o・case 代入）が窓ごとに rotation。mapping 方向（0→`w1`/`atn`、1→`w2`/`btn`）は固定で有効。TN subdomain `atn`/`btn` は双方 200（p4・interchangeable）。現行 code の片側固定はどちらへ反転させても半分の窓で同じ失敗 → polarity 反転は実施せず。正典修正は faithful parse（`parseGG` に初期 o・case 代入保持、`M()` faithful 化、`gen_ggset.py` 観測 polarity 生成）＝ユーザー承認判断点。記録: decision `2026-11-03-gg-case-polarity-correction.md`（line 15 の構造主張を supersede）、known-issues `2026-11-03-gg-polarity-inverted-vs-live-ggjs.md`。作業前チェック読了: `2026-11-01-root-and-tn-display-correction-policy.md`、`2026-11-01-fast-nonblocking-viewer-policy.md`、`2026-11-01-ggset-generation-and-drift.md`。

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
- polarity是正項目検証（2026-11-03・code 変更なし）: `go vet . && go test -count=1 -timeout 300s .`（app）ok `38.339s`。`python -m pytest tests/ -v` 33 passed `4.28s`。canonical exe 再 build + server 8731 起動（scratch root `polarity-live-root`・scratch dataDir・製品 output/永続 cache 非汚染）: `var o = 1` 窓（12:00窓）では CLI download gallery 4301695 → `attempted=50 written=0`（全 404。code 側 URL が live 側と逆）、viewer `/api/image/<hash>?format=webp` → `404` `x-cache: no_full` → A-10 合格条件この窓未成立（窓依存）。`var o = 0` 窓（11:00窓・p8）は live 側=code 側一致（全 file 200）で条件成立。server は再 build exe で 8731 のまま稼働継続。
- commit: 2026-11-03 ユーザー承認により A-10 分（`app/main.go`・`app/hitomi_test.go`・本記録）のみ単独 commit＋push。live gg polarity 反転で live 実測未検証のまま・項目は進行中 maintained（close せず）。`docs/ai/current-task.tsv`（B-09 行削除混在）は分離 commit へ回すため非含。判断点の polarity 反転は单独項目として別途是正予定、是正後に実在 gallery で live 実測（1回目 attempted==written）を再測して閉じる。

### live 実測再測（2026-11-03・faithful parse 導入後・単独 commit 前）

- 現窓（fresh 取得 epoch `1793709619`）: `var o = 1`・case 代入 `o = 0`・cases 2112・bucket `1793707201/`。faithful mapping: in-cases → `m=0` → `w1`、out-of-cases → `m=1` → `w2`。
  - in-case hash `c05d79ed389022eaf23b396eb41d7e11e9d9dad4dd9704e5295dd589e37d8c63`（g=966）: app 構築 `w1` = 200（300986 bytes）／逆 `w2` = 404
  - out-of-case hash `4a3c2ca208dc33e34f82abb5cb0676cda3a9e145d481fe1c229927ce30f40854`（g=1157）: app 構築 `w2` = 200（246926 bytes）／逆 `w1` = 404
  - 窓依存の注記: 窓 rotation で構造（初期 o・case 代入）が反転しても faithful parse が live から読むため app 構築 URL と live 200 の一致は維持する（mapping 方向 `0→w1/atn`・`1→w2/btn` は固定）。probe: `docs/ai/tmp/assess-20261103-1220/a10_window_final.py`（out `a10_window_final_out.txt`）
- canonical exe 再 build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）で実測。scratch dataDir `docs/ai/tmp/assess-20261103-1220/a10-cli-final/`（製品 output・既定永続 cache `cache/images` 非汚染）:
  - run 1（実在 gallery 4301695・`--limit 3`）: `attempted=3 written=3 skipped=0` → 合格条件1（attempted と written 一致）。written sizes: `300986`/`316356`/`291158`
  - run 2（同一 dataDir・同一 gallery）: `attempted=3 written=0 skipped=3` → 合格条件2（remote 呼出 0、written は skipped として報告）
  - 回帰（skip 条件の誤拡大なし）: `5269b5c6...dfed6c58.webp` に +2 bytes（`316358` vs DB 記録 `316356`）、`b2885b49...c6d61bd0.webp` を正典 `kilo-safe-remove` で欠損化 → run 3: `attempted=3 written=2 skipped=1`（不一致・欠損は取得）。run 3 直後の sizes は live 実測値へ復元（`316356`/`291158`/`300986`）。DB 記録なしの case は mock 固定テストで固定（呼出 3）。
    - 注記: 検証 script（`a10_break_final.py`）の再実行で `5269b5c6...` は `316358`（+2 bytes）になった。合格 evidence は run 3 直後の復元値であり、この +2 bytes は scratch 上の artifact。
- remote 呼出回数（metrics・明示テストモード `HITOMI_HUB_LIVE=1`・`TestDownloadFilesParallelLiveSecondRunSkipsExistingFiles`・gallery 4294614）: run1 `attempted=2 written=2 download fetch calls=2`、run2 `attempted=2 skipped=2 download fetch calls=0`、TN calls `0`。metrics snapshot: `data calls=2`（download の shard full-image fetch）・`gallery calls=1`・`gg calls=1`。download は TN を表示源にしない（TN kind `image` calls 0 を assertion で固定）。
- テスト更新（テストのみ・faithful parse 基準と A-10 skip 判定 `e10f91d` は変更なし）: live test の呼出計上を shard full-image fetch の実際の metrics kind（`classifyFetchURL` の `data`）へ修正（TN の `image` kind を参照していたため呼出 0 と誤読していた）。TN kind 0（download で TN を表示源にしない）の assertion を追加。
- 検証（commit 前）: `go vet . && go test -count=1 -timeout 300s .`（app）ok `37.806s`。`python -m pytest tests/ -v`（ルート）33 passed `3.23s`（`test_canonical_build_freshness.py` 含む・再 build 後の exe）。
- 判定: 合格条件 1〜3 が実測で成立 → 合格。polarity 反転 block は faithful parse（commit `f76dd7a`）で解消済み。項目 close（`task_manager.py end`）。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（live 実測成立後・正典 `task_manager.py end`。commit は分離扱い）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）

---

## faithful parse 実装結果（2026-11-03・assess-20261103-1220・追記・code+テスト commit）

- `app/gg.go` `parseGG()`/`GG` を faithful parse へ修正: 初期 o（`InitialO`）と case 代入（`CaseValue`）を保持、`M(value)` はその gg.js が実際に返す値を返す（case 内 → `CaseValue`、case 外 → `InitialO`）。polarity 固定なし。mapping 方向（0→`w1`/`atn`、1→`w2`/`btn`）維持。`sameGG()` は polarity も比較。
- `tools/gen_ggset.py`: 生成時点の live 観測 polarity を snapshot へ記録（polarity ハードコードなし）。`app/ggset.go` を同じ基準で再生成（観測: `initial_o=1 case_value=0` 窓）。
- テスト: `TestCanonicalGGRuleComposition`（両構造・w1/w2・atn/btn 反転固定）、`TestSnapshotGGIsPopulatedAndUsedAsFallback`（snapshot polarity・faithful M）、`TestStorageURLUsesFormatNotNameExt`・TN 関連テスト（構造差分で subdomain 反転）。EnsureGG/viewer/main fixture を faithful 構造へ更新。
- 現窓（`var o = 1`/case→`o = 0`）live 実測: in-case hash `c05d79ed...e37d8c63`(g=966) app 構築 `w1`=200（300986 bytes）/他 `w2`=404、out-of-case hash `4a3c2ca2...f40854`(g=1157) app 構築 `w2`=200（246926 bytes）/他 `w1`=404。
- canonical exe 実機（scratch dataDir・8731・製品 output/永続 cache 非汚染）: CLI download gallery 4301695（1 回目）`attempted=3 written=3` 一致、`skipped=0`。viewer `/api/image/<hash>?format=webp` → in-case 200 `x-cache: miss` 300986 bytes、out-of-case 200 `x-cache: miss` 246926 bytes（`no_full` なし）。現窓 `var o = 1` で faithful parse が live 実測 200 と一致（固定 polarity なら in-case は 404）。
- 検証: `go vet . && go test -count=1 -timeout 300s .`（app）ok（追加テスト含む）。`python -m pytest tests/ -v` 33 passed（`test_canonical_build_freshness.py` 含む）。
- A-10 の live 実測（1 回目 attempted==written）は faithful parse 導入後、現窓（12:00 窓・`var o = 1`）で成立。窓 rotation で live 実測は窓依存のまま。A-10 項目は A-10 側で live 再測して閉じる（本項目では end しない）。
- decision: `docs/ai/decisions/2026-11-03-gg-faithful-parse-not-fixed-polarity.md`（`2026-11-01-ggset-generation-and-drift.md` line 15 の固定構造主張を supersede・mapping 方向は有効）。作業前チェック読了: `2026-11-01-root-and-tn-display-correction-policy.md`、`2026-11-01-fast-nonblocking-viewer-policy.md`、`2026-11-01-ggset-generation-and-drift.md`、`2026-11-03-gg-case-polarity-correction.md`。

