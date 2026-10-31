# hitomi ハブのスキャン層：Defender 非統合・WSL 非依存・移植可能単体バイナリ

**Date**: 2026-11-01

## Decision

- ウイルス判定引擎はアプリ側で所有しない。Defender との統合（cmdlet 呼び出し・イベントログ読取）は行わず、OS 側のリアルタイム保護に委ねる。アプリが DB に書くのは自前検査結果のみ。
- `scan_status` の語彙を `pass` / `hit` / `not_evaluated` / `external_removal` / `scan_error` に固定。画像は `not_evaluated`（Defender 管轄で結果不明）とし、clean を約束しない。
- 検査層は 3 部分: 構造検査（Python stdlib: zip エントリ偽装・Zip Slip・解凍比・PDF の viewer-action/embedded・magic byte 不一致）、ハッシュ評判（VirusTotal の hash-only 照会、アップロード既定無効）、SHA256→verdict の永続キャッシュ。
- 単体 exe は「版固定＋GitHub の SHA-256 digest 検証済み」のみ受け入れる。追加は `yr`（YARA-X CLI）を任意、sqlite3 CLI を任意。ClamAV 非公式ビルド・yextend・第三者ミラー exe は不使用。
- 完成形は単体静的バイナリ（Go）＋ data ファイル。現状 dev toolchain（Go）未導入のため、実装は stdlib のみで動く Python 版を先行させる。
- exe と DB を含む完成フォルダは「アプリ自身の書き込みをフォルダ内に閉じる」ことを移植条件とする（絶対パス・`%APPDATA%`・temp 展開を書かない）。

## Reason

- Defender は exit code で clean と「検知＋駆除済み」を区別できず、quarantine により DB の参照ファイルが消える。ファイル単位スキャンの呼出しコストと lock 失敗（並列実行で exit 2）も大きく、DB 更新の信号源として不適。
- WSL/Docker 前提の引擎（ClamAV daemon）は「コピペで動く」要件と衝突。非公式 Windows ビナルは provenance が弱い。
- 画像（webp/avif）はシグネチャ引擎の検率がほぼゼロ。実効性はアーカイブ内部・PDF 構造・拡張子と magic bytes の不一致にあり、これは引擎に依存せず自前検査で覆盖できる。
- VT のハッシュ照会は内容を送信せず、既存レポート参照のみでクォータ（4 req/min・500 req/day）内で運用でき、キャッシュにより再ダウンロードは 0 回で済む。

## Rejected Alternatives

- Defender 統合（`Get-MpThreatDetection`・Operational log 1116/1117）: 判定をアプリが所有することになり、方針と矛盾。quarantine でファイルが消えるため DB 参照が壊れる。
- ClamAV daemon（WSL/Docker）: 移植性・インストール数で劣る。
- `.gitignore` に gallery フォルダ排除相当の運用: 排除すると検知イベント自体が消える（信号源を失う）。
- Flutter（hitomi_search_plus）・Tauri（hitomi-downloader）・.NET（Hitorus）を基盤選択: 単体 exe 化できない、WebView2/.NET 9 ランタイム依存、submodule と SDK 前提で「コピペ」要件に合わない。Hitorus は GPL-3.0 により改造配布の制約もある。

## Impact

- `scripts/hitomi_hub/`（scan.py・schema.sql・config.example.json・rules/structural.yar）と `tests/test_hitomi_scan.py`（11 tests）。テストは VT を mock し外部 API を呼ばない。
- `tools/VERSIONS.md` に版と digest を記録。exe は非追跡。
- Go 単体バイナリ化（pure-Go SQLite）は dev toolchain の導入承認を要する。FTS5 を設計に入れるなら pure-Go ドライバの対応確認が前提。
