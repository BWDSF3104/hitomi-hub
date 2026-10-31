# hitomi-hub

hitomi.la 用クライアントのハブ。検索・閲覧・ダウンロードに加え、サイト側にない機能（既読管理・お気に入り・構造検査）をローカル DB で持つことを目的とする。

設計方針（`docs/ai/decisions/`）:

- ウイルス判定引擎はアプリ側で所有しない。OS 側のリアルタイム保護に委ねる
- WSL・Docker・常駐 daemon に依存しない
- 完成フォルダをコピーすればそのまま動く（単体静的バイナリ + data ファイル）

## 現状

実装済み: 構造検査・ハッシュ評判・検索（nozomi）・gallery メタ・ダウンロード・既読・お気に入り・保存検索。UI は未実装。

`src/scan.py` はスキャン層の参照実装（stdlib のみ）、`app/` は製品形（単体静的バイナリ・pure-Go）で、status 語彙と finding 文字列は一致させている。

```
python src/scan.py --root <root> --gallery-dir Galleries
hub.exe scan --root <root>
hub.exe search --term "female:yuri" --lang english --page 1
hub.exe search --title "kemo nee-san 10"                       # galleriesindex（word 倒引の共通部分）
hub.exe gallery --id <id> | --url <hitomi URL>
hub.exe download --id <id> --limit N
hub.exe read --id <id> --page N [--complete]
hub.exe fav --id <id> [--unfav]
hub.exe list --list unread | favorites | queries
hub.exe check --lang english [--limit N]                       # 保存検索の新着を追跡（galleries に記録）
hub.exe view [--port 8731]                      # 1 ページのローカル Web ビューア（127.0.0.1）
go build -o hub.exe .                                   # CGO_ENABLED=0
```

## 構成

```
hitomi-hub/
├── app/                       # Go 実装（単体バイナリ）
│   ├── main.go                # CLI・DB・walk・verify・サブコマンド
│   ├── scan.go                # magic / zip entry / PDF key / status
│   ├── vt.go                  # hash-only lookup・rate gate・cache・yr 呼び出し
│   ├── hitomi.go              # nozomi 検索・gallery メタ・URL 構築・呼出し間隔
│   ├── searchindex.go         # galleriesindex の B-tree 降下（hash_term・node 解码・token 共通部分）
│   ├── library.go             # galleries upsert・read_state・favorite・bookmark_queries
│   ├── check.go               # 保存検索の新着追跡（term ごとの集合の共通部分・last_run_at）
│   ├── viewer.go              # ビューア（1 ページ Web・local-first・path guard）
│   ├── ui.html                # embed される単一ページ（外部 asset なし）
│   ├── ggset.go               # gg.js（ltn）から生成した sharding 集合
│   ├── *_test.go
│   └── go.mod                 # modernc.org/sqlite（pure-Go・CGO なし）
├── src/scan.py                # 参照実装（stdlib のみ・Python 3.10+）
├── schema.sql                 # galleries / files / scan_cache / findings / bookmark_queries / read_state
├── config.example.json
├── rules/structural.yar       # YARA-X 規則（文字列・数値条件のみ）
├── tests/test_scan.py
├── tools/VERSIONS.md          # 単体 exe・toolchain の版固定と digest
└── docs/ai/                   # AI Memory Bank（1エントリ1ファイル）
```

data ファイル（`schema.sql`・`rules/`・`config.example.json`）は完成フォルダ直下に置く。exe 自身はコミットしない。

`scan_status` の語彙: `pass` / `hit` / `not_evaluated` / `external_removal` / `scan_error`。画像は `not_evaluated`（引擎なしのため clean を約束しない）。

## 単体 exe 方針

exe はコミットしない。版固定 + GitHub の SHA-256 digest 検証済みのもののみ受け入れ、記録は `tools/VERSIONS.md`。

## 検証

```
python -m pytest tests/ -v
```

テストは外部 API を呼ばない（VirusTotal は mock）。
