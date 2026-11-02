# imageProxy cache atomic write

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: commit は変更内容のユーザー事前承認後に実施（現状未コミット）
**Decision**: `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope-correction.md`

## 意図 / 背景

- 現行の full image cache は既に永続化されている。`imageProxy` は local → `cacheRoot`（既定 `cache/images`）→ remote StorageURL の順で処理し、remote full 成功時に `cachePath` へ `os.WriteFile` する。したがって「未保存だから後から表示できない」という前提で再設計しない。
- shard の時間窓を逃した未取得 full bytes は、後から full actual pixels として復元できない。この制限は維持する。
- full pass と TN 表示状態は別層で扱い、混ぜて数えない。
- TN visual fallback を reader 表示へ入れる案は方針変更であり、このタスクでは扱わない。`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md` で TN display fallback は禁止、TN は search-card cover 専用。
- cache root を `scratch/` や `output/` へ移さない。`scratch/` は canonical 確認用の使い捨て root で、永続先ではない。永続先は製品 root 配下の `cache/images` とダウンロード済み Galleries。
- `image_files` / `gallery_files` のような DB table 追加は不要。cache path は hash + format で決定的に決まり、現行 `serveStoredImage` は DB 参照なしで動作する。`WAL` / `busy_timeout` は既存 DB の話であり、画像 cache の永続化とは無関係。

## 現行コードの事実

- `app/viewer.go:41` は `cacheRoot` を `filepath.Join(root, orDefault(cfg.CacheDir, "cache/images"))` で解決する。
- `app/viewer.go:546-557` の `imageProxy` は full cache hit を `x-cache: cache` で返し、remote full 成功時は `x-cache: miss` で応答後に `os.WriteFile(cachePath, body, 0o644)` する。
- `app/viewer.go:568-570` の live GG force refresh 成功後も同じ `cachePath` へ `os.WriteFile` する。
- `app/viewer_test.go:339-345` は現行 cache miss 応答後に persisted cache file が存在することを既存テストで確認済み。
- `app/viewer_test.go:378-381` は TN を full-image cache として永続化しないことを確認済み。
- `app/viewer_test.go:591-599` は TN cache path が full cache path と別であることを確認済み。
- `imageProxy` の cache path は `safeCachePath(cacheRoot, hash, format)` で決定的に決まる。DB lookup は不要。

## 実装範囲（最終版）

このタスクで実施してよいのは、`imageProxy` の full image cache `WriteFile` の atomic 化のみ。

### 変更する 2 箇所

`imageProxy` の remote full success path にある `os.WriteFile(cachePath, body, 0o644)` を、同一 atomic write helper 経由へ置き換える。

- `app/viewer.go:557`
  - 1 回目の remote full success 時の `WriteFile`
- `app/viewer.go:570`
  - GG force refresh / live retry 成功時の `WriteFile`

片方だけの置換は不可。両方と同じ helper を使う。

### 変更しない箇所

- `thumbProxy` の TN cache write。
- `acquisition.go` の WriteFile。
- `main.go` の download WriteFile。
- cache root の場所。
- `scratch/` を永続 cache 先にする案。
- DB schema migration。
- `image_files` / `gallery_files` 追加。
- SQLite `WAL` / `busy_timeout` / schema version。
- TN を reader 表示へ入れる案。
- reader/strip display pass 定義。
- `#reader`、`#thumbs`、search-card cover placeholder の既存方針。
- full actual pixels の合格条件。
- manual retry pass criteria。
- UI click handler、recount、no_full/error entry definitions。

## Atomic write helper 仕様

`app/viewer.go` に internal helper を追加する。例として `writeImageCacheAtomically(cachePath string, data []byte) error` を検討する。

- temp file は `cachePath` と同じディレクトリ、つまり `cacheRoot` 内のみで作る。
- OS の temp directory、`scratch/`、`output/` を temp 先として使わない。別ドライブになり rename 失敗し得る。
- 例: `os.CreateTemp(v.cacheRoot, ...)` のように、temp を `cacheRoot` 内に作る。
- helper は write, close, rename の失敗時すべて temp を cleanup する。成功時だけでなく失敗時にも `.tmp` 残物がないことをテストで確認する。
- existing target がある場合は上書きせず skip する。
- close 完了前に rename しない。
- `imageProxy` の response semantics を変えない。
  - remote full success 応答は現行どおり `x-cache: miss`。
  - cache hit は現行どおり `x-cache: cache`。
  - write 失敗時も、応答済みであるため response は変えない。helper 側は temp cleanup だけ行う。

## Singleflight 仕様（実装前固定）

### Scope

- singleflight の対象は **write のみ**。
- fetch + write は singleflight にしない。
- したがって同 hash / format の同時 request では、remote fetch 自体は重複し得る。このタスクは fetch 重複抑止、レートゲート変更、GG refresh 変更をしない。

### Key

- singleflight key は `cachePath`。
- hash だけでなく format を含む path を key にする。
- external package を追加しない。local singleflight を使う。

### Success / failure semantics

- helper が `nil` を返してよいのは、次の場合だけ:
  - helper 開始時点で target cache file が既に存在する。
  - helper が actual write attempt を実行し、rename 成功で target file が完成した。
- actual write, close, rename 失敗時、helper は error を返す。
- waiting side は「書き込み済み」と誤判定しない。
  - singleflight result は first actual attempt の result を共有する。
  - first attempt が失敗して error を返した場合、waiting side に success を渡さない。
  - target が存在しない限り success を返さない。
- singleflight group 完了後、別の helper call が target を確認し、target がまだ存在しなければ new attempt を実行できる。
- `x-cache` header の意味を変更しない。`x-cache: miss` response は現行どおり。

### Test seam

- `cacheWriteAttemptCount` と `cacheRenameSuccessCount` を viewer 側の internal test-visible seam として持つ。
  - `cacheWriteAttemptCount` は actual write attempt の開始回数を数える。existing target skip は actual attempt として数えない。
  - `cacheRenameSuccessCount` は actual rename 成功回数を数える。
- failure injection hook をテスト seam として持つ。例:
  - `cacheWriteFailureHook func(path string) error`
  - `cacheCloseFailureHook func(path string) error`
  - `cacheRenameFailureHook func(path string) error`
- production path は hook が nil で通常動作する。テストでは hook で write / close / rename 失敗を再現し、temp cleanup と result を確認する。

## full / TN 判別規則

- TN/full の file magic 判別は使わない。TN と full は同じ webp になりうる。
- 判別には次を使う:
  - `x-cache` の値。
  - `galleryinfo` の width/height に対する実寸。
  - `tn` / `tn-cache` を full cache hit と混ぜない。
  - full pass は `imageProxy` の full cache path だけを `x-cache: cache` として扱う。

## テスト条件

### 必須 command

- `app` directory で:
  ```sh
  go vet . && go test -count=1 -timeout 300s .
  ```
- `./...` の省略形にしないこと。
- `-count=1` を落とさないこと。
- repo root に `go.mod` は無いため、`./...` を使わないこと。
- canonical build 後、commit チェックリストの canonical build freshness tests を実行する:
  ```sh
  python -m pytest tests/ -v
  ```

### TestImageProxyCacheMissWritesAtomically

- `imageProxy` の remote full success で `x-cache: miss` を確認する。
- cache path に final file が存在する。
- final bytes が fetch bytes と一致する。
- `.tmp`、`*.tmp-*`、rename 前 temp 残物がない。
- write failure path で target が partial で作成されない。
- write failure path で `.tmp` 残物がない。
- close failure path で `.tmp` 残物がない。
- rename failure path で `.tmp` 残物がない。
- existing target がある場合、上書き skip される。

### TestImageProxyCacheConcurrentWriteSingleflight

- 同 `cachePath` へ concurrent helper calls を送る。
- test seam で以下を確認する:
  - helper call count は 2。
  - `cacheWriteAttemptCount` は 1。
  - `cacheRenameSuccessCount` は 1。
  - `x-cache` には単一の `cachePath` write result しか返らない。
  - duplicate actual write が発生しない。
  - singleflight waiting side が duplicate write success を数えない。
- final file は単一の正しい内容で残る。
- final state に `.tmp` がない。

### TestImageProxyCacheSingleflightFailureDoesNotReportSuccess

- failure hook で first actual write failure を再現する。
- helper は error を返す。
- waiting side は success として result を受け取らない。
- target が存在しない限り `cacheWriteAttemptCount` 1、`cacheRenameSuccessCount` 0、final `.tmp` なし。
- failure hook で close failure、rename failure も同じ成功誤判定規則を確認する。

### TestImageProxyCacheStateIsNotTN

- `tn` / `tn-cache` cache path を full cache pass に混えない。
- full cache path と TN cache path が別である既存規則を維持する。
- reader display pass を TN で成功扱いしない。

## 実機確認条件

### 前提

実機確認に使う hash は、確認開始時点で製品 root の cache にファイルが存在せず、かつ live shard 窓内にあるものを使う。

- 既に cache 済みの hash では 1 回目が `x-cache: miss` にならない。
- 窓外の hash では `no_full` になり、atomic write 手順が成立しない。
- `scratch/` を確認 root として使わない。永続確認先は既存の製品 root である。

推奨: 新規に検索で見つけた gallery の hash を使う。実機確認で使う hash、format、product root、cache path の initial absence、live shard window、command、結果、経過時間を Verification に記録する。

### 手順

1. 製品 root で canonical server を起動または確認する。
2. `cacheRoot` 配下の `<hash>.<format>` が存在しないことを確認する。
3. `GET /api/image/<hash>?format=<format>` を 1 回取得する。
4. `200` と `x-cache: miss` を確認する。
5. `cacheRoot` 配下の `<hash>.<format>` final file を確認する。
6. server を再起動する。
7. 同じ hash / format を 2 回目に取得する。
8. `200` と `x-cache: cache` を確認する。
9. `x-cache: tn` / `tn-cache` の hit は full persistence pass として報告しない。

## 実行順

1. `app` directory で `go vet . && go test -count=1 -timeout 300s .`
2. canonical build:
   ```sh
   go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .
   ```
3. `python -m pytest tests/ -v`
4. canonical server 実機確認。

## 実装前記録（2026-11-02 goal run）

作業前チェック（フル画像・viewer タスクの必須）として読了を確認し、ファイル名を記録する:

- `docs/ai/decisions/2026-11-02-imageproxy-cache-atomic-write-scope-correction.md`
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

実装開始指示: この goal run の指示（修正・実装・検証を一任）をもって実装開始指示とする。

処理項目（優先順位順・逐次）:

1. 項目A — atomic write helper（temp を cachePath と同一ディレクトリ＝ `cacheRoot` 内のみ。write / close / rename 失敗時に temp cleanup。existing target 上書き skip。close 完了前に rename しない）
   - 影響範囲: `app/viewer.go` の `imageProxy` full cache write 2 箇所のみ。`thumbProxy` TN write・`acquisition.go`・`main.go`・cache root・DB は不変。
   - 合格条件: helper 経由で final file が fetch bytes と一致、失敗経路で target partial なし・`.tmp` 残物なし、`x-cache` の意味は不変（miss / cache）。
   - 検証方法: `TestImageProxyCacheMissWritesAtomically`、`go vet . && go test -count=1 -timeout 300s .`。
2. 項目B — singleflight（write のみ、key は `cachePath`、外部 package 不使用、local implementation）
   - 合格条件: 同 `cachePath` の concurrent call で actual write attempt 1、rename success 1、waiting side が失敗 first attempt を success と誤判定しない。existing-target skip は actual attempt に数えない。
   - 検証方法: `TestImageProxyCacheConcurrentWriteSingleflight`、`TestImageProxyCacheSingleflightFailureDoesNotReportSuccess`。
3. 項目C — full / TN 判別規則の維持（file magic 不使用、`x-cache` と galleryinfo 実寸で判別、`tn` / `tn-cache` を full cache pass に混ぜない）
   - 合格条件: full cache path と TN cache path が別である既存規則をテストで維持。reader display pass を TN で成功扱いしない。
   - 検証方法: `TestImageProxyCacheStateIsNotTN`。UI 変更はないため画面操作確認は該当なし（UI 非変更項目）。
4. 項目D — canonical build + commit checklist（`python -m pytest tests/ -v` に `tests/test_canonical_build_freshness.py` を含む）
5. 項目E — canonical server 実機確認（製品 root。hash は確認開始時点で cache 不在かつ live shard 窓内）

## Verification

実施結果（2026-11-02 goal run）:

静的解析・テスト:

- `app` で `go vet .` → clean。
- `app` で `go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 35.207s`（全件 pass）。
- 新規テスト個別: `TestImageProxyCacheMissWritesAtomically` PASS 0.22s、`TestImageProxyCacheConcurrentWriteSingleflight` PASS 0.04s、`TestImageProxyCacheSingleflightFailureDoesNotReportSuccess` PASS 0.11s、`TestImageProxyCacheStateIsNotTN` PASS 0.29s。
- canonical build 後の `python -m pytest tests/ -v` → 25 passed in 2.04s。`tests/test_canonical_build_freshness.py`（`hub.exe` が現行 `app/ui.html` のバイトを含む）pass。build 前 baseline は 25 passed in 5.19s。

Test seam counts:

- existing-target skip: helper call 1、`cacheWriteAttemptCount` 0、`cacheRenameSuccessCount` 0、既存内容 unchanged。
- concurrent（同 `cachePath`）: helper call count 2、`cacheWriteAttemptCount` 1、`cacheRenameSuccessCount` 1、waiting side は first actual attempt の result を共有（両側 nil）、duplicate actual write なし、group は完了後 empty、final file は正しい単一内容、`.tmp` なし。
- failure injection（write / close / rename 各自）: helper は error を返し、waiting side も同じ error を受け取る（success 誤判定なし）、target 作成なし、`cacheWriteAttemptCount` 1、`cacheRenameSuccessCount` 0、`.tmp` 残物なし。group 完了後、target 不在のため new attempt が実行できる（attempt 2、rename success 1）。
- join 検出のため `cacheSingleflightJoinHook` を internal test seam として追加（production は nil）。write / close / rename failure hook と 2 counts の条件は維持。

実機確認（canonical server・製品 root）:

- product root（server cwd）: `S:\Documents\Kilo_Commons\hitomi-hub`、cache root: `S:\Documents\Kilo_Commons\hitomi-hub\cache\images`
- server: canonical `output\hitomi-hub\hub.exe view`（pid 12424、background id `bgp_19ce0e4b2001NSfjednFbVbVqD`）
- gallery id 4299649、hash `a75177974fe57f71d2d3e005ea7d77225aac6068bc2a40b3dadab36e91047cb1`、format `webp`
- cache path initial absence: `cache\images\a75177974fe57f71d2d3e005ea7d77225aac6068bc2a40b3dadab36e91047cb1.webp` → 確認開始時 不在
- live shard window（確認時 live `gg.js`）: `b = 1793631601/`（2026-11-02T15:00:01Z の窓）。hash の `s(hash)`=459。1 回目 200 が窓内の証拠
- 1 回目 `GET /api/image/<hash>?format=webp` → `200`、`x-cache: miss`、426626 bytes。final file 存在 426626 bytes、`.tmp` 残物 0、経過 2.77s
- server を canonical exe から再起動
- 2 回目 同 hash / format → `200`、`x-cache: cache`、426626 bytes、cache file と response の SHA256 一致（`23D5832FEE19F200978C2FF0654B594C3D29A46EA262395E9597943B450E69ED`）、経過 0.17s、`.tmp` 残物 0
- `x-cache: tn` / `tn-cache` hit は full persistence pass として報告していない（full pass は full cache path の `x-cache: cache` のみ）

画面確認（canonical server が提供する UI。UI ファイル変更はないが full 表示経路の確認として実施）:

- 実際の DOM 判定: `#reader` の img が `sourceType=objectURL`（full 経路 `/api/image/<hash>`）で `naturalWidth > 0`。page1 hash `a7517797…` → 2400x8000、page2 hash `7630f33e…` → 1405x2880。`x-cache` 判定は full 経路のみ使用。
- UI 表示状態: `actual pixels 36/425 | local direct 0/425 | objectURL cache 207/425 | no full 0/425 | TN display fallback disabled`
- 保存先: `I:\マイドライブ\kilo\2026-11-02-imageproxy-cache-atomic-write\actual-ui-reader-full-pixels-20261102.png`
- 確認結果: reader に actual full pixels 表示、TN display fallback disabled、no full 0 → 合格。

commit/push は変更内容の事前承認が必要（未コミット）。commit exclusions は `app/docs/`, `hitomi.db-shm`, `hitomi.db-wal`, `docs/ai/tmp/` を維持。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（正典 `task_manager.py end` で実施）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（該当なし）

## 変更ファイル

- `app/viewer.go`: `writeImageCacheAtomically` / `writeCacheFileOnce` 追加、`imageProxy` の 2 箇所（remote full success、GG force refresh 後の再試成）を同一 helper 経由へ変更。TN write・acquisition・download・cache root・DB は未変更。
- `app/viewer_test.go`: 4 テスト追加（atomic write、concurrent singleflight、singleflight failure、full/TN 判別）と `assertNoCacheTempResidue` helper。
- `docs/ai/tasks/2026-11-02-imageproxy-cache-atomic-write.md`: 実装前記録と Verification。
