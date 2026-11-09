# Web UI settings acquisition folder editable

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: Implement acquisition_dir editable UI write, validation, immediate root reflection, tests, confirmation.

## 意図 / 背景

ユーザー指示で `acquisition_dir` を Web UI から変更可能にする。Settings API の top-level settings value と Web UI editable write channel は別層で、以前の実装では `config.example.json` key としての設定化のみで UI write allowlist 対象外だった。今回、`acquisition_dir` の UI editable write、path validation、immediate memory reflection、group card UI、tests、served confirmation を追加した。

## 調査結果 / 変更内容

（記入）

## Verification

- `python -m pytest tests/ -v` in `S:\Documents\Kilo_Commons\hitomi-hub`: `60 passed in 17.30s`. `tests/test_canonical_build_freshness.py` passed after canonical build; `app/settings.html` is embedded in canonical exe.
- Canonical build `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` in `app` succeeded.
- Go test: first run failed only on settings UI exact string `key == "acquisition_dir"`; corrected test anchor to JS actual `key === "acquisition_dir"`. Rerun `go vet . && go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 25.176s`.
- Served confirmation server from canonical exe:
  `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --port 8731 --data-dir "S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\tmp\settings-acquisition-ui-confirmation" --root "S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\tmp\settings-acquisition-ui-confirmation"`
  in isolated context `settings-acquisition-ui-20261109`.
- `GET /api/settings` returned `status 200`, `acquisition_dir = acquisition`, `editable_keys` contains `acquisition_dir`.
- Settings UI POST: input `input[data-key="acquisition_dir"]` was changed to `collect`, `settings-status` became `Saved`, `GET /api/settings` after POST returned `acquisition_dir = collect`; scratch primary `config.json` contained `"acquisition_dir": "collect"`.
- Full-page screenshot showed `取得フォルダ` group with editable `collect` input, `Saved` status, `cache_disk_max_entries` as `キャッシュ` editable group, and `Viewer lower bound (viewer_floor) = 1000` in `内部定数`.
- Screenshot saved to `I:\マイドライブ\kilo\2026-11-09-settings-webui-edit-design\settings-page-20261109-v6-acquisition-group-full.png`. Local confirmation screenshot and scratch confirmation folder were removed by `kilo-safe-remove` / `kilo-safe-rmdir`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
