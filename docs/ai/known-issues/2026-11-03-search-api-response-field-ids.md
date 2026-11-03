# 検索 API 応答の配列フィールドは `ids`（`results` ではない）

**Date**: 2026-11-03
**Status**: 観察（実装変更なし・低優先）
**Backlog**: assess-20261103-1220

## 現象

- 調査・UI確認スクリプト側で `/api/search` の応答を `results` で読むと 0 件と誤読する事例があった（B-03 実操作の途中で発生）。
- 正しくは `ids`。実操作（live 検索）で検索が動作することを確認済み（`female:collar` lang=japanese page 1 に hit があり、card 経由で gallery を表示できる）。

## 対応方針

- code 変更はしない。今後の調査スクリプト・画面確認スクリプトは検索応答を `ids` で読む。
- 誤読を修復に枝打たず、この記録で留める（B-03 の合格判定は実操作の実測に基づくため影響なし）。

## 補足

- `artist:suji_niku` の nozomi は 404。page 1 に hit がある語を使う必要がある（B-03 では `female:collar` を使用）。hitomi 側の窓依存であり app の缺陷ではない。
