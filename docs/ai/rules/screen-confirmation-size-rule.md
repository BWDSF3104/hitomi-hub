# 本プロジェクトルール: 画面確認サイズ

**Date**: 2026-11-05
**Status**: active
**Scope**: hitomi-hub

## Rule

- 固定目標サイズ（`1280x900` など）を画面確認の合格条件にしない。
- 画面確認の基準は実際に表示している window sizeとする。
- fresh targetで測った actual window sizeを基準にし、人工 resizeで特定サイズへ合わせて合格判定しない。
- actual window sizeと content viewportを記録する。
- その window sizeで要素が切り取られず、明示された固定寸法契約が保たれることを確認する。
- レイアウト確認は window sizeに合わせる。固定寸法契約を個別に要求する場合は、ルール、decision、taskに明示する。明示しない限り固定 target sizeで不合格にしない。
- 通常スクリーンショットの被写体、Playwright browser画面基準、例外的 OS full-screen 確認の扱い、例外理由の記録は `docs/ai/rules/normal-screenshot-rule.md` を維持する。
- このルールは `docs/ai/decisions/2026-11-05-window-size-based-screen-confirmation-rule.md` に基づく。
