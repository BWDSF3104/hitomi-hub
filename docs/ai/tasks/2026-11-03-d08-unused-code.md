# D-08: 未使用コード

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

確認済み（production 未使用）: `fileExt`（`app/hitomi.go:322`、参照は `searchhtml_test.go` のみ）、`imageExts`（`app/scan.go:17`、参照なし）、`Limits.MaxRequestsPerMinute`（parse のみ、gate は per-kind interval を使用）、`Search`/`SearchQuery`（`SearchQuery` の参照はテストのみ、production は `SearchHTML`/`IndexIDs`/nozomi path を使用）、`nozomiURL`（`Search` 経由のみ）。誤り: `pageSource` は未使用ではない（route `app/viewer.go:69` + handler `app/viewer.go:573`）。追加: `app/ui.html:1458` の `current.favorite = current.favorite;` は no-op。対応: 削除 or 使用箇所の明示（削除は実装段階、テスト参照の関数は移動）。推定規模: trivial。

実施前再確認（2026-11-04、行番号漂移あり・内容一致）: `fileExt` 現 `app/hitomi.go:508`（参照 `searchhtml_test.go:632-633` のみ）、`imageExts` 現 `app/scan.go:18`（参照なし）、`MaxRequestsPerMinute` 現 `app/main.go:36`（raw `113`・parse block `144-146`。raw タグ `json:"-"` で実質 dead parse）、`Search` 現 `app/hitomi.go:663`（production 呼び出しは `SearchQuery` 内のみ・`SearchQuery` 参照は `hitomi_test.go` のみ）、`nozomiURL` 現 `654`。`pageSource` route `viewer.go:119` + handler `673`（使用中・変更なし）。ui.html no-op 現 `1616`。

変更内容（承認済み方針 2026-11-03-remaining-item-recommendations-approved に従う）:
- 削除: `fileExt`（削除でなく移動へ）、`imageExts`（scan.go）、`Limits.MaxRequestsPerMinute`（struct フィールド・raw フィールド・parse block、code 側のみ。config.example.json キーは承認範囲外のため未変更）、`nozomiURL`（production 未使用）
- 移動（テスト参照関数）: `Search`/`SearchQuery` → `app/hitomi_test.go`。`Search` は nozomi URL 構築テストに必要なためテスト内自己完結形（`nozomiPathParts`/`nozomiPath` を直接使用）。`fileExt` → `app/searchhtml_test.go`（参照テスト所在）
- `hitomi.go` から `encoding/binary` import 削除（残存使用なし）、`hitomi_test.go` に `encoding/binary` import 追加
- ui.html no-op 1 行削除（functional change なし）
- 記録外副作用（未変更・報告）: `nozomiURL` 削除により `nozomiPathParts`（`hitomi.go:623`、D-08 対象外）は production 呼び出し元がなくなり移動後テストからの参照のみ。`MaxDailyRequests`（同型だが未使用判定なし・D-08 対象外）未変更

## Verification

- app `go vet . && go test -count=1 -timeout 300s .`（2026-11-04）: vet PASS、suite 17.060s で `TestSearchCacheSizeCapBoundsMemory`（searchhtml_test.go:173）1 回 FAIL（既知 flaky LRU tie・known-issue 2026-11-04・本項目非触）。単独実行 `-run TestSearchCacheSizeCapBoundsMemory` PASS 0.158s、suite 再実行 PASS 17.198s
- canonical exe 再 build: app `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（成功）
- ルート `python -m pytest tests/ -v`: 40 passed 7.96s（`test_canonical_executable_embeds_current_ui` = ui.html bytes 照合 PASS。ui.html は no-op 1 行削除で functional change なし。実機 UI 操作は該当なし＝未検証点として記録）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
