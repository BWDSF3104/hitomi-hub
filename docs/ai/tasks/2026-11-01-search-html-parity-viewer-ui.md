# search.html 完全一致検索・Viewer UI 強化

**Status**: 完了
**Date**: 2026-11-01
**Next Action**: docs/ai/usage-memo.md と完成フォルダコピー更新、実測確認

## 意図 / 背景

Hitomi の `search.html` は単一 word term を substring 検索ではなく、`galleriesindex` の exact term key lookup で扱う。ユーザーの `furry` は token `furry` として index を降りる必要があり、`furr` や `fur` と混同してはいけない。同時に 1 ページ Viewer を単なる画像表示から、検索結果一覧・cover/mini page thumbnails・local proxy・read state 更新・hash route を含む実用 UI に寄せる。

## 調査結果 / 変更内容

- Hitomi live `search.html` で current `furry` total `32832`、`furr` total `32837`、`fur` total `47007`、`furryx` total `0` を確認。
- `search.html` 実測 `furry` first ids: `4295047, 4294784, 4294785, 4294786, 4294788, 4294536, 4294426, 4294423, 4294422`。
- `hash_term('furry')` live key bytes: `a1d72c62`; Go `hashTerm("furry")` same。
- Live index data order from browser `get_galleryids_for_query('furry')` matched rebuilt binary. The earlier `4325205` top IDs were stale page cache.
- `SearchHTML` は plain term（single word）で `IndexIDs("galleries", term)` exact hashTerm を使う。colon term は nozomi、`language:` は state の lang を更新し、後続 nozomi path に反映。`language:` 単独は `/n/index-<lang>.nozomi`。
- Viewer: `/api/search` は query を plain exact term、colon terms、negative terms、order/pagination として `SearchHTML` へ渡す。`lang=all` default。
- Viewer proxy: same-origin `/api/thumb/<hash>?format=webp` と `/api/image/<hash>?format=webp` を追加。Browser ORB で direct CDN image fetch を避けるため UI は local proxy path を使う。
- `ThumbnailURL` は live Hitomi `real_full_path_from_hash` と一致: `<last char>/<prev two chars>/<hash>.<ext>`。例 `2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89` → `webpbigtn/9/b8/...`。
- `validHash` は lowercase hex length `>=3` を受け入れる。Live result hashes は 64 に限らない。
- `ggset.go` を current `gg.js` 実測から生成。`gg.m(3)=1`, `gg.m(3628)=1`, `gg.m(2488)=1`, `gg.m(1)=0`。Current `gg.b = 1793476801/`。`ImageURL` は `w2` を返す。
- `TestImageURLMatchesLiveSample` は `w2` path と content-type を確認。

## Verification

- 2026-11-01 23:06: Go toolchain を winget でインストール: `GoLang.Go 1.27.0`, path `C:\Program Files\Go\bin\go.exe`。`app/go.mod` の `go 1.27.3` toolchain requirement で実測 `go1.27.3 windows/amd64`。User PATH に `C:\Users\fujim\go\bin` が追加されたが、現在の Kilo tool process PATH refresh は未反映。
- `go vet ./...` from `app` module: pass。
- `go test -count=1 ./...` from `app` module after current generator: pass。Test names:
  - `TestImageURLMatchesLiveSample`
  - `TestSearchHTMLPlainTermUsesIndexExactTerm`
  - `TestSearchHTMLColonTermUsesNozomiState`
  - `TestSearchHTMLLanguageOnlyUsesNozomiIndex`
  - `TestSearchHTMLNegativeTermExcludes`
  - `TestThumbnailURLMatchesTNBigtnPath`
  - `TestViewerImageProxyServesSameOriginWebp`
  - `TestViewerThumbProxyUsesRealTNPathAndContent`
  - `TestEmbeddedUIUsesLocalThumbProxy`
  - `TestSearchResultThumbnailFormatUsesImageFormat`
  - `TestEmbeddedUIThumbUsesServerFormat`
- 2026-11-01 22:58: completion binary rebuild pass from `app` using absolute scratch Go path and `CGO_ENABLED=0`:
  - `go build -trimpath -ldflags "-s -w" -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
  - viewer restart/new process pid `19028`; `/api/search?query=furry&lang=all&page=1&limit=3` returns live current total `32833`, first thumbnail path with `format=webp`, thumbnail proxy `200 image/webp`.
- CLI parity from rebuilt `hub.exe` with DB `scratch\hitomi-usage\hitomi.db`:
  - `search --term "furry" --lang all --page 1`: total `32832`, first ids `4295047, 4294784, 4294785, 4294786, 4294788, ...`
  - `search --term "furr" --lang all --page 1`: total `32837`, first ids `4295047, 4294784, 4294788, 4294786, 4294785, ...`
  - `search --term "fur" --lang all --page 1`: total `47007`, first ids `4295047, 4294785, 4294784, 4294788, 4294786, ...`
  - `search --term "furryx" --lang all --page 1`: total `0`, `count 0`
- Browser live:
  - `http://127.0.0.1:8731` shows `search.html` parity UI cards, cover thumbnails, mini page thumbnails, `note`, `pagenum`, strip.
  - `/api/image/098...?format=webp` returns `200` and `WEBP` content type; reader full image loads width `1997`.
  - Screenshot `scratch\hitomi-usage\viewer-new-ui-loaded.png` shows left result cards and right reader image/control bar.

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（該当行なし）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
