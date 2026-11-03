# A-09: serveStoredImage の全読み

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/viewer.go:269-276`（`serveStoredImage`）は `os.ReadFile` で全readし `writeImage` で書く。実測: cached hash `001313d703107affb5ad5406aa740f64c731dcbcfa17c36a087e7cb77046e5bc`（format=avif, gallery=4300588）で `Range: bytes=0-1023` → 200・全 168505 bytes（Accept-Ranges/Content-Range なし）、`If-None-Match: *` → 200（304 なし）。cache path の Cache-Control は `private, max-age=604800`（UI は `cache: "no-store"` fetch を使うので UI 判定は守られている）。対応: stat + 条件付き（304/ETag・Range）またはストリーム書き込み。合格条件: 304/206 が返ることを実測、full actual pixels 判定に影響しないこと。推定規模: 小。フル画像ガード関連: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
