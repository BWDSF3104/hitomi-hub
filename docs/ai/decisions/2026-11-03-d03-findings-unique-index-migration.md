# D-03 findings unique index の key に path を含め、迁移は非破壊（定義置換のみ）

Date: 2026-11-03

**Related decisions**:

- `2026-11-03-d03-findings-unique-index-migration.md`（本エントリ。調査根拠は `docs/ai/tasks/2026-11-03-d03-findings-unique-path.md`）

**Decision**:

- findings の unique index の key は `(path, sha256, rule, detail)`。同一内容でも path が違えば別行として保持する（同一内容の別 path が first row のみになる挙動を解消）。
- unique index の定義は Python 参照実装（`schema.sql`）と Go（`ensureSchema`）で一致させる。Go 作成 DB にも unique index を作成する。
- 既存 DB への迁移は**非破壊**：index 定義の置換のみで行い、既存行の削除・改変（重複行の除去）をやらない。
- 既存 DB に新 key で重複行が存在して unique index を作成できない場合は、エラーとして拒否し、行削除や自動 dedup をしない。判断はユーザーへ報告する。
- schemaVersion を 4 へ上げる（定義差異の検出のため）。

**Supersede**:

- 本エントリは `app/main.go` の `migrateSchema` が行っていた暗黙 dedup（重複行の DELETE）挙動を supersede する。対応する既存 decisions エントリは存在せず、コード挙動としての記録のみが更新対象。

## Reason

findings は path ごとに意味を持つ（構造検査の結果はファイル単位の記録）。unique key に path を含まないと、同一内容の別 path の finding が失われ、実測で scan 2回分の重複行が発生する（Go 作成 DB の `PRAGMA index_list(findings)` に unique index が無い差異も実測済み）。既存行の削除はユーザーのデータの再構築を強いるため、定義置換に限定し、拒否条件を明示する。

## Non-goals

- 既存 DB の重複行を自動除去しない。
- `repro.db` 等の調査用 scratch DB を本程序で修正しない。
- UI・viewer・フル画像経路・cache の判定を変えない。

## Resolution（2026-11-03・ユーザー承認）

- 重複存在時は index 作成を拒否する方針（非破壊迁移）で確定。暗黙 dedup へは戻さない。
- 調査用 scratch DB `docs/ai/tmp/assess-20261103-1220/repro/root/repro.db`（rows=20・新 key 重複 group=5）は `kilo-safe-remove` でゴミ箱へ送った（判断ではなく掃除）。`d03_check/repro_copy.db` は失敗実測の証拠として保持。
- root `hitomi.db`（rows=8・旧 key・重複0）は今すぐの手入れ不要。次回 `ensureSchema` 実行で自然に揃う。揃ったことを canonical server 起動時に一度確認する。
