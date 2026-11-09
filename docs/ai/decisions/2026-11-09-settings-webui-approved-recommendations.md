# Web UI settings 変更設計の推奨承認（7決定点）

**Date**: 2026-11-09
**Status**: active

## Decision

Web UI settings は recommended form で承認した。実装済み範囲は dedicated settings page と editable write channel。Future write allowlist はこの decision の条件を満たす形だけで実装した。

1. settings write target は primary `config.json`。`config.example.json` は既定正典として書き込まない。atomic write は config 用 temp+rename。
2. settings write で即時反映できるのは request 時に cfg を読む値だけ。port/bind・db・dataDir・dir root・schema/pragmas は restart required と UI で明示した。
3. settings write allowlist は `viewer.max_object_urls`、`hitomi` interval/retry、`image_format`、`use_live_gg`、`limits.*`、`virustotal.enabled`、`virustotal.hash_only`、`virustotal.upload_unknown` に限定。dir・port・bind・db・YARA path・api_key value・cap・floor・TTL・page constants は UI write 対象外。
4. settings write では floor guard を書き込み時に再実行し、不正値を拒否して表示する。path を UI で書ける形にはしない。
5. secret は UI へ api_key value として出さない。`api_key_env` 名だけ表示できる。Existing primary `config.json` の `api_key` は write 時に preserve する。
6. settings UI は `viewer.html` に置かない。`/settings` dedicated settings page を embed 追加する。Viewer page に settings panel を残さない。
7. R-15 の「viewer page に cap の通知 UI を置かない」条件は維持する。Dedicated settings page では settings values と constants を Japanese labels + spacing で表示する。Viewer page に cap notification UI を置くことは許さない。

## 実施（settings dedicated page）

- `GET /api/settings` は read-only settings response を返す。`api_key` value を返さず、`api_key_env` だけ返す。
- `GET /settings` で dedicated settings page を served する。
- Settings values/constants は Japanese labels、value、meaning、grid gap `.75rem 1.25rem` を使う。
- Dedicated settings page では `Browser memory cap` と `Disk cache cap` を settings/constant として表示できる。Viewer page は settings fetch、settings panel、`settings-values` を持たない。
- Settings API に `constants` を追加。Canonical freshness test に `app/settings.html` を追加。
- `api_key` value、YARA path は UI へ表示しない。

## 実施（settings editable write）

- `POST /api/settings` を dedicated settings page へ追加。`GET /api/settings` は `editable: true` と `editable_keys` を返す。
- Write allowlist は `viewer.max_object_urls`、`hitomi` interval/retry、`image_format`、`use_live_gg`、`limits.*`、`virustotal.enabled`、`virustotal.hash_only`、`virustotal.upload_unknown`。unknown top-level/settings key は reject。
- Integer fields は `json.Number` で decode し、floor/negative guard を validation 時に再実行する。Boolean fields は UI checkbox から bool で送る。`hitomi.image_format` は `webp` / `avif` / `jpg` / `png` に限定。
- Existing primary `config.json` を read して allowlisted field だけ update し、existing fields と `api_key` を preserve する。
- Config write は temp+rename の atomic helper。`config.example.json` を path に指定しても primary write target は同じ dir の `config.json`。
- Immediate memory update は successful write 後に `v.cfg` へ反映し、`v.client.cfg = v.cfg.Hitomi` で client request-time interval values を即時反映する。
- Settings page button は `設定を保存`、labels は Japanese name + original constant name を使う。

## 実施（cache settings and group card UI）

- `cache.cache_disk_max_entries` を settings group に追加し、allowlist へ含める。Value semantics は `0` = 未指定で canonical default `defaultCacheDiskMaxEntries = 4000`、explicit positive は `1..maxCacheDiskMaxEntries = 20000`。Negative と `20000` 超過は reject。
- Successful write 後は `v.cfg.Cache.CacheDiskMaxEntries` と `v.cacheDiskMaxEntries` を memory へ即時反映する。Settings API response の `cache` group から current memory value を返す。
- `viewer_floor` は settings 化しない。Runtime cap と lower-bound guard の性格が異なるため、`minViewerMaxObjectURLs = 1000` を定数正典として read-only constants group で表示する。
- Settings page は accessible group card を使う。Group は `fieldset` / `legend` で区切り、editable input は `input[data-key]`、label は `for` association で input に結び付け、note は `aria-describedby` で参照する。Settings page test anchor は `setAttribute("for", inputId)`、`aria-describedby`、`data-key`、`キャッシュ`、`Viewer memory`、`取得・rate`、`scan limits`、`VirusTotal`、`内部定数`。
- `config.example.json` に `cache` block を追加し、`cache_disk_max_entries` key を canonical default `4000` の写しとして列挙する。Existing `limits` test は `cache_disk_max_entries` を `limits` key として検出しない。
- Settings page で `viewer_floor` を `内部定数` の read-only row として表示し、`cache_disk_max_entries` は `キャッシュ` の editable row として表示する。Viewer page の settings panel、settings fetch、`settings-values`、`renderSettings()` 禁止条件は維持する。

## Supersedes

この decision の page 配置点だけ、同日の初期 viewer panel 案を supersede する。R-15 の viewer cap notification UI 禁止、secret 表示条件は維持する。`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` の cap notification UI 条件は維持する。`cache_disk_max_entries` を settings へ移動する点だけ、`docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md` の「cap・floor・TTL は settings 化しない」条件のうち cache cap 部分を supersede する。`viewer_floor` の settings 化は supersede しない。`docs/ai/decisions/2026-11-09-disk-cache-cap-count-4000.md` の canonical default 値 `4000` はそのまま。
