# R-19（未検証）: scene_indexes のある gallery でページ番号の基準が食い違う可能性

**Status**: 有効（実測確認 2026-11-09・修正案は承認待ち。読み取りのみ→実測済み）
**Date**: 2026-11-06
**Source**: 外部指摘（Claude）2026-11-06 項目 17 → `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md`

## 読み取りで確認できた機構

- strip は `scene_indexes` の先頭から末尾-1 までを `sceneIndex + 1` を page として items にする（`app/viewer.html:444-453`）。
- `current.count` は `sceneIndexes.length ? sceneIndexes.length - 1 : g.files.length`（`app/viewer.html:256-263`）。
- `movePage` の上限と pagenum input の上限は `current.count`（`app/viewer.html:527-537`、`1426-1429`）。
- `badge` は `sceneIndexes[idx+1] - sceneIndex`（scene ごとの枚数）なので、scene 内の枚数分だけファイル数が scene 数より多くなり得る。

## 影響（未確認）

ファイル数 > scene 数（scene 境界間の枚数）の場合、`current.count`（scene 数基準）が最終ファイルの page 番号より小さく、`次` で末尾のファイルへ到達できない可能性がある。実 gallery での再現は未確認。

## 次の対応（未実施）

- 実 gallery fixture または live galleryinfo で「scene 数 < ファイル数」の実例を確認してから判定。実例があれば page 基準（scene 基準か files 基準か）の設計判断を要る。読み取りのみで実装はしていない。

## 2026-11-09 再調査（assess-20261109-1320）

**有効性**: 有効（実測で閉じた。判断保留ではない）

### 現行コードで確定した混在経路（現行行番号）

- `current.count` = `sceneIndexes.length ? sceneIndexes.length - 1 : g.files.length`（`app/viewer.html:258-263`。ヘッダの「N files」表示にも same value、`viewer.html:269`）
- `stripItems()` scene 基準: `page = sceneIndex + 1`（files index）・`entry = files[sceneIndex]`・`badge = next scene - scene`（`app/viewer.html:459-468`。2026-11-06 の行番号引用 444-453 は現行では 459-468 に移動）
- `entryForPage(page) = files[page-1]` files 基準（`app/viewer.html:496-498`）
- `movePage` 上下限 = `current.count`（`app/viewer.html:542-557`）→ 加算は連続 page（files 基準的な動作）
- `readerItems()` の second: strip 基準 item（`items[firstIdx+1]`）と files 基準 `entryForPage(first.page+1)` が同一関数内で混在（`app/viewer.html:524-540`、second 取得 531-535）
- pagenum input guard `page <= current.count`（`app/viewer.html:1441-1444`）
- 読み完了判定 `lastPage = current.count || files.length`（`app/viewer.html:1136-1140`）→ `/api/read` に scene 基準 page が記録される（DB の last_page 基準が gallery により混在）

1:1（scene_indexes = files index の連続 + sentinel）なら count = files 数となり全経路一致。scene が複数ファイルを含む（sparse）ときだけ矛盾が顕在化する。

### 外部仕様（hitomi 正典）確認 — 1:1 は保証されない

- live `gallery.js`（2026-11-09 実測・`docs/ai/tmp/assess-20261109-1320/gallery.js`）: `length = scene_indexes.length-1`（scene あり）または `files.length`。thumbnail は scene ごと 1 枚、`pagenum = scene_indexes[index] + 1`（**files index**）、`badge_number = scene_indexes[index+1] - scene_indexes[index]`（badge>1 の scene を明示的に扱う）→ scene 数 < files 数は仕様の想定内。1:1 保証という外部仕様は成立しない（「混在経路あり・実害なし」判定は不可）。
- hitomi の page 番号は scene start + 1（= files index）。`scene_indexes` の末尾要素は sentinel（= files 数）。実例 733697 で `last == files` を実測（3 サンプル中 sentinel 確認 1、sparse 実例 2: 733697(210 files/13 scenes)・test fixture 形 [0,13,15]）。`reader.js` は gamecg で prevScene/nextScene（scene 単位遷移）を使用。
- hitomi-hub の strip（`sceneIndex + 1`）と `entryForPage` は hitomi 正典の page 番号（scene start + 1）と一致。矛盾するのは `current.count`（scene **ordinal** 数）を page 上限として連続加算（movePage・pagenum・position 分母）に使う点だけ。

### 実測（fixture 再現 + served 画面）

- fixture 再現（`docs/ai/tmp/assess-20261109-1320/repro_js_logic.py`・results json）: 733697 形（files 210・scene 13）で count=12、movePage 到達最大 page=12、strip page は 1,2,32,…,187、最終ファイル #210 到達 false、page 3 で表示 file=files[2]（scene start ではない）。1:1+sentinel fixture は矛盾なし、scene indexes が files 数を超える fixture（4294614 test 形）では `entryForPage` が undefined。
- served 画面確認（正典ルール準拠: fresh canonical `hub.exe`・fresh server instance・isolated origin・window size 1666x932・before/after 2組）。gallery 733697（live・gamecg）で: 位置「1 / 12」「12 / 12」（実ファイル 210）、ヘッダ「12 files」表示、上限で「次」false（最終ファイル #210 到達不可・hash 一致で確認）、pagenum 入力 210 拒否（page 12 のまま）、strip page 32 クリックで位置「32 / 12」（表示上の基準不一致が画面に現れる）・page 32 から「次」false（33 > count）。保存先: `I:\マイドライブ\kilo\2026-11-09-KI-20261106-01-scene-page-count-basis\`（before/after png 4 枚 + confirmation text。local copy はゴミ箱へ送った）。server は実 DB 非変更のため data-dir/root を scratch に変更（正典 command からの逸脱・記録）。
- 検証: `go vet .` 合格・`go test -count=1 -timeout 300s .` ok（27.030s）・freshness test 2 passed。作業前チェックの decision 3 本読了（`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`）。

### 推奨修正（設計判断・実装前）

- **推奨 A（files 基準の完全統一）**: `current.count = g.files.length`（scene いても）、movePage/pagenum/position 分母/読み完了を files 基準へ統一。strip の scene badge 表示は維持。理由: app は全ファイルを下載・表示するローカル機能（完成フォルダに scene 内ファイルも存在）で、scene 内ファイルを表示不能にする hitomi の scene-page 模型より app の reader model（1 entry = 1 page）に一致。混在経路（readerItems second・entryForPage）がすべて正しくなる。
- 代替案 B（hitomi 正典準拠の scene 基準統一）: movePage を strip item 境界（scene start + 1）で遷移、count は scene ordinal のまま page 番号を scene start 基準で表示。hitomi と一致するが scene 内ファイルが viewer で表示・既読判定不能（app 的功能と衝突）。
- 副作用・影響範囲: `read_state` の last_page/complete 基準変化（既存 DB の sparse gallery 行は混在基準で記録済・移行判断要）、`renderPosition`/ヘッダ表示の user-visible 意味変更（「12 files」→「210 files」等）、テスト re-anchor（`app/viewer_test.go` の `sceneIndex + 1`・`current.count` 文字列アサート）、prefetch 件数増（推奨 A は items=files 全部。733697 で 12→210）。実装規模: 小（viewer.html 数関数 + tests）。
- 基準選定は design decision（page 基準の正典化）を伴う。推奨 A の採用は承認後（decisions/ に記録し、supersede 対象があれば明示）。

### 次の対応

- 承認後に推奨 A 実装（実装開始指示待ち）。実例は 733697 で確定済み（fixture 追加不要。test 化は `TestGalleryParse` 実例 [0,13,15] + 733697 形 fixture で可能）。
