# アーキテクチャ

```
[完成フォルダ]
  hub.exe (単体静的バイナリ・目標)  ← 現状は src/scan.py (stdlib)
  config.json  rules/*.yar  schema.sql
  cache/  hitomi.db  Galleries/
```

層:

1. **正典 DB（ハブ）** — SQLite。主キーは `gallery_id`。`files`（sha256・magic・scan_status）、`scan_cache`（sha256→verdict）、`findings`（根拠）、`bookmark_queries`（クエリブックマーク）、`read_state`（既読・お気に入り）
2. **構造検査** — zip エントリ偽装・Zip Slip・解凍比・エントリ数、PDF の viewer-action/embedded、magic byte と拡張子の不一致、PE ヘッダ。引擎に依存しない
3. **ハッシュ評判** — VirusTotal hash-only（任意・既定無効）。エンジン所有なし
4. **OS 側の検知** — 統合しない。アプリは書き込みを完成フォルダ内に閉じ、欠損を `external_removal` として記録するのみ

単体 exe は版固定 + GitHub SHA-256 digest 検証済みのもののみ（`tools/VERSIONS.md`）。
