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
.\hub.exe search --title "kemo nee-san 10"                      # galleriesindex（word 倒引の共通部分）
.\hub.exe gallery --id 4294614                                   # gallery メタ（閲覧）
.\hub.exe gallery --url "https://hitomi.la/doujinshi/...-4252697.html"  # URL から id を抽出
.\hub.exe download --id 4294614 --limit 2                        # Galleries/<id>/ へ保存し scan まで実行
.\hub.exe read --id 4294614 --page 3                             # read_state 更新（--complete で完全既読）
.\hub.exe fav --id 4294614                                       # お気に入り（--unfav で解除）
.\hub.exe list --list unread                                     # unread | favorites | queries
.\hub.exe check --lang english --limit 10                        # 保存検索の新着を追跡し galleries に記録
.\hub.exe export --id 4294614                                    # cbz（page 順）。--format pdf で pdf
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

### 保存検索の新着追跡（`check`）

- `bookmark_queries` の各 query を 1 回引き、`galleries` にまだ無い id だけを記録し、`last_run_at` を更新
- multi-term query は term ごとの集合の共通部分（nozomi）。`language:` term は fetch せず lang を設定（hitomi と同じ）。`language:x` 単独は `/n/index-<lang>.nozomi`
- namespace なし（phrase・単一 word）は galleriesindex の token 共通部分
- 新着判定は `galleries` 行の有無。重复実行は 0 件になる（実測: `artist:kikunyi language:japanese` → 10 件記録 → 再実行で残り 15 件 → 3 回目は 0）
- `--limit N` は query ごとの記録上限。`notify` 列は現行実装では未使用

### エクスポート（`export`）

- page 順は `pageorder`（download 時に記録）。`pageorder` が無い gallery は `files` 行の挿入順（rowid）
- entry 名は `NNN_<hash>.<ext>`。zip は Store（再圧縮なし）
- `hit`・`scan_error` のファイルは skip。`missing`（ファイル消失）も skip。`--out` は root 外へ出せない
- `files.gallery_id` を使う。過去の行は NULL なので `scan` を再実行すると path から埋まる（ON CONFLICT で更新）
- pdf は decoder が通る形式のみ embed（webp・png・jpeg）。AVIF は pure-Go decoder が無いので skip。webp は `golang.org/x/image/webp`（pure Go・CGO なし）
- pdf は 1 ページ 1 画像（MediaBox は画像サイズ・FlateDecode の raw RGB）

### `search.html` exact term 規則（2026-11-01 実測）

- `search.html` の単一 word term は substring 検索ではなく `galleriesindex` の exact term key lookup（`hashTerm(term)`）で扱う。`furry` と `furr` と `fur` は別 term。
- `hash_term('furry')` live key bytes: `a1d72c62`; Go `hashTerm("furry")` same。
- Current live total: `furry` `32832`, `furr` `32837`, `fur` `47007`, `furryx` `0`。`furry` first ids: `4295047, 4294784, 4294785, 4294786, 4294788, 4294536, 4294426, 4294423, 4294422`。
- CLI parity with rebuilt `hub.exe`:
  - `search --term "furry" --lang all --page 1`: total `32832`, first ids `4295047, 4294784, 4294785, 4294786, 4294788`
  - `search --term "furr" --lang all --page 1`: total `32837`
  - `search --term "fur" --lang all --page 1`: total `47007`
  - `search --term "furryx" --lang all --page 1`: total `0`
- `results.js` の `do_search()` は order を再ソートせず、`get_galleryids_for_query()` の data order を results として使う。Go `SearchHTML` も index data order を保持し、`orderbydirection:asc` で reverse、`random` で shuffle する。
- Viewer `/api/search?query=<q>&lang=<lang>&page=<page>` は plain exact term、colon term、negative term、order、pagination を `SearchHTML` に渡す。`lang=all` default。

### Viewer UI / proxy（2026-11-01）

- `http://127.0.0.1:8731` は 1 page single-page UI。検索 result card は hitomi の galleryblock と同じ cover TN（`bigtn`）だけ。reader strip は hitomi work page と同じく `files[]` の全 entry を TN variant で描画。note、pagenum、reader controls を表示。
- Browser ORB を避けるため画像は same-origin proxy を使う: `/api/thumb/<hash>?format=webp`、`/api/thumb/<hash>?format=webp&gallery=<id>&variant=<tn variant>`、`/api/image/<hash>?format=webp`。hitomi work page の UI variant は DPR 1 で `smallsmalltn`、それ以外 `smalltn`。server は hitomi TN path で TN-first にし、TN が無い場合 local file、さらに full image proxy へ fallback。
- `/api/gallery/<id>` は `files[].local` を返す。`local` は app 機能の marker（`title='local'`・`data-local`・alt）として表示し、表示対象の filter にしない。
- `/api/gallery/<id>` は hitomi `galleryinfo` の `hasavif`（number）と `scene_indexes` を UI へ渡す。UI の reader strip は `scene_indexes` があれば scene item で page を作り、block size を badge として表示。`hasavif` entry は TN `avif` variant、その他は server format/webp を使う。`scene_indexes` empty では `files[]` の全 entry を page として描画。
- `/api/image/` は main image で local full file > remote full image > TN variant fallback。TN fallback order は `bigtn`, `smallbigtn`, `smalltn`, `smallsmalltn`。`hasavif` entry は `avif` TN を優先し、その TN が無い場合は webp TN を最終 fallback として試す。main image load failure は UI note に出す。
- `pageSource` は local を config format で確認後、remote source に `hasavif` format を使う。実測 gallery `2648601` page 2 の `hasavif=1` entry では、`pageSource` が `format=avif` を返し、`imageProxy` の TN variant fallback で `200` 表示できた（`bigtn` は 404、`smalltn` が 200）。スクショ: `scratch\hitomi-usage\full_fallback_page2.png`。
- Search UI は `/api/search` の initial response を ids/total だけ受け取って `search-status` と submit button で実行中表示し、card metadata は `/api/search-card/{id}` で lazy load する。search card cover の TN variant は hitomi と同じ `bigtn`。lazy card lookup selector は `[data-search-id='<id>']`（closing single quote を含む）でなければならない。スクショ: `scratch\hitomi-usage\search_status_executing.png`。
- `/api/image/` は `local full file > persistent full cache > remote full > TN variant fallback`。server は full storage fetch success bytes を `cache/images/<hash>.<format>` に永続保存し、`x-cache` に `local|cache|miss|tn` を返す。TN fallback は full quality cache として保存しない。UI 画面の `prefetch-status` は viewer 画面に入った gallery の残 full images を concurrency `2` で background prefetch する。prefetch は current page を即表示した後、`AbortController` で current gallery/page change に応じて abort され、`full prefetch <done>/<total>` を表示する。スクショ: `scratch\hitomi-usage\image_cache_background_prefetch.png`。
- `ThumbnailURL` の tn path（実測で確定）: `https://<{a|b}tn>.<domain>/<tn base>/<last hex char>/<prev two hex chars>/<hash>.<format>`。TN base は format で決まる — webp `77656270626967746e`（webpbigtn）・avif `61766966626967746e`（avifbigtn）。`746e626967746e`（tnbigtn）は base として存在せず 404。subdomain は `String.fromCharCode(97 + gg.m(g)) + 'tn'`（common.js）。実測例: `https://btn.gold-usergeneratedcontent.net/77656270626967746e/9/b8/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89.webp` → 200 `image/webp`。TN に格納されるのは webp・avif ののみ。`fileExt(name)` は tn path に使わない。
- TN は `referer: https://hitomi.la/` が無いと 404（browser UA でも 404）。client は UA と referer を必ず送る。content-type は format に一致（webp → `image/webp`、avif → `image/avif`）。
- `validHash` は lowercase hex length `>=3`。Live result hashes は 64 に限らない（48 chars など）。
- `ggset.go` は `gg.js` の snapshot（fallback）。実行時は 1 実行 1 回正典 `gg.js` を読む（`app/gg.go` の `parseGG`・config `hitomi.use_live_gg` 既定 true・失敗時 snapshot へフォールバックし stderr に通知）。`gg.m(g)` は switch の case にある value で 0（`w1`/`atn`）、それ以外 1（`w2`/`btn`）。`gg.b`（shard base）は 1 時間刻みで前進する（実測 `1793491201/` → `1793494802/`）。shard URL は live が指す窓に data が在るときのみ解ける（9:35 に 200 の URL は 10:05 に 404）。TN path は bucket を使わないため影響なし。生成正典: `tools/gen_ggset.py`。tests は rotating 値をハードコードしない（fixture で規則を検証）。
- Live image proxy sample: `/api/image/098fa2e4529e651af3fe594fa57a57053598502d5ead2533edc03726d1f3f2ce?format=webp` returns `200` WEBP. Reader full image loads width `1997`.
- Screenshot: `scratch\hitomi-usage\viewer-new-ui-loaded.png` (left cards and right reader visible).

### 完成品フォルダ・git 管理（2026-11-01）

- 完成品フォルダは `output\hitomi-hub\`。実体は `hub.exe` と同フォルダ直下の data file（`schema.sql`・`config.example.json`・`rules\structural.yar`・`使い方メモ.md`）。exe と `output/` は `.gitignore` で非追跡。
- 開発 runtime・scratch・DB（`scratch/`、`*.db`、`scratch/go`、`scratch/gopath`、`scratch/gocache`）は git にコミットしない。repo が追跡するのは source・tests・docs・`tools/VERSIONS.md` など。
- Go toolchain は winget で適切な開発環境としてインストール済み。この project の command では `go` を使う。
- Kilo tool process の PATH refresh が反映されない場合は、正典 path を絶対パスで使う:
  - `C:\Program Files\Go\bin\go.exe`
- 開発 runtime・scratch・DB（`scratch/`、`*.db`、`scratch/gopath`、`scratch/gocache`）は git にコミットしない。repo が追跡するのは source・tests・docs・`tools/VERSIONS.md` など。
- 完成 exe の build（pure-Go・CGO なし）:
  - `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o output\hitomi-hub\hub.exe .`
- 完成品起動: `hub.exe` を単体静的バイナリとして運用する。view mode は `hub.exe --root <runtime-root> view --port 8731`。`--root` を省略すると cwd または nearest `.git` parent を root とみなす。単体コピペフォルダで使うなら runtime root を完成品フォルダ内（または明示指定した runtime フォルダ）に限定する。
- 現在の dev viewer は `output\hitomi-hub\hub.exe` から、runtime root `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage` を明示して起動している。

### 検索の分区規則（hitomi と同じ）

- `female:`/`male:` → area = `tag`、tag はクエリ全体（例 `/n/tag/female:yuri-english.nozomi`）
- other namespace（`type:`・`artist:`・`group:`・`series:`・`character:`）→ area = namespace、tag = 右辺（例 `/n/type/doujinshi-japanese.nozomi`）
- `language:` → lang に反映
- namespace なしの単語 term（単一タグ）→ 直Path（例 `/n/goudoushi-japanese.nozomi`）
- **単語数 2 つ以上の phrase / タイトル全文検索**は `--title` で `galleriesindex` を降りる（下記）

### 検索の index 経路（`--title`）

- `hash_term(term)` = sha256(UTF-8(term)) の先頭 4 bytes（小文字化後）。node は big-endian: key 数 → keys（key ≤ 32 bytes）→ datas(offset,length) → subnode `B+1`（`B=16`・`max_node_size=464`）
- `galleriesindex` の field `galleries` は **word → gallery IDs の倒引**。phrase は token（whitespace と `- / # '` 区切り）ごとの集合の共通部分。順序は最初の token の順序（newest first）
- version は `https://ltn.gold-usergeneratedcontent.net/galleriesindex/version`（`ltn.hitomi.la` はこの環境で応答せず）。1 実行で 1 回だけ読む
- 実測: `kemo` → 100726 件（data 402908 bytes）。`--title "kemo nee-san 10"` → 共通部分 22 件（`4252697` を含む）。`nee-san` は 1 key ではなく `nee`・`san` に切れる
- `--page`（1 origin・25件）と `--limit` は**共通部分の後**に適用。DB に記録するのは表示分だけ

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
| 検索 | 動作（nozomi + index） | nozomi 1 ページ 25 件（`female:yuri-english`）。`--page 2` は Range `bytes=100-199`。`--title "kemo nee-san 10"` は galleriesindex 降下・token 共通部分 22 件（`4252697` を含む）。galleries に行を記録（表示分のみ） |
| URL からの参照 | 動作 | `--url` で `<digits>.html` から id 抽出（`GetGalleryId` と同じ規則）。gallery/download/read/fav で使用可 |
| 閲覧 | 動作（CLI + 1 ページ Web） | `view` で `http://127.0.0.1:8731`。local-first 表示、ページ到達で `read_state` 更新。実測: `/api/gallery/4252697/page/1` → local（download 済み）、未保存は shard URL。path guard は gallery dir 内固定 |
| 一括ダウンロード | 動作 | `--limit 2` で 2 件保存し、そのまま scan 実行（files に sha256・magic・status を記録） |
| 既読管理 | 動作 | `read --id 4294614 --page 3` → `last_page=3`、`--complete` → `fully_read=1` |
| お気に入り | 動作 | `fav --id` → `favorite=1`。`--unfav` で解除 |
| 保存検索 | 動作 | `--name/--query` で `bookmark_queries` に記録、`list --list queries`。`check` で新着を galleries に記録し `last_run_at` 更新（実測: `artist:kikunyi language:japanese` → 10 件記録・再実行 15 件・3 回目 0） |
| CBZ/PDF エクスポート | 動作 | `export --id 4294614` → cbz entry `001_<hash>.webp`（Store・page 順）。`--format pdf` → 1 ページ 1 画像（webp decoder が通る）。実測: cbz 1 entry・pdf 2114910 bytes・xref/EOF 確認 |
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

- AVIF の pdf embed（pure-Go decoder が無い）
- デバイス間同期
- native GUI（1 ページ Web で代替。CGO・webview 依存を避けるため不採用）

## 留意点

- main ループは scan target すべてで `yr` を走らせる（構造検査の hit 判定に依存しない）。役割分担: 構造検査は単独 key/entry 条件、YARA は複合条件・参照 entry（`Zip_Reference_Entry`・`PDF_OpenAction_Launch`・`PDF_AA_Script`・`PDF_XFA_Embedded`）。findings の `rule` 列は engine 名（`structural`/`yara`/`virustotal`）、`detail` は検知内容（決定: `docs/ai/decisions/2026-11-01-yara-role-division-and-gate.md`）
- exe はコミットしない。版・digest の記録は `tools/VERSIONS.md`（go1.27.3 / yr v1.22.1）
- OS 側のリアルタイム保護（Defender）はアプリから制御しない。external_removal として記録する
