# viewer: 検索実行中表示と FULL 画像 TN variant fallback

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: 完了として締める。実装・テスト・画面確認済み、commit `7edd5b2`

## 意図 / 背景

- ユーザー報告: viewer 検索が遅い、実行中か画面から分からない。
- ユーザー報告: viewer 閲覧の 2 ページ目以降で FULL 画像が `image load failed` になる。
- 正典 hitomi work page の TN rules は webpbigtn / avifbigtn だけでなく、list では smallsmalltn / smalltn、fallback に smallbigtn / smalltn / smallsmalltn を使う可能性があり、FULL proxy は variant fallback を実装する。

## 調査結果 / 変更内容

- 検索の体感遅延は UI が `/api/search` の完了を待つ点と、`v.search()` が `SearchHTML` の ids を取得した後 `searchResult(id)` を page 分の id に対して順次呼ぶ点から来ていた。これは gallery JSON fetch を結果分実行して card metadata を揃えてから返していた。
- 実装方針: `/api/search` は `ids` / `total` を返して initial response を速くし、UI は `search-status` と submit button で「実行中」を表示。card は `/api/search-card/{id}` で lazy load する。既存の search card cover 規則 (`r.thumbnail`) は helper に残す。
- FULL 画像 page 2 失敗の実測: gallery `2648601` page 2 hash `19793843306679ce3cab65db3ca0bf219477d0e8df09df23055f13ae957f5696` は `hasavif=1`。`pageSource` は base format の webp src を返し、`imageProxy` は storage full 失敗後に `webpbigtn` TN だけ試して 404 で 502 にしていた。strip の `avifsmalltn` は 200 で表示できており、TN variant 落ち込み不足が原因。
- 実装: `pageSource` は local を base format で見た後 remote source に `entryFormat(entry, base)` を使う。`imageProxy` は TN fallback order に `bigtn`, `smallbigtn`, `smalltn`, `smallsmalltn` を追加し、TN format が avif の場合 webp TN も最終 fallback として試す。テストと screen verify で 200 になることを確認する。

## Verification

- `python -m pytest tests/ -q` → `16 passed`（0.88s）
- `go vet ./...` → no output
- `go test -count=1 ./...` → `ok hitomi-hub/app 28.952s`
- UI lazy card selector root cause: `loadSearchCards` selector was `[data-search-id='\" + ids[i] + \"]`（closing single quote missing); fixed to `'[data-search-id='\" + ids[i] + \"']`, test asserts exact selector.
- Full image screen verify: `http://127.0.0.1:8731/#/g/2648601/page/2`で`pageSource`は`format=avif`を返し、`imageProxy`がTN variant fallbackで`/api/image/19793843306679ce3cab65db3ca0bf219477d0e8df09df23055f13ae957f5696?format=avif`を`200`で返した。`naturalWidth=2059`, `naturalHeight=2880`, `note=cdn/TN（未ダウンロード）`。スクショ: `scratch/hitomi-usage/full_fallback_page2.png`。
- Search status screen verify: initial status `検索実行中: furry`, submit disabled true, results placeholder `検索実行中…`; after lazy cards loaded status `検索完了・カード表示済み 25件`, cover count 25, submit restored to `検索`。スクショ: `scratch/hitomi-usage/search_status_executing.png`。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
