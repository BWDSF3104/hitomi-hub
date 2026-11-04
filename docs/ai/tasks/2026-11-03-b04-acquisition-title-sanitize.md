# B-04: acquisition のフォルダ名と確認処理の不一致

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了。単独 commit へ反映（check 側 acquisitionTitleName 統一・テスト固定・修正前対照実測）

## 意図 / 背景

check（`acquisitionCheck`）が確認する folder path と、acquire（`acquire`）が書く folder name が title の無効文字で食い違い、check が not_exists/confirm を誤報告する。承認済み方針（`docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md` B-04 条）: check 側を `acquisitionTitleName`（sanitize 名）へ統一。acquire 側命名は変更しない。

## 調査結果 / 変更内容

- 実装（承認範囲内）: `app/acquisition.go:88` の `acquisitionCheck` で、無加工 title（`acquisitionTitleFromDB`）直接利用をやめ、`acquisitionTitleName(acquisitionTitleFromDB(...), strconv.Itoa(body.GalleryID))` へ変更。acquire（`app/acquisition.go:163`）と同一関数・同一 fallback で folder を指す。check 応答の `title` は sanitize 名（acquire 応答と一致）。既存挙動（空 title → gallery_id fallback、`..` の 403）は不変。`D-10追加` の `SanitizeTerm`（正典 space 形）とは別関数（混同しない）。
- テスト固定（追加）: `TestViewerAcquisitionCheckUsesSanitizedTitleFolderMatchingAcquire`（`app/viewer_test.go`）。fixture title `what?/file` → sanitize 名 `what__file`。check（copy 前 not_exists の path）= `acquisition/what__file`、acquire が同一 folder へ `001.png` を書き、copy 後 check が confirm を返すことを実測（check path と acquire path の `filepath.Dir` 一致で合格条件を検証）。
- 修正前対照（実測・scratch `docs/ai/tmp/assess-20261103-1220/b04-prefix/`）: HEAD 版 `acquisitionCheck`（無加工 title）+ 同テストで FAIL — check が `acquisition\what?\file` を報告し、acquire の folder `what__file` と不一致。static 判定（静的）が実測で確認された。
- 既存テスト `TestViewerAcquisitionCheckReturnsRunStartFolderStatus`（title `Kemo`・無効文字なし）は sanitize 名前後も PASS（sanitize("Kemo") == "Kemo"）。

## Verification

- `go vet .`（app）: clean（出力なし）
- `go test -count=1 -timeout 300s .`（app・suite 全体）: ok **16.493s**（既知 flaky `TestSearchCacheSizeCapBoundsMemory` 発火なし）
- 固定テスト単独: `TestViewerAcquisitionCheckUsesSanitizedTitleFolderMatchingAcquire` PASS **0.04s**（修正後の実コード）/ 修正前 scratch 対照 FAIL（上記根拠）
- ルート `python -m pytest tests/ -v`: **40 passed 7.79s**（`test_canonical_build_freshness.py` 含む）
- canonical exe 再 build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）: 成功（server 非起動・UI 実操作確認は親実施）
- 実機確認（parent・2026-11-04・正典 exe 再 build 後 hash `24D0CFAD72C1D5029F2C2AC4E9F9A43A9CA0D18C02626D4802D8EB48624BDED3`・scratch dataDir/root `docs/ai/tmp/assess-20261103-1220/b04-live`・`--no-vt`・正典 server 8731・served UI 経由 fetch・seed gallery 900101 title `what?/file`・cache fixture `abc123.webp`）:
  - check1 → 200 `not_exists`・title `what__file`・path `acquisition\what__file`（sanitize 名）
  - acquire → 200 `copied`・path `acquisition\what__file\001.webp`（check が指す folder と同一 folder へ書かれた）
  - check2 → 200 `confirm`・path `acquisition\what__file`（check と acquire が同一 folder を指すことを実測）
  - server 停止後、開始時状態（8731 未稼働）へ復元。正典 DB は非使用（scratch DB のみ）。コンソール error は favicon.ico 404 のみ（非関連）

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（`task_manager.py end` で実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
