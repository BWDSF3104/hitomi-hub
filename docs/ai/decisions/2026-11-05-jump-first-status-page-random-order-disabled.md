# jump-to-first-status-page: random order では機能無効（2026-11-05）

**Status**: 確定（ユーザー 2026-11-05 07:59）
**Date**: 2026-11-05
**対象タスク**: `docs/ai/tasks/2026-11-05-jump-to-first-status-page.md`

## 決定

`orderbydir=random`（`sortby:random`）の検索では「最初のステータス付きページへ移動」を**無効**とする。

## 根拠

- shuffle は呼出ごとに変わります（`app/searchhtml.go:247-248`）。`copyInts` で cache 本体は不変だが、表示順と cache 順序が食い違うため、page 番号が表示と一致しない。
- cache 順序固定（rand order cache）は複雑度上昇の割に再現性の価値がない。

## 挙動（既定案）

- UI: random order のとき button を disabled 表示（click 不可）。
- CLI: random が含まれる query では error（`random order is not supported`）。
