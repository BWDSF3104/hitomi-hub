# galleryinfo disk 層の置き場を cache_dir 形へ統一（正典既定統一・実データ正規化）

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: decision 記録 → app/main.go の既定値統一 → test → canonical build → 実データ移行 → served 確認

## 意図 / 背景

- `docs/ai/decisions/2026-11-09-settings-value-centralization.md` 系で残った設計点（`defaultCacheDirName = "cache/images"` と `defaultInfoCacheDirName = "cache"` の形差）を確定する。ユーザーが推奨案（galleryinfo を `cache_dir` 形へ統一）を承認（2026-11-09 04:06 UTC）。
- 承認内容: galleryinfo disk 層の置き場を `cache_dir` 形へ統一し、正典既定も同じ形にする。実データ（`output\hitomi-hub`）の孤立分 429 file（9.7 MB）を正位置へ正規化する。
- 実データ正規化が本項目の目的の一部。単体静的バイナリ・移植性条件（書き込みを完成フォルダ内に閉じる）は変更しない。
- 作業前チェック（フル画像・viewer に触るタスク）で読む正典: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。本項目は viewer の info cache 置き場解像だけを変え、full cache 順序・TN 表示源・永続先を変えない。

## 調査結果 / 変更内容

- decision: `docs/ai/decisions/2026-11-09-info-cache-shape-unified-with-cache-dir.md`（galleryinfo の base を `cache_dir` 形へ統一。`defaultInfoCacheDirName` = `defaultCacheDirName`）。`docs/ai/tasks/2026-11-09-settings-value-centralization.md` line 79 の残る設計点を閉じる。
- `app/main.go:74-82`: `defaultInfoCacheDirName` を `defaultCacheDirName`（`cache/images`）へ統一。正典コメントの「形未確定」を閉じた。accessor `InfoCacheDirName()`（`:87`）は同一 config key `cache_dir` を見る形のまま（挙動は既定値のみ変更）。
- `app/viewer.go:40-42`: 置き場コメントを `<cache_dir>/galleryinfo/<id>.json`（既定 `cache/images`）へ更新。`infoCacheRoot` 生成（`app/viewer.go:140`）は変更なし。
- `app/main_test.go`: `TestInfoCacheDirNameSharesCacheDirBase` 追加（既定で両層が `cache/images`・明示値で両層が同じ folder を追う）。
- 実データ正規化（ユーザー指示）: `output\hitomi-hub\cache\galleryinfo` の 429 file を `output\hitomi-hub\cache\images\galleryinfo` へ move。move 後: old 0・new 429。空になった old folder は正典 `kilo-safe-rmdir` でゴミ箱へ送った。`output/` は git 追跡外（`git ls-files output` = 0）でリモートとの差異なしを確認してから move した。

## Verification

- `go vet .`（app）clean。`go test -count=1 -timeout 300s .`（app）→ `ok hitomi-hub/app 25.356s`（新 test 含む）。
- `python -m pytest tests/ -v` → `60 passed in 13.75s`。canonical build 後に `tests/test_canonical_build_freshness.py` を再実行 → `2 passed in 0.82s`。
- canonical build（正典 command・app で実行）: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`
- served 実測（fresh canonical exe・fresh instance・real runtime root `output\hitomi-hub`・port 8747）。UI 変更なし・操作なしなので screenshot は該当なし（画面確認は行っていない）。
  - before file 数: `cache\images` top-level 930・thumbs 436・`cache\galleryinfo`（旧形）429・app 経路 0・`Galleries` 2・`acquisition` 7・DB 行 full 51・thumbs 40
  - `/api/settings` → `cache_dir` `cache/images`・`cache_counts` full 51/thumbs 40
  - `GET /api/gallery/3888535` → 応答が移行後 path の disk file 内容と一致（title `Witch Doctor`・32 file・hash 列一致）。request 後も LastWriteTime `2026-11-06 16:45:57` のまま → disk 層から読み、remote 取得・書き換えなし。この id は `galleries` 既存（605 行中・429/429 が既存）で DB 行増なし
  - `GET /api/image/001313d7…e5bc?format=webp`: 1 回目（04:12:02 UTC）は remote fetch で error body 195 byte。同一 hash の 3 回目（04:12:29 UTC）→ `200`・`X-Cache: cache`・`227012 byte`（`no_full` でない・full cache 順序は変更なし）。この fetch の副次効果で実データに file 1（`001313d7…e5bc.webp`・227012 byte）と DB 行 1（full 51 → 52）が増えた。削除はしていない
  - completion folders: `acquisition` 7・`Galleries` 2（本項目で無変更）
- 未検証: memory 層の `x-cache` clear はこの実測で確認していない（remote 呼出しを避けた。memory clear の実測は 03:41 の scratch 実測で確認済み）

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除（`task_manager end`）
- [x] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（`KI-20261109-01`）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
