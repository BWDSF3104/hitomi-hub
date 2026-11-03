# A-07a: cache 書き込み前の magic 検査がない

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 調査（assess-20261103-1220）。完了後実装タスクへ引き継ぎ

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効。`app/viewer.go:315-371`（`writeCacheFileOnce`）は remote bytes を magic/format 検査なしで cache へ書く。scan 側の magic 検査は gallery ファイルに対してのみ（`scan_file`）。対応: 書き込み前に format magic（webp=RIFF/WEBP, avif=ftypftypavif）を照合し、不一致は cache しない。合格条件: 不正 bytes で cache ファイルが増えない、既存テスト `TestViewerImageProxyCachesBytes`（mocked fetch）を更新して照合。推定規模: 小。フル画像ガード関連（cache 書き込み）: `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md` 参照。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
