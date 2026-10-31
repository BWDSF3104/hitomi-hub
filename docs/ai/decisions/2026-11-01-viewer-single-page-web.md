# ビューアは埋め込み 1 ページのローカル Web（native GUI 不採用）

**Status**: 採用
**Date**: 2026-11-01

## 決定

- 閲覧層は `hub.exe view` が `http://127.0.0.1:8731` に立つ 1 ページ Web。ページ HTML は `app/ui.html` を `embed` で単体 exe に同梱（完成フォルダに追加ファイルを増やさない）
- native GUI（Fyne・egui・webview）は不採用。CGO・WebView2 依存が「コピペで動く完成フォルダ」の前提（単体静的バイナリ・依存 0）を壊す。ブラウザは既存
- 既定 bind は `127.0.0.1` のみ（LAN に公開しない）。`viewer.port` / `viewer.bind` で変更

## 動作規則

- `GET /api/gallery/<id>/page/<n>` は **local-first**：`Galleries/<id>/<hash>.<format>` があれば `/files/<id>/<hash>.<format>` を返す。無ければ shard URL（`StorageURL`）
- `read_state` はビューアが実際にページを表示した時点で更新（POST `/api/read`）。設計どおり「開いた」ではなく「ページ到達」
- お気に入りは POST `/api/fav`、保存検索は POST `/api/query`
- 画像の bytes はブラウザが直接 CDN から読む（rate gate は hitomi API 呼出しのみ）。大量閲覧は `download` 後に local で見る運用を推奨
- `/files/` は `filepath.Clean("/"+rel)` で gallery dir に固定。`..`・絶対路径は gallery dir 内の名前に解消され、外へ出られない
- SQLite は単一接続（`SetMaxOpenConns(1)`）。handler 同時書込みでの SQLITE_BUSY を避ける

## 既知の制限

- phrase・タイトル全文検索は `nozomiurlindex` B-tree が未実装（`known-issues/2026-11-01-plain-title-search-needs-index.md`）
- 保存検索の自動更新（新着追跡）は未実装。保存と一覧のみ
- CBZ/PDF エクスポートは未実装（DL 経路ができてから）
