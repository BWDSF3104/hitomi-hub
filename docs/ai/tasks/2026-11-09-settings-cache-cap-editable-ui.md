# Approved settings cache cap editable and group card UI

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: Implement approved cache_disk_max_entries setting, keep viewer_floor constant, update settings page group cards, tests, confirmation.

## 意図 / 背景

ユーザー推奨を承認した。Settings page で、`cache_disk_max_entries` を settings として変更可能にする。`viewer_floor` は lower-bound guard の性格が runtime cap と混ざりやすいため、定数正典として read-only に残す。Settings page の group boundary が分かりづらいため、accessible group card を採用する。

## 調査結果 / 変更内容

既存 decision `docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md` は cap・floor を定数正典として維持する条件を持っていた。今回の承認は `cache_disk_max_entries` の settings 化だけ supersede する。`viewer_floor` の settings 化は supersede しない。

変更内容:
- `cache` settings group を追加。Field `cache.cache_disk_max_entries` を allowlist へ追加。
- Value semantics: `0` は未指定・既定 `defaultCacheDiskMaxEntries=4000`。Explicit positive value は `1..20000`。Negative と `0..20000` 超過を reject。
- Successful write 後に `v.cacheDiskMaxEntries` を memory へ即時反映する。
- Settings page は `fieldset` / `legend` / `label for` / `aria-describedby` を使う。Editable group は group card として visual boundary を明確にし、read-only constants は別 group card にする。
- Existing `viewer.max_object_urls` allowlist・floor guard・client immediate reflection condition を維持する。Existing viewer page must not contain settings panel, settings fetch, `settings-values`, or `renderSettings()`.

## Verification

- `python -m pytest tests/ -v` in `S:\Documents\Kilo_Commons\hitomi-hub`: `60 passed in 17.52s`. `tests/test_canonical_build_freshness.py` passed after canonical build; `app/settings.html` is embedded in canonical exe.
- Canonical build `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` in `app` succeeded.
- Served confirmation server from canonical exe:
  `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --port 8731 --data-dir "S:\Documents\Kilo_Commons\hitomi-hub\docs\ai\tmp\settings-cache-ui-confirmation" --root "S:\Documents\Kilo_Commons\hitomi-hub"`
  in isolated context `settings-cache-ui-20261109`。
- `GET /api/settings` returned `status 200`, `cache.cache_disk_max_entries = 4000`, `editable_keys` contains `cache.cache_disk_max_entries`, `constants` has `viewer_floor = 1000` and does not contain `cache_disk_max_entries`.
- Settings UI POST: input `cache.cache_disk_max_entries` was changed to `5000`, `settings-status` became `Saved`, `GET /api/settings` after POST returned `cache.cache_disk_max_entries = 5000`; scratch primary `config.json` contained `"cache_disk_max_entries": 5000`。
- Go test: first run failed on settings UI exact string `label for`; JS actual accessible label association is `setAttribute("for", inputId)`, so test anchor was corrected. Rerun `go vet . && go test -count=1 -timeout 300s .` in `app`: `ok hitomi-hub/app 25.142s`.
- Screenshot saved to `I:\マイドライブ\kilo\2026-11-09-settings-webui-edit-design\settings-page-20261109-v5-cache-group-card-full.png`. Full-page screenshot showed group cards, `キャッシュ` editable cache group, `内部定数` read-only constants without `cache_disk_max_entries`, `Viewer lower bound (viewer_floor) = 1000`, and `Saved` status.
- Local confirmation screenshot and scratch confirmation folder were removed by `kilo-safe-remove` / `kilo-safe-rmdir`.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
