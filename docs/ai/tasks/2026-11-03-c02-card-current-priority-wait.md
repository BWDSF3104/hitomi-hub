# C-02: カードごとの currentPriorityDone 待ち

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 完了

## 意図 / 背景

- `loadSearchCardCover` が card ごとに `await currentPriorityDone` を待つ。
- 問題は card fetch 直列化と current priority wait が混ざること。A-05/C-01 と同一項目として、bounded parallel worker で card fetch を分離し、cover load の priority wait は current priority の完了後だけ行う。

## 調査結果 / 変更内容

実装:
- card fetch を `loadSearchCards` の bounded worker pool で行う。
- `loadSearchCardCover` 内の `await currentPriorityDone` を維持し、searchGeneration guard を維持する。
- worker から cover load を呼ぶが、card fetch 自体は current priority wait で直列化しない。
- stale search の DOM 書き込みを `searchGeneration !== searchGeneration` で停止する。
- TN display source は full display source と分離したまま維持し、TN failure を full actual pixels として数えない。

## Verification

- `go vet .` in `app`: clean。
- `go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 36.599s`。A-05/C-01 と同一 commit 項目で検証済み。
- `python -m pytest tests/ -v`: 25 passed, including canonical build freshness。
- Canonical UI confirmation from rebuilt canonical server, `http://127.0.0.1:8731/`, fresh tab, viewport `1280x900`, actual search click `female:yuri` / `english` / page `1`:
  - UI status reached `検索完了・カード表示済み 25件`。
  - First card DOM `data-search-id='4300644'` matched network request `#90` for `/api/search-card/4300644` (`x-cache: cache`)。
  - First TN cover network request `#54` returned `200`, `x-cache: tn-cache`。
  - `statusIsViewerDisplaySource("tn")` returned `false`, so TN display fallback remained disabled。
  - Screenshot same as A-05/C-01 item: `I:\マイドライブ\kilo\2026-11-03-claude-review-349778d-assessment\a05-parallel-search-cards.png`。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
