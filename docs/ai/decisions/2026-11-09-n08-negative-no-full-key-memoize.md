# N-08: negativeNoFullKey の key 単位 memoization（指紋化ではなく memo 採用）

**Date**: 2026-11-09
**Status**: 決定・実装済み（assess-20261109-1320）

## 決定

- `negativeNoFullKey` の呼出ごと O(n log n)（CaseSet 約 2000 件ソート・約 10 KB 連結）に対し、**key 単位の memoization** を採用（ユーザー同意済み 2026-11-09）。fingerprint（SHA-256）案は採用しない — key の byte 列を変えずに ns 級改善を達成できるため。
- memo 構造: memo key = `hash|format|gg.Source|gg.Bucket` + CaseSet instance 識別（map pointer・件数）。value = 正典 builder が生成した key 文字列 + `lastAccess`（access 時刻記録）。**淘汰なし**（同意済み）。
- CaseSet の insertion 順正典意味（insertion 順・既定 3000・bump 優先・FIFO）とは**分離して維持**: memo は insertion 順の memo ではなく、builder 生成の byte 列を保持するだけの別構造。insertion 順正典テスト・FIFO・gate・singleflight・x-cache・TN 条件は非触。

## 根拠

- 意味等価性は構造保証: CaseSet は生成後に内容変更されない（書き込みは `parseGG` の new map 生成時だけ・snapshot `ggCaseSet` 静的）。memo には正典 builder の出力 byte 列そのものが入るため、memo 版＝現状（同一 case 集合→同一 key）は定義上成立。実測（400 random + 11 known・`hitomi_test.go` 2 テスト）で確認。
- 実測: 現行 161.4 µs/呼出（160.1/159.8）→ memo hit 110–120 ns/呼出（N=100,000・3 run・byte-identical 全 copy・SHA256 照合・`docs/ai/tmp/assess-20261109-1320/r08/`）。約 1,345–1,455x。既存正典記録（144–178 µs → ns 級）と整合。
- 正典検証: `go vet .` clean・`go test -count=1 -timeout 300s .` ok（27.294 s）・`python -m pytest tests/ -v` 64 passed・正典 build（fresh）合格。UI 項目でないため画面確認該当なし。

## 境界

- `ClearMemoryCaches`（手動 clear 正典）の scope は変更しない（memo は clear 列挙の対象外・記録のみ）。
- memo は entry 数無制限（淘汰不要の同意による）。C-05 の bounded（entry 数・stale-only）条件は `noFullCache` の条件であり、memo は非触・非衝突。

関連: `docs/ai/tasks/2026-11-08-n08-negative-no-full-key-cost.md`（実装記録）、`docs/ai/tasks/2026-11-03-c08-no-full-storm.md`（key 構成記載・byte 列不変のため更新不要）
