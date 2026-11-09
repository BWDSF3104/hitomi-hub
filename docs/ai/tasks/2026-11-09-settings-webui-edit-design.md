# Web UI から設定を変更する形の検討（allowlist・反映方法・guard）

**Status**: 進行中
**Date**: 2026-11-09
**Next Action**: Settings dedicated page implemented and verified. Future write allowlist, atomic write, floor re-validation, shared config reference, path guard undecided.

## 意図 / 背景

設定値の集約（単一出力元）が確定した（`docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md`）。その上に、Web UI から変更可能にする形を検討する。UI で変更可能にする範囲は「好みで変えても正典の保証・テストアンカー・データが壊れない値」に限るという基準を適用する。

## 調査結果

現状の口（実測 HEAD）:

- config は起動時 1 回読む: `loadConfig`（`app/main.go:159-217`）→ `resolveConfigPath`（`main.go:219-230`、primary `config.json`、missing 時 `config.example.json`）。UI からの reload 口はない。
- read channel は既存: `GET /api/viewer-config`（`app/viewer.go:179`、`559-561`）が `max_object_urls` だけ page へ渡す。page 側は `app/viewer.html:1494-1498` で受取。
- write channel は存在しない。atomic write helper `writeAtomically`（`app/viewer.go:463-475`）は cache 専用（magic 照合・seam の write 回数カウント）で、config 書き込みへは流用できない。config 用は temp+rename が要る。
- UI の write 口は DB だけ（`POST /api/read`・`/api/fav`・`/api/viewed`・`/api/query`・`/api/cache/acquire`、`app/viewer.go:188-194`）。config 書き込みパターンはない。
- page は embed 2 枚（`app/viewer.go:23`）。settings の配置（既存 page 内 panel か 3 枚目）は未決定。

即時反映できるか:

- request 時に cfg を読む値: gate interval（`app/hitomi.go:527,531`）、gg force（`app/hitomi.go:134`）、limits（`app/export.go:123,136`、`app/scan.go:180,193`、`app/vt.go:260-263,306`）、memory cap（`app/viewer.go:550-555`）。
- ただし `Client.cfg` は構築時の写し（`app/hitomi.go:194-196`、`app/viewer.go:128`）で、memory 更新を反映するには共有参照か client 再構築が必要（未決定の実装点）。
- 即時反映できない: `viewer.port`/`bind`（listener 確定済み）、`db`（openDB）、`dataDir`、`schemaVersion`、pragmas。dir 3 件は `newViewer` で root を確定済み（`app/viewer.go:125-130`）なので、変更则 root 再計算が必要。

guard・secret・security:

- floor guard（`minHitomiRateFloorMS` 50、`minViewerMaxObjectURLs` 1000）は loadConfig のみで効く（`main.go:177-183,213-214`）。UI 書き込み時にも同じ guard を走ませ、不正値は拒否して表示する必要がある。
- `virustotal.api_key` は UI へ出さない（正典条・key 文字列を commit・記録・報告へ書かない）。正典は env `VIRUSTOTAL_API_KEY`。
- `yr` は実行可能ファイルの path。UI で自由に設定すると任意バイナリ実行の口になる。UI 対象外が安全。
- path 系（dir 3 件）には root 内 guard がない。`withinRoot` は export/pdf にある（`app/export.go:107`、`app/pdf.go:301`）。acquisition/cache 経路にはない。cache 永続先を `scratch/`・`output/` へ移さない条（AGENTS.md）と完成フォルダ内書き込み条が前提。

テストアンカー:

- `app/viewer_test.go:4892` が page 側の受取行を照合している。UI 追加は page 文字列照合テストの追加を要る（既存の page anchor 形式、例 `viewer_test.go:1970,2506,2576,2907,3041,3603`）。
- `app/main_test.go` の example 照合テスト（`TestConfigExampleHitomiKeysMatchCodeDefaults`、`TestConfigExampleDirNamesMatchCodeDefaults`、`TestConfigExampleLimitsMatchCodeDefaults`）は `config.example.json` を読む。UI が書く先が primary `config.json` なら影響しない（example は既定正典として分離維持）。
- `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md:64` の「通知 UI を置かない」決定は cap の通知の話。settings panel と同一面かは確認が必要。
- 単一 SQLite 接続（decision `2026-11-04-search-viewer-separation-approach-approved.md`）。config ファイル書き込みは複数 session から競合するため lock + atomic write が要る。

## 決定点（選択が必要）

1. 書き込み先と atomic write の形（`config.json` へ temp+rename。example は既定正典として書かない）
2. 反映方法: 即時（memory cfg 更新・client 共有参照）か、書いてから再起動を明示する形か
3. allowlist の範囲（推荐: `viewer.max_object_urls`、`hitomi` の interval/retry、`image_format`、`use_live_gg`、`limits.*`、`virustotal` の enabled/hash_only/upload_unknown。dir 3 件・port・db・yr・api_key・cap・floor・TTL・page 定数は UI 対象外）
4. UI 側 guard（floor 再実行・不正値拒否・path 系を UI で書ける場合のみ withinRoot）
5. secret の扱い（api_key は env 名だけ表示）
6. page 配置（viewer.html 内 panel か 3 枚目。完成フォルダへ追加ファイルを増やさない条は embed には該当しない）
7. 既存決定との関係（通知 UI を置かない決定と settings panel の関係、test anchor の追加形）

## 変更内容（read-only implementation）

推奨承認（`docs/ai/decisions/2026-11-09-settings-webui-approved-recommendations.md`）で実装したのは display-only settings panel だけ。future write allowlist は未実装。

- `GET /api/settings` を追加。`read_only` を返し、`api_key` value を返さない。`api_key_env` だけ返す。
- settings UI は `viewer.html` から削除し、`/settings` dedicated settings page を追加。
- Settings values/constants は Japanese labels、value cell、meaning、grid gap `.75rem 1.25rem` で表示。
- Settings page labels を Japanese name + original constant name formatへ更新。`Galleries` row は `download completed folder name (gallery_dir)`、`acquisition` row は `acquisition folder (acquisition_dir)` にし、両者を混同しない意味 added.
- Dedicated settings page 内では `Browser memory cap` と `Disk cache cap` を read-only 表示項目として扱う。Viewer page には cap notification UI を置かない。
- `viewer.html` に settings panel、`settings-values`、settings fetch、`renderSettings()` を含めない。
- `settings.html` を `go:embed` に追加し、canonical freshness test へ `app/settings.html` を追加。
- Settings API に `constants` を追加。`api_key` value は返さない。`api_key_env` だけ返す。YARA path は UI で表示しない。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`: ok。Settings API constants test、viewer no-settings-panel test、settings page label/spacing test を追加。
- `python -m pytest tests/ -v`: 60 passed。`test_canonical_executable_embeds_current_pages` 合格（`app/settings.html` を含む）、`test_viewer_objecturl_cap_and_status_priority_behaviours` の no notification UI condition 合格。
- canonical build freshness: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` を再実行。
- served canonical server: `curl.exe -s http://127.0.0.1:8731/api/settings` が `read_only=true`、`api_key` value なし、`api_key_env=VIRUSTOTAL_API_KEY`、`constants.cache_disk_max_entries=4000`、`constants.viewer_floor=1000` を返す。
- served `/settings` visual: fresh isolated origin `settings-page-20261109`。actual window size `1666x932`、content height `1751`。Settings values/constants が Japanese labels + `gap: .75rem 1.25rem` で表示。Updated label verification: `DB file name (db)`, `download completed folder name (gallery_dir)`, `acquisition folder (acquisition_dir)`, `Viewer lower bound`, `negative cache cap`, `politely backoff interval`, `gallery info disk cap`. Drive: `I:\マイドライブ\kilo\2026-11-09-settings-webui-edit-design\settings-page-20261109-v2-full.png`。
- existing viewer behavior: served `/` visual snapshot に settings panel なし。Viewer page の nav、reader controls、acquisition controls、reader form は表示。Drive: `I:\マイドライブ\kilo\2026-11-09-settings-webui-edit-design\viewer-no-settings-panel-20261109.png`。local copies were moved to `scratch/` and sent to Recycle Bin by `kilo-safe-remove`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
