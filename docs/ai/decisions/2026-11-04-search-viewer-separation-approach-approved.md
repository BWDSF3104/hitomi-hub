# 検索と閲覧の完全分離：方式と判断保留4点の承認（assess-20261104-0749）

**Status**: 承認済み（ユーザー 2026-11-04 08:22）
**Date**: 2026-11-04
**実行固有ID**: `assess-20261104-0749`
**分離前チェックポイント**: tag `search-viewer-separation-checkpoint-2026-11-04` → `b1ef252`

## Supersede 対象

`docs/ai/decisions/2026-11-01-viewer-single-page-web.md` の「閲覧層は 1 ページ Web」の点のみを supersede する（ページを2本へ分割するため）。同じファイルの次の点は維持: native GUI 不採用、`embed` により単体 exe へ同梱（完成フォルダに追加ファイルを増やさない）、既定 bind `127.0.0.1`、local-first 画像経路、SQLite 単一接続。フル画像・TN display policy（`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`）は変更しない。

## 決定（承認された推奨）

1. 一覧（保存検索・お気に入り・未読）の所有ページは検索ページのみ。閲覧ページは `showLists` を呼ばない。ページ間同期機構（BroadcastChannel）は第1版に入れない。正典は DB（API）で単一、表示更新は手動で許容。
2. 小道具（`escapeHTML`、読み込み helper、世代番号 guard、TN URL 組み立て）はページごとに複製し、各ページを自己完結させる。共有ファイルを `embed` に追加する案は採用しない（共有があると領域が構造的に混ざりにくくならない、テスト参照先が2本必要になる）。
3. 起動ページは `GET /` = 閲覧、`GET /search` = 検索。ページ間の hash namespace は単独（viewer `#/g/...`、search `#/q/...`）で、誤着用回送分岐は入れない。検索 URL は `/search#/q/...` を使う。既存の閲覧 URL `/#/g/...` と画面確認手順はそのまま。既定 landing を検索にする案は採用しない。
4. `tests/test_canonical_build_freshness.py` の照合対象を `app/viewer.html` と `app/search.html` の2本へ変更（bytes 一致の目的・動作保証は同じで、照合対象だけ増える）。失敗メッセージには両ファイル名を明記。画面確認の基準は閲覧ページ、検索ページは served 画面での確認を追加（画面確認の定義を弱めない）。
5. 実装形態: `app/viewer.html` に閲覧系（reader・strip・nav・acquisition）、`app/search.html` に検索系（form・card list・TN cover・閲覧ページへのリンク）。`//go:embed` を multi-file 化し `GET /search` ルートを追加。objectURL cache はドキュメントごとに閉じる（共有しない）。検索側は pure TN cover（full objectURL 優先・`currentPriorityDone` 待ちを削除）。

## 未実施・個別承認が必要

- `AGENTS.md` の Canonical Latest Build / UI Confirmation Test Rule 条の文章変更（2本照合へ）は write-protected 領域。方向は本決定で承認済みだが、ファイル編集そのものは個別の明示承認が必要（未実施）。
- 実装開始指示は未受領。本記録は承認の記録のみで、ソースコードは無変更。

## 影響（テスト参照先の振り分け）

- `app/viewer_test.go`: `uiPage.ReadFile("ui.html")` 37 assertion を viewer / search へ振り分け（15〜20 件 re-anchor、7〜9 件 rewrite）。
- `tests/strip_rebuild_dom_harness.mjs`: viewer.html へ。`openGallery` 終端 anchor は `showLists` を参照しなくなったため、`renderStrip` 終端へ変更。
- `tests/read_lists_debounce_harness.mjs`: `showLists` block 抽出 → search.html へ。
