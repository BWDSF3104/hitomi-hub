# hitomi-hub 使い方メモ（完成品フォルダ）

正典はこのファイル。完成フォルダの `使い方メモ.md` はここから写したコピー（`output/` は追跡外）。

更新: 2026-11-01

## フォルダの形

```
hitomi-hub/            ← このフォルダだけで完結（絶対パス・%APPDATA%・temp 展開なし）
├── hub.exe            # 単体静的バイナリ（pure-Go・VC++ ランタイム不要）
├── schema.sql         # 正典 schema
├── config.example.json
├── rules/structural.yar
├── Galleries/         # ダウンロード済みフォルダ（スキャン対象）
└── hitomi.db          # 実行時にこのフォルダ内へ生成
```

## 使い方

```
.\hub.exe --root .                          # スキャン（config.example.json を読む）
.\hub.exe --root . --config config.json
.\hub.exe --root . --gallery-dir Galleries
.\hub.exe --root . --verify                 # 記録済みで消失したファイルを external_removal に
.\hub.exe --root . --no-vt                  # VirusTotal を呼ばずに構造検査のみ

.\hub.exe search --term "female:yuri" --lang english --page 1   # nozomi 検索（25件/ページ）
.\hub.exe gallery --id 4294614                                   # gallery メタ（閲覧）
.\hub.exe gallery --url "https://hitomi.la/doujinshi/...-4252697.html"  # URL から id を抽出
.\hub.exe download --id 4294614 --limit 2                        # Galleries/<id>/ へ保存し scan まで実行
.\hub.exe read --id 4294614 --page 3                             # read_state 更新（--complete で完全既読）
.\hub.exe fav --id 4294614                                       # お気に入り（--unfav で解除）
.\hub.exe list --list unread                                     # unread | favorites | queries
.\hub.exe search --term "female:yuri" --name "yuri-en" --query "female:yuri language:english"  # 保存検索
.\hub.exe view --port 8731                                     # 1 ページのローカル Web ビューア（127.0.0.1）
```

サブコマンドは `--root` の前後どちらに置いてもよい（`findCommand` がフラグの値をコマンドと誤読しない）。

### ビューア（`view`）

- `http://127.0.0.1:8731`（config の `viewer.port` / `viewer.bind`。既定は loopback のみ）
- ページ HTML は exe に embed（`app/ui.html`）。外部 asset・CDN CSS なし
- 画像は local-first：`Galleries/<id>/<hash>.<format>` があればフォルダ内から、無ければ shard URL
- ページを表示すると `read_state` が更新される（「開いた」ではなく「ページ到達」。`POST /api/read`）
- 保存検索の自動更新（新着追跡）は未実装。保存と一覧のみ

`--id` の代わりに `--url` を `gallery`/`download`/`read`/`fav` で使える（hitomi の `GetGalleryId` と同じ規則で `<digits>.html` から id を抽出）。

### 検索の分区規則（hitomi と同じ）

- `female:`/`male:` → area = `tag`、tag はクエリ全体（例 `/n/tag/female:yuri-english.nozomi`）
- other namespace（`type:`・`artist:`・`group:`・`series:`・`character:`）→ area = namespace、tag = 右辺（例 `/n/type/doujinshi-japanese.nozomi`）
- `language:` → lang に反映
- namespace なしの単語 term（単一タグ）→ 直Path（例 `/n/goudoushi-japanese.nozomi`）
- **単語数 2 つ以上の phrase / タイトル全文検索は未実装**（`nozomiurlindex` の B-tree 降下が必要。`docs/ai/known-issues/2026-11-01-plain-title-search-needs-index.md`）

`--root` を省略すると cwd（または `.git` のある祖先）を使う。`--data-dir` を省略すると exe と同じフォルダの `schema.sql`・`rules/`・`config.example.json` を使う。

## config.json（JSON のみ。TOML は非対応）

```json
{
  "db": "hitomi.db",
  "gallery_dir": "Galleries",
  "yr": null,
  "virustotal": {
    "enabled": false,
    "hash_only": true,
    "upload_unknown": false,
    "api_key_env": "VIRUSTOTAL_API_KEY",
    "api_key": "",
    "max_requests_per_minute": 4,
    "max_daily_requests": 500
  },
  "hitomi": {
    "min_interval_ms": 1000,
    "max_retries": 3,
    "retry_backoff_ms": 2000,
    "user_agent": "hitomi-hub/0.1 (local hub client)",
    "image_format": "webp"
  },
  "viewer": {
    "port": 8731,
    "bind": "127.0.0.1"
  },
  "limits": {
    "max_entries": 2000,
    "max_uncompressed_bytes": 209715200,
    "max_ratio": 100,
    "max_scan_bytes": 2097152,
    "yr_timeout_sec": 10,
    "yr_cpu_limit": 50,
    "yr_skip_larger": 67108864,
    "yr_fast_scan": true
  }
}
```

- `yr`: `tools/yr.exe` のパスを設定すると構造規則を実行。未設定（null）なら exe 内蔵の構造検査のみで完結
- `api_key_env`: 環境変数から key を読む（config の `api_key` より優先）
- `upload_unknown`: 現行実装では未使用（hash-only のみ）

## scan_status の語彙

| status | 意味 |
| --- | --- |
| `pass` | 検査対象（pdf/cbz/zip）で構造検査通過 |
| `hit` | 構造 findings あり（または cache/VT の verdict hit） |
| `not_evaluated` | 引擎なし（画像等。clean を約束しない） |
| `scan_error` | 解読不能（`bad_zip`・読み取りエラー） |
| `external_removal` | `--verify` で消失確認 |

findings の例: `path_traversal:../payload.exe`、`executable_entry:../payload.exe`、`ratio_exceeded:<entry>`、`entry_size_exceeded:<entry>`、`entry_count_exceeded:<n>`、`magic_mismatch:cbz_vs_png`、`mz_without_pe_header`、`bad_zip`、`pdf_object:/Launch`。

## 結果の読み方

```sql
SELECT path, scan_status, scan_source FROM files ORDER BY path;
SELECT path, rule, detail FROM findings ORDER BY path;
SELECT sha256, verdict, engine, threat_name FROM scan_cache;
```

## 機能状況（2026-11-01 実測）

| 機能 | 状況 | 実測結果 |
| --- | --- | --- |
| 検索 | 動作（namespace/単一tag） | nozomi 1 ページ 25 件（`female:yuri-english`）。`--page 2` は Range `bytes=100-199` で 25 件。galleries に行を記録。phrase/タイトル全文検索は index 未実装 |
| URL からの参照 | 動作 | `--url` で `<digits>.html` から id 抽出（`GetGalleryId` と同じ規則）。gallery/download/read/fav で使用可 |
| 閲覧 | 動作（CLI + 1 ページ Web） | `view` で `http://127.0.0.1:8731`。local-first 表示、ページ到達で `read_state` 更新。実測: `/api/gallery/4252697/page/1` → local（download 済み）、未保存は shard URL。path guard は gallery dir 内固定 |
| 一括ダウンロード | 動作 | `--limit 2` で 2 件保存し、そのまま scan 実行（files に sha256・magic・status を記録） |
| 既読管理 | 動作 | `read --id 4294614 --page 3` → `last_page=3`、`--complete` → `fully_read=1` |
| お気に入り | 動作 | `fav --id` → `favorite=1`。`--unfav` で解除 |
| 保存検索 | 動作 | `--name/--query` で `bookmark_queries` に記録、`list --list queries` |
| ウイルスチェック | 動作 | 構造検査 hit 2（`path_traversal`・`executable_entry`・`pdf_object:/Launch`）/ pass 1 / not_evaluated 1。VT cache・rate gate はテストで検証 |
| VirusTotal | 動作（key 要） | key なしでは `no_key` を返し何も書かない。テストは mock（外部 API 呼ばず） |
| YARA-X | 動作（任意） | `yr` 設定時に規則実行。integration test で 5 規則の hit を確認 |

### download について

- 保存先: `Galleries/<gallery_id>/<hash>.<format>`（完成フォルダ内のみ）
- shard にあるのは webp / avif ののみ。`files[].name` の ext（例 `001.jpg`）は元の形式なので URL に使わない。実測: 同じ hash の `.jpg` → 404、`.webp` → 200
- hitomi の `files[].hash` は URL 構築用の内部ハッシュ。DB の `sha256` はローカル計算の SHA256（VT cache はこちらを使う）
- 画像は `not_evaluated`（構造引擎が画像を持たないため clean を約束しない）
- `--limit N` は試行数の上限（失敗した分も含む。rate gate 1000 ms/件）

## 未実装

- phrase・タイトル全文検索（`nozomiurlindex` B-tree 未実装）
- 保存検索の自動更新（新着追跡）
- CBZ/PDF エクスポート（DL 経路ができてから）
- デバイス間同期
- native GUI（1 ページ Web で代替。CGO・webview 依存を避けるため不採用）

## 留意点

- 現在の規則集は構造検査と重複するため、main ループでは `yr` が「構造 findings なし」のファイルにのみ走る。現行規則では YARA 単独の hit が発生しない（既知問題: `docs/ai/known-issues/2026-11-01-yara-no-op-in-main-loop.md`）
- exe はコミットしない。版・digest の記録は `tools/VERSIONS.md`（go1.27.3 / yr v1.22.1）
- OS 側のリアルタイム保護（Defender）はアプリから制御しない。external_removal として記録する
