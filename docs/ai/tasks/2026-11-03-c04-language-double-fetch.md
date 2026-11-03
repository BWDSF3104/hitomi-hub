# C-04: language: 併用で同じ tag を 2 回取得

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

`language:` term が前置 term で fetch 済みの同じ area/tag を新 language で再 fetch する重複（nozomi calls 1 → 3）を排除し、mocked fetch で同一 tag の language 違い fetch を 1 回に固定する。

## 調査結果 / 変更内容

有効（静的＋実測補助）。`app/searchhtml.go:115-121`（`termIDs` の `language` 分岐）は `state.language` を set した後、同じ `state.area`/`state.tag` で `SearchNozomiParts` を呼ぶ → 前置 term 已 fetch した同一 tag を新 language で再 fetch。実測: `female:yuri language:english`（lang=all, limit=1）で nozomi calls が 1 → 3（client 累積メトリクス）。対応: `language:` term だけ language を更新し fetch しない（`state.language` 更新後に最初の term fetch へ反映）、または update 後に 1 回だけ再 fetch。合格条件: mocked fetch で同一 tag の language 違い fetch が 1 回。推定規模: 小。

実施（設計確定: `docs/ai/decisions/2026-11-03-c04-language-term-single-fetch.md`）:

- `app/searchhtml.go`:
  - `isLanguageTerm` helper 追加。
  - `SearchHTML`: fetch 前に positive term の language term を pre-scan し `state.language` を最初の term fetch へ反映（first-wins）。positive[1:] の language term は dataset redundant（実 data の subset 保証で intersection no-op）なので fetch しない。
  - `termIDs` の `language` 分岐: 同じ area/tag の再 fetch をやめ、自前 dataset `index-<lang>`（既存 language-only 規則と同一 key）のみ fetch。negative・or-group の language term は dataset 寄与が必要なので termIDs 経由を維持。
  - `SearchNozomiParts`（fetch semantics・TTL/LRU/size cap・singleflight・negative cache）は無変更。UI 変更なし。
- `app/searchhtml_test.go`:
  - fixed test `TestSearchHTMLColonTermUsesNozomiState` を更新: `female:yuri language:english` で fetch 1 回（`female:yuri-english` のみ）、同一 tag の language 違い fetch なし。fixture は subset 規則（実 hitomi data）に統一、期待結果 {20,30} は維持。
  - 回帰テスト 4 件追加: `TestSearchHTMLLanguageTermIntersectionRegression`（intersection・fetch 2 回・-all なし）、`TestSearchHTMLNegativeTermExcludesWithLanguageRegression`（exclusion・language 反映）、`TestSearchHTMLNegativeLanguageTermExcludesItsOwnLanguageDataset`（negative language term は own index dataset）、`TestSearchHTMLOrGroupLanguageTermContributesOwnLanguageDataset`（or-group language term は own index dataset・同一 tag 重複なし）。
  - 既存 tests（language-only、plain term、negative、or-union、cache TTL/LRU/size cap、order、TN）は無変更・回帰維持。
- 未判断（テストなし・pathological）: positive に複数の異なる language term → first-wins（旧実装は実 data で ∅）。`language:` 右辺空 → pre-scan ignore、term は dataset を寄与しない。

## Verification

- `app` で `go vet .` → clean。
- `app` で `go test -count=1 -timeout 300s .` → ok 66.941s（fixed test・回帰 4 件含む。UI 変更なしのため Playwright 実機確認は該当なし）。
- canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（app）→ 成功。`tests/test_canonical_build_freshness.py::test_canonical_executable_embeds_current_ui` PASSED。
- project root `python -m pytest tests/ -v` → 38 passed 3.91s（freshness test 含む）。
- 正典 tool 変更なし → smoke test 不要。正典 Common_Operations `python -m pytest tests/ -q` → 120 passed 6.11s（task_manager validation 破れなし確認）。
- 実機 download/viewer 確認はこの項目の合格条件外（search 経路の fetch 回数の固定は mock test で固定済み）。remote 実測（nozomi calls 1→3 の是正 1→1）は未実施 → 未確認として記録。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
