# Phase 3 実装 — gallery 情報の永続キャッシュ（優先1・未開始）

**Status**: 未開始（実装開始指示待ち）
**Date**: 2026-11-05
**Next Action**: 実装未開始・明示的な実装開始指示待ち。開始後は評価記録の推奨（file方式 cache/galleryinfo・3層参照・TTL30日・上限5000/256MB・stale採用推奨）と正典合格条件（mockテスト4件・canonical build 実機確認・decision記録）に従う

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache.md`）の実装タスク。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 1（3項目中。全体順 Phase 1→**P3**→P2→P4。根拠は評価記録の記載のみ。重要度高＝ゲート消費そのものを消す・依存なし）。
- 評価記録（有効性「有効」・実コード＋DB 実測で確定・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase3-gallery-info-cache.md`。
- 検討結果の反映（推奨）: file 方式 `cache/galleryinfo/<id>.json`（parse 済み `GalleryInfo` JSON＋取得時刻・atomic temp+rename A-07a 流儀）。参照順=memory `infoCache`→永続→remote（`info()` 本体実装＝Phase 2 成果の自動永続化、Phase 2 側追加作業不要）。TTL 30 日（既存 TTL 定数ブロック流儀）。上限 5000 件/256MB・取得時刻順 evict（書き込み時間引き・テスト決定化）。stale 暫定表示=採用推奨（metrics/ログで stale 判別可）。CLI 共有は非必須（申し送り）。
- 前実行の known-issues（実装前対処）: `docs/ai/known-issues/KI-20261105-01-noncanonical-running-instance-tn-cache.md`（正典 rebuild・正典 server 再起動・再測定）。

## 調査結果 / 変更内容

（記入）

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
