# galleryinfo disk 層の置き場を `cache_dir` 形へ統一（shape 決定）

**Date**: 2026-11-09
**Status**: 決定（ユーザー承認 2026-11-09 04:06 UTC「推奨を承認する」・「output 下が正規の状態になるようにすること」）
**タスク記録**: `docs/ai/tasks/2026-11-09-info-cache-shape-unified.md`

## 決定（正典）

- galleryinfo disk 層の置き場は `<root>/<cache_dir>/galleryinfo/<gallery id>.json`。正典既定も full cache と同じ形へ統一する（`defaultInfoCacheDirName` = `defaultCacheDirName` = `cache/images`）。
- `cache_dir` は cache 層全体の base。層ごとの subfolder は info 層 `galleryinfo`・TN 層 `thumbs` の名のみで、base を分けない。
- 正典コメントの「形が異なる点は未確定」を閉じる（`app/main.go:74-75`）。
- 実データ正規化: runtime root（`S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`）の `cache\galleryinfo` 429 file（9730497 byte）を `cache\images\galleryinfo` へ move（copy ではなく move）。git 管理外の data file。

## Supersede

- `docs/ai/tasks/2026-11-09-settings-value-centralization.md`（line 79）に残った設計点「`defaultInfoCacheDirName` の形未確定」を本決定で閉じる。
- `docs/ai/known-issues/KI-20261109-01-galleryinfo-shape-vs-cache-dir.md` は本決定で解消（実データ move 完了後 `known-issues/archive/` へ）。

## 影響（画面・挙動）

- 実データ側の孤立分 429 file（9.7 MB）が即利用される。remote 再取得（429 回・既定間隔 1 秒で約 7 分）は不要。
- `cache_dir` を変えると info 層も同じ base を追う。変更前は info 層だけ既定 `cache` に残っていた。
- 件数表示・間引き・手动 clear は同じ `infoCacheRoot` を見る（code path 不変）。

## 不変条件

- full image 取得順序（local full → persistent full cache → remote `StorageURL` → `no_full`）を変えない。
- TN を reader・thumbs の表示源にしない条、TN バイトを full cache に保存しない条を変えない。
- 完成フォルダ（`Galleries`・`acquisition`）は削除対象外のまま。永続先を `scratch/`・`output/` へ移さない条を変えない（本決定は runtime root 内で同一 data dir の subfolder 形を揃えるだけ）。
