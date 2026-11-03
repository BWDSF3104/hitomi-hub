# D-02: 既定の設定ファイルが追跡対象

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: D-02単独commit完了。決定ファイル `docs/ai/decisions/2026-11-03-d02-config-copyable-nontracked.md` を参照。実行計画の残項目はplanの次の実施順へ従う

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（方針）。既定 config path は `<data-dir>/config.example.json`（`app/main.go:713,732,739`）で repo 追跡下。`config.example.json` 自体は `api_key: ""`（秘密なし）だが、運用で同一 path に key を書くと追跡ファイルへ秘密が入る。`--config` フラグは存在（prior review の「missing --config」は解消済み）。対応: 既定 config を非追跡（`config.json` + `.gitignore`）へ移すか、example を追跡し実 config path を分離。合格条件: 実 key を置く path が追跡下でないこと。推定規模: trivial。

実装（ユーザー方針「非追跡かつ他の自端末へのコピーが簡単な方式」）:
- `.gitignore` へ `config.json` と `secrets/` を追加。
- `app/main.go` の既定config pathを `<data-dir>/config.json` へ変更。明示 `--config` は優先。`config.json` 不在時のみ `config.example.json` へfallback。
- `config.example.json` は key なしで追跡維持。key は `.gitignore` 済みの非追跡 `config.json`、または env `VIRUSTOTAL_API_KEY` へ置く。key文字列をcommit・記録・報告へ書かない。

実測:
- `git status --short` で `config.json`（実key path）は追跡対象にならない。`config.example.json` は追跡下だが `api_key: ""`。
- fallback確認: `config.json` 不在時 `resolveConfigPath` は `config.example.json` へ戻る。
- `config.json` 存在時、`resolveConfigPath` は `config.json` を優先し、`--config` は明示値を優先。

## Verification

- `app` で `go vet .` clean・`go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 41.711s`。追加固定テスト:
  - `TestResolveConfigPathPrefersNonTrackedConfigJSONAndFallsBackToExample`
  - `TestResolveConfigPathFallsBackToExampleWhenConfigJSONMissing`
- ルートで `python -m pytest tests/ -v` → 38 passed in 3.64s（`test_canonical_build_freshness.py` を含む。UI未変更のため server起動確認なし。canonical exeは再build済み）。
- canonical exe再build: `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` → canonical serverは開始時未稼働のまま。
- commitはD-02単独commit `ef2d038`（push済み: `.gitignore`、`app/main.go`、`app/main_test.go`、D-02記録、decision、`current-task.tsv`参照行削除）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
