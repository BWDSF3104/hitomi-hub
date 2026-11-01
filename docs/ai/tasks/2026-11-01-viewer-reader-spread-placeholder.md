# Viewer hitomi-like reader spread fullscreen placeholder

**Status**: 完了
**Date**: 2026-11-01
**Backlog**: P2 reader UI
**Next Action**: screen verify and commit implementation

## 意図 / 背景

- Hitomi reader の `https://hitomi.la/reader/2648601.html#2-3` のように、viewer reader で通常ページサイズ時に見開き表示を再現する。
- Full screen / single view modes と keyboard navigation を再現する。
- Full image load の応答性のため、reader item に placeholder thumbnail（hitomi-like strip TN variant）を先に表示し、full image load 完了後に同じ `<img>` を swap する。

## 調査結果 / 変更内容

- `parseReaderHash(hash)` を追加。`#/g/<id>/page/2-3`, `#/g/<id>/2-3`, `#2-3`（current id がある場合）を pair hash として扱う。
- `displayMode()` は default `double`。full screen mode、narrow window fallback single。
- `readerItems()` は `stripItems()` の scene items を base に current item と second item を返す。`#2-3` の second hash が明示指定されている場合は `currentPairEnd` を優先する。
- `route(id, page, second)` と `setRoute()` で hitomi reader-like `#2-3` style route を生成する。
- `loadFullImage(img, entry)` は placeholder src を保持しながら、full image onload 後に同じ img element に full src を swap。placeholder uses `entryPlaceholder(entry)`, full uses `entrySource(entry)`。
- Cost: placeholder は displayed items ごとに small TN 1 request。すでに strip/background prefetch で cache に存在する full image に対しては通常 network cost は追加。displayed item だけの placeholder に限定することで background full prefetch 全体よりコスト低。

## Verification

- `go test -count=1 ./...` → `hitomi-hub/app 30.407s`, passed。Added UI tests for view-mode, displayMode, readerItems, parseReaderHash, route second, fullscreenchange, keyboard `f`, requestFullscreen, narrow fallback, placeholder swap, renderReader nonblocking load.
- `go build -trimpath -ldflags "-s -w" -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → passed。
- Screen verify `http://127.0.0.1:8731/?x=1#/g/2648601/page/2-3`: `reader` class `reader double`, displayed pages `2,3`, pagenum `2`, full images load, `naturalWidth=2059`. Screenshot `scratch\hitomi-usage\reader_double_placeholder.png`.
- Fullscreen view mode verify: `single` -> `reader single`; `fullscreen` -> `reader fullscreen` and `document.body.classList.contains('fullscreen') == true`.
- Commit: `55bb9c7` `feat: reader で見開き/fullscreen placeholder と hitomi-like #2-3 hash を実装`。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
