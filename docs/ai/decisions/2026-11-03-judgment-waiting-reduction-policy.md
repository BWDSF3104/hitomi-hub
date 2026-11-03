# 判断待ちを減らす今後の運用方針

**Status**: 決定
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220

## 決定

- これまでの最終判断は、ユーザーの明示指示がある場合を除き、推奨案を default として採用する傾向にある。
- 今後の作業では、判断待ちを次の3段階へ分ける。
  1. **defaultで進めてよい項目**: 推奨案と実測根拠があれば単独 commit/push へ進める。
  2. **実測で確定する項目**: hitomi convention、UI actual confirmation、live API response は未検証として扱い、判断待ちではなく実測で閉じる。
  3. **ユーザー承認が必要で残す項目**: UX意味変更、外部API policy、不可逆 migration、正典rule変更、hitomi convention未確認領域だけ質問する。

## defaultで進めてよい領域

- Python ↔ Go parity修正:
  - B-06: `empty results + positives 0` → `not_evaluated`。
  - B-07: YARA errorを構造passの `scan_status` へ上書きしない。`scan_source` の `structural+yara_error` で retry 用表現として維持する。
- docs drift修正:
  - D-06: faithful parse、TN古記述、check説明、README UI記述を実物確認後に正典決定へ寄せる。
- test-only最適化:
  - C-10: test-only gate interval inject。本番 gate interval 1000msは変更しない。
- cache TTL / bounded cache の保守的実装:
  - C-05 cache limits、TTL、bounded memory。正典 cache pathや full image rulesを変えない限り default で進める。
- UI debounce / generation guardの機械的統一:
  - C-06 strip rebuild guard。
  - C-07 resize generation/debounce。
  - C-09 read/lists debounce/guard。
  - stale async writeを防ぐため generation guardを優先する。
- resource safety / ctx-aware化:
  - A-06 `time.Sleep` ctx-aware化。
  - A-09 Range/304/Content-Range safe implementation。
  - A-13 string key allocation回避。
  - A-12 SHA256全件計算の最適化。
- `.gitignore`、canonical test premise固定、D-01既存方針の反映。

## 判断待ちとして残す領域

- ユーザーに見えるUX意味変更:
  - nav Enter意味。
  - newTabMode。
  - acquisition overwrite確認。
  - title表示、fullscreen/view mode、button disabled semantics。
- 外部API policy / key扱い:
  - 本番 gate interval。
  - hitomi endpoint convention。
  - VirusTotal API key handling。
  - API call count増加の副作用。
- 非破壊 / 不可逆 data migration:
  - DB index migration。
  - duplicate data removal。
  - file deletion。
- 正典ruleそのものの変更:
  - full image proxy順序。
  - persistent cache path。
  - TN display源禁止。
  - TN display fallback禁止。
  - TN bytesをfull actual pixelsに数えない規則。
  - `x-cache`, `no-store`, objectURL表示, generation guard。
- hitomi convention未確認領域:
  - A-04。
  - D-09。
  - D-10。

## 運用

- 実施順の次の項目は、上記 default領域に含まれる場合は推奨案と実測で単独 commit/push へ進める。
- 判断項目として残すのは、上記「判断待ちとして残す領域」に該当する場合だけ。
- 記録は `docs/ai/decisions/` または個別タスクの `調査結果 / 変更内容` / `Verification` へ反映する。
- `Next Action` は単一行の次 action だけ書き、実測・窓依存・長文は本文へ移す。
