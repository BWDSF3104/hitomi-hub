# Web UI settings 変更設計の推奨承認（7決定点）

**Date**: 2026-11-09
**Status**: active

## Decision

Web UI settings は recommended form で承認した。現在の実装範囲は read-only display だけ。Future write allowlist はこの decision の条件を満たすときだけ実装できる。

1. future write target は primary `config.json`。`config.example.json` は既定正典として書き込まない。atomic write は config 用 temp+rename。
2. future write で即時反映できるのは request 時に cfg を読む値だけ。port/bind・db・dataDir・dir root・schema/pragmas は restart required と UI で明示する。
3. future write allowlist は `viewer.max_object_urls`、`hitomi` interval/retry、`image_format`、`use_live_gg`、`limits.*`、`virustotal.enabled`、`virustotal.hash_only`、`virustotal.upload_unknown` に限定。dir・port・bind・db・YARA path・api_key value・cap・floor・TTL・page constants は UI write 対象外。
4. future write では floor guard を書き込み時に再実行し、不正値を拒否して表示する。path を UI で書ける形にするなら `withinRoot` guard が前提。
5. secret は UI へ api_key value として出さない。`api_key_env` 名だけ表示できる。
6. settings UI は `viewer.html` に置かない。`/settings` dedicated settings page を embed 追加する。Viewer page に settings panel を残さない。
7. R-15 の「viewer page に cap の通知 UI を置かない」条件は維持する。Dedicated settings page では settings values と constants を read-only で Japanese labels + spacing で表示できる。Viewer page に cap notification UI を置くことは許さない。

## 実施（settings dedicated page）

- `GET /api/settings` は read-only settings response を返す。`api_key` value を返さず、`api_key_env` だけ返す。
- `GET /settings` で dedicated settings page を served する。
- Settings values/constants は Japanese labels、value、meaning、grid gap `.75rem 1.25rem` を使う。
- Dedicated settings page では `Browser memory cap` と `Disk cache cap` を settings/constant として表示できる。Viewer page は settings fetch、settings panel、`settings-values` を持たない。
- Settings API に `constants` を追加。Canonical freshness test に `app/settings.html` を追加。
- `api_key` value、YARA path は UI へ表示しない。

## Supersedes

この decision の page 配置点だけ、同日の初期 viewer panel 案を supersede する。R-15 の viewer cap notification UI 禁止、設定化範囲、secret 表示条件は維持する。`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` の cap notification UI 条件、`docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md` の設定化範囲、`docs/ai/decisions/2026-11-09-disk-cache-cap-count-4000.md` の disk cap 値を変えない。
