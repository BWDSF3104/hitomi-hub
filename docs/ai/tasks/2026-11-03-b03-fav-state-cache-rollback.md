# B-03: お気に入り状態がキャッシュで巻き戻る

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 実施前確定（修正方法・影響範囲・合格条件・検証方法）

作業前チェック（AGENTS.md）読了ファイル名明記:
- `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`
- `docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`
- `docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`
- 前提: `docs/ai/decisions/2026-11-03-b01-nav-enter-noop.md`（nav Enter no-op・意味を変えない）

修正方法（確定）:
- `app/ui.html` に `syncReadStateCache(id, patch)` を追加: `searchCardCache` / `galleryCache` の該当 entry の `read_state` を patch で更新（entry なければ何もしない）。
- `#fav` click handler: `post("/api/fav")` 成功後、`current.favorite` 更新に加えて `syncReadStateCache(current.id, { favorite: current.favorite })` を両 cache へ反映。
- `#complete` click handler: `post("/api/read", complete: true)` 成功後、`syncReadStateCache(current.id, { last_page: current.page, fully_read: 1 })` を両 cache へ反映（DB の setReadState 効果と一致）。
- 対応案の「click 前に DB と突き合わせる」は不要: toggle 直後に cache を DB 効果と一致させるため、合格条件の巻き戻りシナリオは解消される（B-03 の実測シナリオは handler 内の cache 未更新が原因）。
- 影響範囲: ui.html の handler と helper のみ。server 側（/api/fav・/api/read・imageProxy・TN・永続 cache・LRU・form submit セマンティクス）は無変更。
- 合格条件（固定テストで固定）: toggle 時に両 cache の read_state が更新される／cache 経由再表示（`setGalleryMeta` は `read_state.favorite` を使う）後も favorite 反映と toggle 方向が正しい／再 click で送る `on` 方向が DB と一致。
- 検証方法: `go vet . && go test -count=1`（固定テスト追加）→ canonical exe 再 build → `tests/test_canonical_build_freshness.py` → canonical server を scratch dataDir ルートで再起動（正典 DB への書き込み残さない）→ Playwright 実操作（gallery 4300588）で request body と `/api/lists` 実測 → 画面キャプチャを Google Drive へ保存 → ルートで `python -m pytest tests/ -v`。

## 調査結果 / 変更内容

有効（実測）。`#fav` click（`app/ui.html:1448-1454`）は POST 後 `current.favorite` だけ更新し、`galleryCache`/`searchCardCache` の `read_state.favorite` を更新しない。実測（gallery 4300588, `?newtab=1` tab）: 1 回目 click で `{"gallery_id":4300588,"on":true}` → `current.favorite` 1。同じ card を再 click（cache hit path, `app/ui.html:1147,1157,1173`）で `current.favorite` が 0 へ巻き戻り、card の `read_state.favorite` も 0。続けて click すると unfavorite 期待で `on:true` が送られ（実測 request body `{"gallery_id":4300588,"on":true}`）、DB の favorite は 1 のまま（`/api/lists` の favorites に残る）。対応: toggle 時に `galleryCache`/`searchCardCache` の read_state を更新、または click 前に `!current.favorite` を DB と突き合わせ。合格条件: cache 経由で gallery を再表示した後も favorite 反映と toggle 方向が正しい。推定規模: 小。

## Verification

- 変更（実装）: `app/ui.html` に `syncReadStateCache(id, patch)` 追加（`searchCardCache`/`galleryCache` の `read_state` を patch 更新、entry なければ何もしない）。`#fav` handler: `post("/api/fav")` 成功後 `current.favorite` 更新 → `syncReadStateCache(current.id, { favorite: current.favorite })`。`#complete` handler: `post("/api/read", complete: true)` 成功後 `syncReadStateCache(current.id, { last_page: current.page, fully_read: 1 })`（DB の setReadState 効果と一致）。固定テスト: `app/viewer_test.go` `TestEmbeddedUIFavoriteToggleSyncsReadStateCaches`（両 cache 更新・fav handler の on→post→update→sync の順序・complete handler の post→sync・setGalleryMeta が cache の `read_state.favorite` を読むことを固定）。
- `go vet .`（app）: 出力なし（pass）。
- `go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 36.862s`（2026-11-03）。
- 固定テスト単独: `go test -count=1 -run "TestEmbeddedUIFavoriteToggleSyncsReadStateCaches|TestEmbeddedUIEnterImplicitSubmitIsFixedToSearchSubmit" -v .` → 両 PASS（0.159s）。B-01 の固定（nav Enter no-op）は変更なしで維持。
- canonical exe 再 build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`（canonical server PID 1900 を停止後に実行）。
- ルート `python -m pytest tests/ -v`: 33 passed（3.61s）、`tests/test_canonical_build_freshness.py` 単独 2 passed（0.94s・exe 内 ui.html バイト実測照合）。
- UI 実操作確認（Playwright・canonical exe を scratch dataDir ルート `docs/ai/tmp/assess-20261103-1220/scratch-run` で起動、既定 http://127.0.0.1:8731。正典 DB `output/hitomi-hub/hitomi.db` への書き込みなし＝LastWriteTime 17:57:43 のまま、テストデータは scratch DB のみ）:
  1. 実検索 `female:collar`（lang=japanese）page 1: total 32893・25 card、4300588 含む（`searchCardCache` に card 確立）。初期: card `read_state.favorite=0`、`current.favorite=0`。
  2. card click（searchCardCache 経路）→ gallery 表示。1 回目 `#fav` click: POST body 実測 `{"gallery_id":4300588,"on":true}`（network #114）→ `current.favorite=1`、`searchCardCache` read_state=1、`galleryCache` read_state=1、`/api/lists` favorites=[4300588]（DB favorite=1）。
  3. 同一 card 再 click（cache 経由再表示・`setGalleryMeta`）→ `current.favorite=1`（巻き戻りなし）、card read_state=1、galleryCache read_state=1。
  4. 2 回目 `#fav` click: 送った body 実測 `{"gallery_id":4300588,"on":false}`（network #120・送付時点 DB favorite=1 と方向一致）→ DB=0、cache/current=0 で一致。3 回目 click: body `on:true`（#123・時点 DB=0 と一致）→ DB=1・current=1・両 cache=1 一致（合格条件充足）。
  - 画面キャプチャ: `I:\マイドライブ\kilo\b03-ui-check-2026-11-03.png`（760,610 bytes・workspace 内コピー `docs/ai/tmp/assess-20261103-1220/b03-ui-check-2026-11-03.png`）。確認結果: 検索結果・reader・お気に入りリスト（4300588 p1）表示、状態一致。
  - 保存先・方式: scratch dataDir ルートで正典 exe 起動（書き込みは scratch に閉じる）。正典 DB にテストデータなし。UI 確認後、canonical server を再 build 済み exe で既定 8731（正典 runtime・canonical runtime ディレクトリ）へ復旧（PID 28168）。
- 影響: 変更は `#fav`/`#complete` handler と helper のみ。B-01（form submit・nav Enter no-op）、B-02（objectURL LRU）、viewer・imageProxy 順序・TN・永続 cache・YARA・download への影響なし（固定テストで B-01 維持を確認）。既知の他項目の再オープンなし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
