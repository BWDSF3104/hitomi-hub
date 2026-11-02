# imageProxy cache atomic write

**Status**: 提案（実装未着手）
**Date**: 2026-11-02
**Next Action**: 実装開始指示が出るまで実装しない

## 意図 / 背景

- 現行の full image cache は既に永続化されている。`imageProxy` は local → `cacheRoot`（既定 `cache/images`）→ remote StorageURL の順で処理し、remote full 成功時に `cachePath` へ `os.WriteFile` する。したがって「未保存だから後から表示できない」という前提で再設計しない。
- shard の時間窓を逃した未取得 full bytes は、後から full actual pixels として復元できない。この制限は維持する。
- full pass と TN 表示状態は別層で扱い、混ぜて数えない。
- TN visual fallback を reader 表示へ入れる案は方針変更であり、このタスクでは扱わない。`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md` で TN display fallback は禁止、TN は search-card cover 専用。
- cache root を `scratch/` や `output/` へ移さない。`scratch/` は canonical 確認用の使い捨て root で、永続先ではない。永続先は製品 root 配下の `cache/images` とダウンロード済み Galleries。
- `image_files` / `gallery_files` のような DB table 追加は不要。cache path は hash + format で決定的に決まり、現行 `serveStoredImage` は DB 参照なしで動作する。`WAL` / `busy_timeout` は既存 DB の話であり、画像 cache の永続化とは無関係。

## 調査結果 / 変更内容

### 現行コードの事実

- `app/viewer.go:41` は `cacheRoot` を `filepath.Join(root, orDefault(cfg.CacheDir, "cache/images"))` で解決する。
- `app/viewer.go:546-557` の `imageProxy` は full cache hit を `x-cache: cache` で返し、remote full 成功時は `x-cache: miss` で応答後に `os.WriteFile(cachePath, body, 0o644)` する。
- `app/viewer.go:568-570` の live GG refresh 成功後も同じ `cachePath` へ `os.WriteFile` する。
- `app/viewer_test.go:339-345` は現行 cache miss 応答後に persisted cache file が存在することを既存テストで確認済み。
- `app/viewer_test.go:378-381` は TN を full-image cache として永続化しないことを確認済み。
- `app/viewer_test.go:591-599` は TN cache path が full cache path と別であることを確認済み。
- `imageProxy` の cache path は `safeCachePath(cacheRoot, hash, format)` で決定的に決まる。DB lookup は不要。

### 承認する改善範囲

このタスクで実施してよいのは、`imageProxy` の full image cache `WriteFile` の atomic 化のみ。

- 対象は `imageProxy` 内の full image cache write。`imageProxy` の remote full success path と live GG refresh success path を同じ atomic helper へ寄せる。
- `thumbProxy` の TN cache write、`acquisition.go` の WriteFile、`main.go` の download WriteFile はこのタスクで変更しない。
- atomic write の形:
  - temp file は同 volume、同 `cacheRoot` 配下に作る。
  - system temp、`scratch/`、`output/` を永続 cache 先として使わない。
  - write → close → rename。close 完了前に rename しない。
  - rename 失敗時は temp を残さず cleanup する。
  - 既存 target がある場合は上書きせず skip する。
  - `os.Rename` の atomicity は same-volume rename に依存する。Windows の temp + rename を前提にする。
- 同 hash / format の同時書き込み対策:
  - singleflight key は `cachePath`。hash だけでなく format を含む path を key にする。
  - local singleflight を使う。外部 package を追加しない。
  - 同 key の重複 write は 1 回に抑制する。
  - response の `x-cache` 意味を変えない。remote full success の応答は現行どおり `x-cache: miss` のままとする。
  - write 失敗時も、応答済みであるため現行 `imageProxy` の response semantics を変えない。helper 側は temp cleanup だけ行う。

### 触ってはいけない範囲

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

### full / TN 判別規則

- TN/full の file magic 判別は使わない。TN と full は同じ webp になりうる。
- 判別には次を使う:
  - `x-cache` の値。
  - `galleryinfo` の width/height に対する実寸。
  - `tn` / `tn-cache` を full cache hit と混ぜない。
  - full pass は `imageProxy` の full cache path だけを `x-cache: cache` として扱う。

## 別セッションで実施する場合の手順

1. 実装開始指示を確認する。このタスクファイルだけでは実装権がない。
2. `app/viewer.go` だけを変更する。変更対象は `imageProxy` の full image cache write に限定する。
3. `app/viewer.go` に internal helper を追加する。例として `writeImageCacheAtomically(cachePath string, data []byte) error` を検討する。
4. 同 path singleflight を internal helper で実装する。既存 `app/gg.go` の singleflight と混ぜず、cache write key `cachePath` で独立にする。
5. `imageProxy` の remote full success branch と live GG refresh success branch の `os.WriteFile(cachePath, body, 0o644)` を helper 呼び出しへ置き換える。
6. `app/viewer_test.go` に tests を追加する。既存 tests は削除しない。
7. `go vet ./...` と `go test ./...` を実行する。
8. canonical build で exe を rebuild し、既存の製品 root で server を再起動し、実機で same hash cache persistence を確認する。`scratch/` を確認 root にしない。

## 追加テスト案

### TestImageProxyCacheMissWritesAtomically

- `imageProxy` の remote full success で `x-cache: miss` を確認する。
- cache path に final file が存在する。
- `.tmp`、`*.tmp-*`、rename 前 temp 残物がない。
- final bytes が fetch bytes と一致する。
- write error path では target が partial で作成されない。
- existing target がある場合、上書き skip される。

### TestImageProxyCacheConcurrentWriteSingleflight

- 同 `cachePath` へ同時 request を送る。
- singleflight が同 key の write を 1 回に抑制する。
- 同時 write のうち 1 回のみ成功し、重複 write が発生しない。
- temp cleanup 後に final file が単一の正しい内容で残る。
- `go test -race` は本地では `CGO_ENABLED=0` により blocked。race test は必須条件にしない。local singleflight で test 可能にする。

### TestImageProxyCacheStateIsNotTN

- `tn` / `tn-cache` cache path を full cache pass に混えない。
- full cache path と TN cache path が別である既存規則を維持する。
- reader display pass を TN で成功扱いしない。

## 実機確認条件

- canonical build 後、既存の製品 root で `hub.exe` server を再起動する。
- `scratch/` を永続確認 root として使わない。
- 同一 product root 上で:
  1. cache に未存在の hash を `GET /api/image/<hash>?format=<format>` で 1 回取得する。
  2. `200` と `x-cache: miss` を確認する。
  3. `cacheRoot` 配下の `<hash>.<format>` final file を確認する。
  4. server を再起動する。
  5. 同じ hash / format を 2 回目に取得する。
  6. `200` と `x-cache: cache` を確認する。
- `x-cache: tn` / `tn-cache` の hit は full persistence pass として報告しない。
- 確認に使用した product root、hash、format、コマンド、結果、経過時間を Verification に記録する。

## Verification

このタスクで実施後に以下を記録する:

- `go vet ./...`
- `go test ./...`
- `python -m pytest tests -v`（既存 canonical freshness tests が必要なら）
- canonical build 後の server 実機確認:
  - first request: `200`, `x-cache: miss`
  - restart
  - same hash second request: `200`, `x-cache: cache`
- commit/push は変更内容の事前承認が必要。commit exclusions は `app/docs/`, `hitomi.db-shm`, `hitomi.db-wal`, `docs/ai/tmp/` を維持。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
