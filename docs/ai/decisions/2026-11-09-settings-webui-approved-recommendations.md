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
6. page 配置は existing `viewer.html` 内 settings panel。3 枚目 embed は増やさない。
7. R-15 の「cap の通知 UI を置かない」決定を settings panel の表示で壊さない。Settings API には machine-read 用 values を置けるが、UI settings row から cap を notification として表示しない。

## 実施（read-only display）

- `GET /api/settings` は read-only settings response を返す。`api_key` value を返さず、`api_key_env` だけ返す。
- `viewer.html` の settings panel は `/api/settings` を fetch して `settings-values` に表示する。
- settings row から `viewer.max_object_urls` を表示しない。これは R-15 no notification UI 条件の保護。
- settings panel は `viewer.html` 内 panel。3 枚目 embed を追加しない。

## Supersedes

既存 decision を supersede しない。`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` の cap notification UI 条件と `docs/ai/decisions/2026-11-09-settings-centralization-scope-approved.md` の設定化範囲の上に、実装形を追加する。
