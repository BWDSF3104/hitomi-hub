# 画面確認サイズは window size基準（2026-11-05）

**Status**: 承認済み（ユーザー指示 2026-11-05 07:47）
**Date**: 2026-11-05
**Scope**: hitomi-hub

## Supersede

`docs/ai/decisions/2026-11-02-strict-screen-confirmation-criteria.md` の「固定 target viewport `1280x900`」「exact inner target `1280x900`」という画面確認サイズ項目を supersede する。

fresh target、direct server baseline、actual UI event、network/DOM/visual一致、既存回帰チェックは維持する。

## 決定

1. 画面確認に固定目標サイズを設けない。
2. 基準は実際に表示している window sizeとする。
3. fresh targetで確認を開始し、人工的な resize で特定サイズへ合わせて合格判定しない。
4. 確認時に測った actual window size と content viewport を記録する。
5. その window size で要素が切り取られず、明示された固定寸法契約が保たれることを確認する。
6. 以後の screen confirmation は window size基準で行う。
