# 2026-11-01 Viewer double reader pairs already landscape entries

**Status**: resolved, implementation verified 2026-11-01

## 内容

Viewer reader で `displayMode()` / `readerClass()` が `double` を返すと、entry 自体がすでに横方向（見開き相当）であっても `readerItems()` が第二 item を pairing する。現状 `readerItems()` は entry の `width` / `height` を見て already-double aspect を判定していない。

## 実測

- URL: `http://127.0.0.1:8736/?x=cp14-strip-landscape-check#/g/4252697/page/1-2`
- `readerClass: "reader double"`
- `readerImageCount: 2`
- page1 entry hash: `899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9`
- page1 `width`: `3746`
- page1 `height`: `2525`

## 候補方針

- `entry.width > entry.height` の entry は already-double / single full-width 扱いする。
- `viewMode=double` は portrait page pairing の意味に限定する。
- square は strict に `width > height` で扱う方向を検討する。

## 関連

- Task: `docs/ai/tasks/2026-11-01-strip-group-and-landscape-double.md`
- Backlog: `P2 #14`

## 解決

`ui.html` で `isAlreadyDoublePage(entry)` を追加し、`entry.width > entry.height` の entry を already landscape/spread 扱いした。`readerItems()` はこの entry を second page と pairing せず、`readerClass(firstEntry)` が `reader single landscape` を返す。

Actual screen:
- `http://127.0.0.1:8736/?x=cp15-landscape-fixed#/g/4252697/page/1-2`
- `readerClass`: `reader single landscape`
- `readerImageCount`: `1`
- `readerOk`: `true`
- `prefetchStatus`: `full display cache 203/203 | no full 0/203 | TN display fallback disabled`

Double portrait regression:
- `http://127.0.0.1:8736/?x=cp15-results-search#/g/4252697/page/2-3`
- `readerClass`: `reader double`
- `readerImageCount`: `2`
- pages `2`, `3` remain paired because both are portrait.
