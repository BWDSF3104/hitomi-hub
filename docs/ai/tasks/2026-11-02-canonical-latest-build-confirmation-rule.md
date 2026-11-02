# canonical latest build confirmation test rule

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: create the project test rule and run it against the canonical executable

## 意図 / 背景

UI 確認が stale binary で行われる可能性があった。`app/ui.html` の embedded source string test が通っても、実際に `http://127.0.0.1:8731/` で served される UI は現在の `hub.exe` が返す embedded bytes である。したがって、canonical exe が現在の source と一致しない場合、UI confirmation は invalid。

## 調査結果 / 変更内容

- Previous canonical server `pid 29096` was stale:
  - Canonical exe `LastWriteTime` was `2026/11/02 18:19`.
  - Implementation commit `8288151 fix: apply external review fixes` was later.
  - Served `renderRetryButtonState()` lacked `if (Number(counts.noFullDone) > 0) button.disabled = false;`.
- Added `tests/test_canonical_build_freshness.py`.
  - Builds a scratch executable from current `app`.
  - Asserts the scratch executable embeds current `app/ui.html` bytes.
  - Asserts canonical `output/hitomi-hub/hub.exe` also contains the exact current `app/ui.html` bytes.
- Added project rule to `AGENTS.md` and `docs/ai/workflows.md`.

## Verification

Initial test-first run:
- `python -m pytest tests -v`
  - `test_canonical_build_freshness_rule_exists` failed because the rule was not documented.
  - `test_canonical_executable_embeds_current_ui` failed because canonical `hub.exe` was stale.

After rule documentation, stopping stale server, and canonical rebuild:
- `python -m pytest tests -v`
  - Result: `25 passed`
- `go vet . && go test -count=1 -timeout 300s .` in `app`
  - Result: `ok hitomi-hub/app 34.671s`

Canonical rebuild and served UI confirmation:
- Rebuilt:
  ```text
  go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .
  ```
- Restarted canonical server `pid 17980` from the rebuilt exe.
- Direct served UI from `http://127.0.0.1:8731/` now contains:
  ```js
  if (Number(counts.noFullDone) > 0) button.disabled = false;
  ```

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
