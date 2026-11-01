# 2026-11-01 Viewer strip group/page selection UI missing

**Status**: resolved, implementation verified 2026-11-01

## 内容

Viewer thumbnail strip は `ui.html` の `stripGroupPage(items)` で current page から group を選択し、`renderStrip()` がその current group の 50 件だけ render する。任意 group/page を操作する UI control が存在しない。

## 実測

- URL: `http://127.0.0.1:8736/?x=cp14-strip-landscape-check#/g/4252697/page/1-2`
- route: `#/g/4252697/page/1-2`
- `thumbsControlCount: 0`
- `thumbsRenderedCount: 50`

## 関連

- Task: `docs/ai/tasks/2026-11-01-strip-group-and-landscape-double.md`
- Backlog: `P2 #13`

## 解決

`ui.html` に strip group controls を追加し、`stripGroup` と `stripGroupPage()` で任意 group を選択可能にした。

Actual screen operations:
- `#strip-next-group` click: `stripGroup` became `2`, `strip group 2/5`, pages `51` to `100`.
- `#strip-group` input `5`, `#strip-go-group` click: `stripGroup` became `5`, `strip group 5/5`, pages `201` to `203`.
- `#strip-controls` controls count: `4`.
