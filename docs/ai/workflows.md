# Workflows

## Git

- commit・push は検証完了時点で判断可。前提: 変更の詳細についてユーザーから承認を受けていること
- commit message: prefix は英語、説明は日本語
- GitHub: `BWDSF3104/hitomi-hub`（public・MIT）

## Commit Checklist

1. `python -m pytest tests/ -v`
2. `git status` / `git diff` で意図しない混入を確認（`*.db`・`tools/*.exe`・scratch/ は非追跡）
3. 単体 exe はコミットしない。版・digest は `tools/VERSIONS.md`
4. Memory Bank の変更はコード変更と同じ commit に含める
