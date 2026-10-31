# hitomi-hub

hitomi.la 用クライアントのハブ。検索・閲覧・ダウンロードに加え、サイト側にない機能（既読管理・お気に入り・構造検査）をローカル DB で持つことを目的とする。

設計方針（`docs/ai/decisions/`）:

- ウイルス判定引擎はアプリ側で所有しない。OS 側のリアルタイム保護に委ねる
- WSL・Docker・常駐 daemon に依存しない
- 完成フォルダをコピーすればそのまま動く（単体静的バイナリ + data ファイル）

## 現状

実装済みはスキャン層のみ。ダウンロード済みフォルダに対する構造検査とハッシュ評判、SHA256→verdict のキャッシュ、DB 正典。検索・DL・UI・既読/お気に入りの書き込み口は未実装。

```
python src/scan.py --root <root> --config src/config.example.json --gallery-dir Galleries
python src/scan.py --root <root> --config src/config.example.json --verify
```

## 構成

```
hitomi-hub/
├── src/
│   ├── scan.py                # stdlib のみ（Python 3.10+）
│   ├── schema.sql             # galleries / files / scan_cache / findings / bookmark_queries / read_state
│   ├── config.example.json
│   └── rules/structural.yar   # YARA-X 規則（文字列・数値条件のみ）
├── tests/test_scan.py
├── tools/VERSIONS.md          # 単体 exe の版固定と digest
└── docs/ai/                   # AI Memory Bank（1エントリ1ファイル）
```

`scan_status` の語彙: `pass` / `hit` / `not_evaluated` / `external_removal` / `scan_error`。画像は `not_evaluated`（引擎なしのため clean を約束しない）。

## 単体 exe 方針

exe はコミットしない。版固定 + GitHub の SHA-256 digest 検証済みのもののみ受け入れ、記録は `tools/VERSIONS.md`。

## 検証

```
python -m pytest tests/ -v
```

テストは外部 API を呼ばない（VirusTotal は mock）。
