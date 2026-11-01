# Implement P3 adopted plans from lowest difficulty

**Status**: 進行中
**Date**: 2026-11-02
**Next Action**: Set tests for P3 adopted plans and implement from #16 #11 #13 #15 #18 #14 #10 #17 #12 with screen verify and checkpoint commits

## 意図 / 背景

P3 backlog の採用済み方針を実装する。難易度の低い項目から順に、項目別に tests を設定し、既存機能影響があれば既存機能確認 tests も追加する。実装ごとに画面確認を行い、合格まで修正を繰り返す。想定通り完了した checkpoint ごとに記録と commit を行う。

## 調査結果 / 変更内容

Planned order by difficulty:

1. `#16` tab title to current work
2. `#11` gallery/viewer position display only
3. `#13` search result prev/next page buttons
4. `#15` view mode buttons and state row
5. `#18` default RTL spread display order and toggle button
6. `#14` fullscreen state separation and resize/error handling
7. `#10` search latency measurement
8. `#17` same-title Japanese anchor on display cards
9. `#12` normalized cache acquisition by copy, page-number filenames, overwrite confirmation

## Checkpoint #16 tab title

Implemented in `app/ui.html`:

- `updateTabTitle(title)`
- called from `setGalleryMeta()`, `showGalleryMetaFromCache()`, `renderGalleryLoading()`
- sets `document.title` to `japanese_title || title || id`, with suffix ` - hitomi hub viewer`

Test added in `app/viewer_test.go`:

- `TestEmbeddedUIUpdatesTabTitleForCurrentWork`

Verification:

- `go test .`: pass
- `go vet .`: pass
- Screen verify:
  - URL: `http://127.0.0.1:8731/?x=tab#/g/4295693/page/1`
  - `document.title`: `ぱんぱんジャンヌちゃん - hitomi hub viewer`
  - Screenshot: `scratch/hitomi-usage/p3-16-tab-title-4295693.png`
  - Console: only image cache/no_full 404s on the open gallery; title behavior passed.

Canonical exe was rebuilt and canonical server restarted with the same test build.

## Verification

No verification yet.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
