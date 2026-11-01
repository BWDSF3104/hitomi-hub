# CLI download worker pool checkpoint

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: test, screen verify, commit

## 意図 / 背景
- CLI `download` は file を sequential に fetch し、`processFile` を immediate に呼んでいた。Image gate を下げるだけでは download throughput が上がりにくい。
- Fetch 部分は worker pool にし、DB/scan record は worker pool の後に original page order で sequential に処理する。

## 調査結果 / 変更内容
- `downloadFilesParallel()` を追加し、`Client.getImage()` を worker pool（既定 `4`）で呼ぶ。
- Worker は fetch と `os.WriteFile()` までを行い、`results[idx]` に original page order の slot で結果を保存する。
- `download` command は helper の結果を page order で loop して `recordPage()` と `processFile()` を呼ぶ。
- Test は mock `Client.fetch` で external API を呼ばず、original order と file write を確認。

## Verification
- `go vet ./...`: pass.
- `go test .`: pass, `23.460s`.
- `TestDownloadFilesParallelWritesResultsInOriginalOrder`: pass, mock `Client.fetch` without external API.
- `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`: pass.
- Screen verify at `http://127.0.0.1:8731/?x=downloadpool#/g/4295693/page/1-2`:
  - `PREFETCH_CONCURRENCY`: `4`
  - status `full prefetch 15/15 tn 0/15` for gallery already persisted in image cache.
  - displayed `img.dataset.fullCache`: `cache`
  - displayed `img.src`: `blob:...`
- TN response remains `cache-control: no-store` through the lower image gate.
- Screenshot: `scratch\hitomi-usage\download_worker_pool_screen.png`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
