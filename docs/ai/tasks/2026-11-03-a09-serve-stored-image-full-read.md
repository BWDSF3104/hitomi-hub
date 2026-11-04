# A-09: serveStoredImage の全読み

**Status**: 完了
**Date**: 2026-11-03（実施 2026-11-04・実行固有ID assess-20261103-1220）
**Backlog**: assess-20261103-1220
**Next Action**: なし（完了）

## 意図 / 背景

`serveStoredImage` が保存済み画像を `os.ReadFile` で全readしてから応答していた。承認済み方針（`docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md`）により stream + 条件付き応答（304/206・ETag・Range・`http.ServeContent` 相当）へ変更する。UI full 判定（no-store fetch・x-cache・generation guard）と full actual pixels 判定基準には影響しないこと。

## 調査結果 / 変更内容

作業前チェック（読了確認・ファイル名明記）:
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`

記録の行番号 `app/viewer.go:269-276` は実施時点で `app/viewer.go:313-320`（A-07a 等の追記により移動）。記載された挙動（全read・304/Range 非対応・cache path cache-control `private, max-age=604800`）は実測と一致し、内容上の食い違いはない。

変更（承認範囲: `app/viewer.go` の serveStoredImage 関連＋テストのみ）:
- `serveStoredImage(w, r, format, path, cacheState)`: 全read廃止。`os.Stat` + `os.Open` + `http.ServeContent`（stream）へ。headers（content-type・cross-origin-resource-policy・cache-control〔tn のみ no-store〕・x-cache）は従来通り先に設定。ETag は stat の modtime+size から付与（`http.ServeContent` は 200 時に自前で ETag を設定せず、`If-None-Match` は caller 設定の ETag と比較するため caller 側で明示設定）。Last-Modified・Accept-Ranges・Content-Length・Content-Range・304/206 は `ServeContent` が生成。
- 呼び出し側 4 箇所（serveFile・thumbProxy・imageProxy・serveLocalImage）に `r` を追加（route 定義は変更なし）。
- cache write path（temp+rename・magic 照合 A-07a）・TN 表示禁止・full actual pixels 判定は未変更。UI（ui.html）は無変更。
- テスト追加: `app/viewer_test.go` `TestServeStoredImageStreamsConditionalResponses`（206・304・200+Accept-Ranges・ETag、x-cache cache/local 維持、cache-control 維持、stored hit で network 呼び出しなし、local ルート条件付き 304）。

## Verification

1. app で `go vet . && go test -count=1 -timeout 300s .` → 通過（15.96s）。1回目に `TestSearchCacheSizeCapBoundsMemory`（本項目と無関係）が単独実行では通過するが suite 全体で 1 回失敗（同タイムスタンプの LRU tie と思われる flaky）。既知問題化はスコープ外のため未作成、parent へ報告。2回目以降の suite 全体通過。
2. canonical exe rebuild: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`。稼働中の canonical server なし（開始時状態＝未稼働）。rebuilt exe から `http://127.0.0.1:8731` で起動（pid 23268）。
3. 実測（canonical server・cached hash `001313d703107affb5ad5406aa740f64c731dcbcfa17c36a087e7cb77046e5bc`・format=avif・168505 bytes・gallery=4300588。スクラッチ `docs/ai/tmp/assess-20261103-1220/a09_measure.ps1`・hdr1〜hdr4.txt）:
   - plain GET（条件ヘッダなし）→ 200・Accept-Ranges: bytes・Etag `"6ae76c2a-29239"`・Content-Length 168505・x-cache: cache・cache-control `private, max-age=604800`・content-type image/avif
   - `Range: bytes=0-1023` → 206・Content-Length 1024・Content-Range `bytes 0-1023/168505`・x-cache: cache・返送 1024 バイトは全文先頭 1024 バイトと一致（True）
   - `If-None-Match: "6ae76c2a-29239"` → 304・本文なし・x-cache: cache・cache-control 維持・Etag エコー
   - local hit（`09e34fdfff2fb198be676b313aa97e942c19f03667ecfca69ef7214fba8ad2a6` webp・gallery 4300588）→ 200・x-cache: local・Accept-Ranges: bytes（x-cache cache/local 意味維持）
   - UI no-store fetch 実測（fresh browser tab・UI 同一 `cache: "no-store"` fetch）→ status 200（304 なし）・x-cache: cache・blobSize 168505
   - UI 画面実測（gallery 4300588 page 1・fresh tab・server 直接基準後）: `#reader` img objectURL naturalWidth 1740・local direct naturalWidth 1740（full actual pixels 判定・objectURL 表示・generation guard 弱められていない）
4. スクリーンショット（Google Drive・タスク別フォルダ）: `I:\マイドライブ\kilo\2026-11-04-a09-serve-stored-image-full-read\a09-nostore-fetch-200-and-full-pixels-2026-11-04.png`・手順テキスト `a09-serve-stored-image-confirmation-2026-11-04.txt`（同フォルダ）。repo 内スクショは保存しない（一時ファイルはゴミ箱へ）。確認結果: 合格。
5. canonical server はテスト後、開始時状態（未稼働）へ復元（pid 23268 stop 済み、8731 の listener なし）。
6. 回帰: `python -m pytest tests/ -v` 40 passed（freshness 含む）。既存 viewer テスト（x-cache・no-store・path guard・A-07a magic）通過。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（task_manager end で実施）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [x] `backlog.md` の完了項目を削除（本項目は assess 実行計画項目。plan/backlog ファイルは親の記録用・スコープ外のため未変更）
