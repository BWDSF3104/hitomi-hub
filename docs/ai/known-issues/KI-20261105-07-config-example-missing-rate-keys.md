# `config.example.json` にレートキーの欠落（コード既定と不一致）

**Status**: 解決（assess-20261105-1015 項目1の実装・検証 2026-11-05 10:25 UTC。archive への移動はタスク全体 close 時の Memory Bank 操作として親が実施）

## 症状

- `config.example.json` には `hitomi.min_interval_ms`（1000）と `hitomi.image_min_interval_ms`（250）しか無い。
- コード既定に存在する `hitomi.gg_force_min_interval_ms`（60000・`app/hitomi.go:222`）と `hitomi.bg_grace_ms`（300・`app/hitomi.go:350`）のキーが example に無い。
- 結果、example をコピーして config を作ると、両項目は「未指定＝既定値」に落ちるため実効値は同じだが、**編集可能な設定値として可視にならない**（ユーザーが触れる前に正典コード変更が必要に見える）。

## 想定原因

- Phase 4（2026-11-05）で `bg_grace_ms` を追加時、`config.example.json` の更新が追いついていない。`gg_force_min_interval_ms` は 2026-11-02 の high-priority fix（`decisions/2026-11-02-high-priority-fix-methods-and-acceptance.md`）追加時に example へ反映していない。

## 集約上の注意（同一数値の重複箇所）

- `defaultHitomiConfig()`（`app/hitomi.go:222`）が実行時の単一正典だが、`defaultBgGraceMS`（`app/hitomi.go:350`）と `defaultHitomiConfig()` 内リテラル `300`（`app/hitomi.go:223`）が二重定義。
- 受入条件テストがリテラルで既定値を固定: `app/hitomi_test.go:865`（1000 ms）、`app/hitomi_test.go:948`（250 ms）。方針写しは `api-rate-limits.md`・`usage-memo.md`・decisions 4 件・backlog items 4 件（2026-11-05 数え）。

## 次の対応

- example への 2 キー追加、および `defaultHitomiConfig()` を `defaultBgGraceMS` 参照へ単一化。修复には実装承認が必要（テストリテラルは方針変更と同時に re-anchor）。
