# C-04 `language:` term の single-fetch semantics

Date: 2026-11-03

**Related decisions**:

- `2026-11-01-hitomi-client-endpoints.md`（nozomiPath の分区規則: `language:` は lang に反映、tag は index）
- `2026-11-02-high-priority-fix-methods-and-acceptance.md`

**Decision**:

- `termIDs` の `language:` 分岐は state の area/tag を新 language で再 fetch しない。
- `language:` term の dataset は `index-<lang>`（自前 dataset）のみ。同じ area/tag の language 違い fetch は 1 回に固定する。
- `SearchHTML` は fetch 前に positive term の language term を pre-scan し、`state.language` を最初の term fetch へ反映する。
- pre-scan 反映済みの language term（positive の 2 項目以降）は dataset redundant（実 hitomi data の subset 保証で intersection no-op）なので fetch しない。
- negative・or-group の language term は dataset 寄与が必要なので termIDs 経由（own `index-<lang>` dataset）を維持する。

## Reason

実 hitomi data では language-scoped dataset（例 `female:yuri-english`）は all dataset（`female:yuri-all`）の subset で、前置 term の intersection に `index-<lang>` を加える効果は language filter そのもの（results.js 相当）。旧実装は前置 term で fetch 済みの同じ area/tag を新 language で再 fetch していた（`female:yuri language:english` で nozomi calls 1 → 3）ため、language term 自体の fetch を 0（first fetch へ反映）または own index dataset 1 回へ減らす。

## Edge semantics（実 data の subset 保証が前提）

- positive に複数の異なる language term がある場合: 最初が採用（first-wins）。旧実装は言語間 intersection で実 data では ∅。pathological な queries の差異として未判断（テストなし）。
- `language:` の右辺が空: pre-scan では ignore、term は dataset を寄与しない。
- mock test の fixture は実 data の subset 規則に従う（fixed test `TestSearchHTMLColonTermUsesNozomiState`）。

## Non-goals

- `SearchNozomiParts` の fetch semantics・rate gate・cache rules（TTL/LRU/size cap）を変えない。
- singleflight・negative cache、protected full image rules、TN rules を変えない。
- search order・intersection/exclusion/union semantics（language 以外の term）を変えない。
