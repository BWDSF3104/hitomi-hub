# hitomi クライアントの endpoint と呼出し方針

**Date**: 2026-11-01
**Status**: 実測で確定（この環境から HTTPS 応答を確認）

## 確定した endpoint（実測）

- data 正典ホスト: `ltn.gold-usergeneratedcontent.net`（`hitomi.la` の ltn。この環境では `ltn.hitomi.la` は A レコードなしで解決失敗。`hitomi.la` 本体は 200）
- 検索（nozomi）: `https://ltn.gold-usergeneratedcontent.net/n/tag/<term>-<lang>.nozomi`
  - big-endian int32 の gallery ID 配列。`female:yuri-english` で 10004 件（40016 bytes）
  - ページ取得は `Range` ヘッダ（1 ページ 25 件 = 100 bytes）。`--page 2` → `bytes=100-199`
  - 検索 HTML は結果を含まない（JS shell）。nozomi を直接読む
- gallery メタ: `https://ltn.gold-usergeneratedcontent.net/galleries/<id>.js` → `var galleryinfo = {...}`（`id` は文字列。`files[].hash`・`name`・`width`・`height`）
- 画像 URL: `https://<sub>.gold-usergeneratedcontent.net/1793469601/<g>/<hash>.<ext>`
  - `g = parseInt(末尾3hex反転, 16)`（`gg.s`）。`sub = <dir letter> + (1 + gg.m(g))`。webp → `w`、avif → `a`
  - 実測: hash `2d18…2b89` → `g=2488`（`gg.m=1`）→ `https://w2.gold-usergeneratedcontent.net/1793469601/2488/<hash>.webp` → 200・210784 bytes
  - `gg.m` の 0 集合（2112 件）は `ltn` の `gg.js`（SHA-256 `f388cd6d64e0b845016354355b6de02eb117c81cf29edcc5c7a485b76f19e489`）から `docs/ai/tmp/gen_ggset.py` で `app/ggset.go` に生成

## 呼出し方針（このプロジェクトの運用）

- 呼出し間隔: 最小 1000 ms（`hitomi.min_interval_ms`）。retries 3、backoff 2000 ms × 試行回数、429/5xx のみ再試行
- UA は明示（`hitomi-hub/0.1 (local hub client)`）、`referer: https://hitomi.la/`
- hitomi は公式 API を持たず Cloudflare 後。方針変更は版更新としてこのファイルに記録
- テストは外部 API を呼ばない（`Client.fetch` を注入して mock）。live 確認は handoff 時に 4 呼出し（nozomi 1・gallery js 1・画像 2）で実施

## 留意点

- `galleryinfo.files[].hash` は hitomi 内部のハッシュで、ローカル SHA256 とは別物。VT cache と `files.sha256` はローカル SHA256 を使い、URL 構築だけ hitomi の hash を使う（混同すると cache が誤ヒットする）
- `read_state`・`bookmark_queries` は `galleries` を参照する。CLI は `read`/`fav`/`search` で gallery 行を先に upsert する（未登録 ID で FK 失敗しない）
