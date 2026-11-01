# hitomi-hub のテストは外部 API を呼べる

## 決定

`hitomi-hub` プロジェクトでは、global `AGENTS.md` の「テストスクリプトは外部 API を実際に呼び出してはならない」条を適用しない。

## 背景

P3 `#10` の検索速度検証では、nozomi、galleriesindex、gallery meta、image cache の実際の latency、gate wait、retry backoff、CDN time window を分離して確認する必要がある。mock だけでは gate wait と network time、cache hit/miss の壁時計を正確に分離できない。

## 運用条件

- hitomi.hub のレート制限を守る。
  - data/general calls: `1000 ms` interval
  - image calls: `250 ms` interval
  - retry backoff: existing policy
- VirusTotal public API は既存方針に従う。必要最小限で call count を抑える。テストで大量呼出しをしない。
- Secrets を test code に埋め込まない。
- 可能で、既存テストの安定性・再現性を保つ範囲では mock を維持する。
- 実呼出しテストは `docs/ai/tmp/` scratch scripts or explicit test mode で行う。

## 影響

- `docs/ai/api-rate-limits.md` の final rule を本決定へ更新。
- `#10` の実測検証で、mock-only ではなく live API measurement を許容する。
- 既存 mock tests は壊さない。