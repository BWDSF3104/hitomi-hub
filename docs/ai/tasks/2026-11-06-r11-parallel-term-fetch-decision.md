# R-11: 複数 term の取得が逐次（gate は間隔のみ直列化）

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 設計判断確定（逐次維持）。実装項目ではない。体感速度は R-10 で回収。記録: R-11

## 意図 / 背景

- 外部指摘（2026-11-06 項目 11）: 複数検索語の取得が逐次で、体感で数十秒級。並行化の可否を判断する項目。
- 判断の前に「語ごとに分けて 1 語ずつ取得するのが元サイトと違うのか」を確認する。

## 調査結果 / 変更内容

- 語ごとに分けて 1 語ずつ取得するのは正典準拠。元サイトは 1 リクエストで複数語をまとめて投げる做法ではなく、語ごとの data を取って共通部分を作る（`app/searchhtml.go` の `searchAllIDs` が正典 `results.js` / `search.js` の `get_galleryids_for_query` の集合演算を mirror。`app/hitomi.go:835` に同じ分区規則の記録）。
- 補足（2026-11-06 質問）: 語ごとの分割取得は AND 検索そのもの（共通部分）。`or` は group 内 union、`-term` は exclusion。R-11 の指摘は集合の意味ではなく、取得の逐次・同時の話。
- したがって、集合（出る結果）は元サイトと同じ。並行化は結果を変えず、タイミングだけ変わる。
- ただし同一性が壊れる条件がある:
  1. 順序の起点を変える（最初の positive term の dataset、または plain term で `orderbykey != 'added'` のとき order dataset を使う分岐・D-09）
  2. `language:` term の pre-scan（C-04）の扱いを変える
  3. cache key（`area|tag|language`、order を含む）を変える
- これを満たせば並行化は結果同一。満たさないと順序・集合が変わる。
- 正典 JS で確認したのは集合演算の単位（語ごと）まで。ブラウザ側の取得の並行度（逐次か同時か）は未観測。
- 決定（2026-11-06）: 逐次のまま。politeness gate（1000ms・FIFO・grace）の意味を変えない。体感速度は R-10（同じ検索条件の最終結果を短 TTL で覚えて、page 移動・prefetch・firstStatus で共有）で回収する。
- 速度の実感（2026-11-06・機構から。実測なし）: cold で語数 N の AND 検索は語ごとに gate 通過 + network を重ねる。data lane の正典値は 100 ms（`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md`）。3 語なら gate 0.3 s + network 3 回、5 語なら gate 0.5 s + network 5 回。warm（`searchCache` は 1 時間 TTL・`app/hitomi.go:60`）は network 0 回だが呼出每次に共通部分の map を作り直す（R-10）。
- 実測は P3 #10（`docs/ai/backlog-items/p3-10-search-speed-measurement.md`・採用済み）で gate wait と network を分離して行う。実測前に並行化は採用しない。
- 決定ファイル: `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`

## Verification

- 読み取りのみ。コード・UI・schema は変更していない。テスト実行・live 呼出しなし。
- 同一性の条件（1〜3）は `app/searchhtml.go` の現実装（D-09 mirror・C-04 mirror・cache key）から確認。ブラウザ側 fetch の並行度は未確認として記録。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（`task_manager end` で実施）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし）
- [ ] `backlog.md` の完了項目を削除（該当なし）
