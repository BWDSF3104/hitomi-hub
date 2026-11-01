# Evaluate P3 backlog items

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: Evaluate #10-#18, write per-item reference files under docs/ai/backlog-items, link from backlog

## 意図 / 背景

P3 backlog の `#10` から `#18` について、各項目の実現可能性、制限事項、推奨実装案、その他の実装案を項目ごとに検討し、`docs/ai/backlog-items/` の別ファイルへ記録する。本作業は記録・検討のみで、実装は行わない。

## 調査結果 / 変更内容

Per-item reference files are recorded under `docs/ai/backlog-items/`. Each file includes current behavior, feasibility, constraints, recommended implementation, alternative implementation, and test plan when useful.

Adoption decisions were recorded in `docs/ai/decisions/2026-11-01-p3-implementation-decisions.md`.

Adopted choices:

- `#10`: recommended measurement layer, phase timing, `performance.now()`, cache-less/cache-warm comparison.
- `#11`: gallery/viewer position display only. Search result side is out of scope.
- `#12`: server-side cache collect/bundle inside runtime root. Browser arbitrary-path download is not adopted.
- `#13`: UI-side prev/next search page buttons.
- `#14`: fullscreen state separated from layout mode.
- `#15`: view mode buttons with `#view-state`.
- `#16`: update `document.title` in gallery meta path.
- `#17`: low-risk anchor `同名の日本語版: #id` on display card. Full dedup is not adopted.
- `#18`: default RTL display order with toggle button, logical pair/route unchanged.

External API test rule decision:

- `hitomi-hub` does not apply the global “tests must not call external APIs” rule.
- Live API tests/measurement are allowed in `hitomi-hub` when rate limits are respected.

## Verification

- No project code changes are expected.
- `go test .` and `go vet .` are not required unless code changes are made.
- Documentation files created/modified:
  - `docs/ai/backlog.md`
  - `docs/ai/backlog-items/p3-10-search-speed-measurement.md`
  - `docs/ai/backlog-items/p3-11-position-display.md`
  - `docs/ai/backlog-items/p3-12-cache-collect-download.md`
  - `docs/ai/backlog-items/p3-13-search-prev-next-buttons.md`
  - `docs/ai/backlog-items/p3-14-fullscreen.md`
  - `docs/ai/backlog-items/p3-15-view-mode-buttons.md`
  - `docs/ai/backlog-items/p3-16-tab-title-current-work.md`
  - `docs/ai/backlog-items/p3-17-same-title-japanese-filter.md`
  - `docs/ai/backlog-items/p3-18-spread-rtl-toggle.md`
  - `docs/ai/decisions/2026-11-01-p3-implementation-decisions.md`
  - `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`
  - `docs/ai/api-rate-limits.md`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
