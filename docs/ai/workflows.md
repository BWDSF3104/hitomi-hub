# Workflows

## Git

- commit・push は検証完了時点で判断可（自己判断による実施を許可・事前承認不要）。Commit Checklist の検証は実施の前提として維持
- commit message: prefix は英語、説明は日本語
- GitHub: `BWDSF3104/hitomi-hub`（public・MIT）

## Commit Checklist

1. `python -m pytest tests/ -v` must pass, including `tests/test_canonical_build_freshness.py`.
2. If `app/` files changed, run `go vet . && go test -count=1 -timeout 300s .` in `app`.
3. If UI files changed, rebuild canonical `output/hitomi-hub/hub.exe` and confirm served UI/network response from the canonical server. Source `ui.html` string tests alone are not enough.
4. `git status` / `git diff` で意図しない混入を確認（`*.db`・`tools/*.exe`・scratch/ は非追跡）
5. 単体 exe はコミットしない。版・digest は `tools/VERSIONS.md`
6. Memory Bank の変更はコード変更と同じ commit に含める
