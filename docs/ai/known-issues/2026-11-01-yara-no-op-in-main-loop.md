# main ループで YARA が実質 no-op

**Status**: 未解決（記録のみ・修復は本タスクの枝打たず）
**Date**: 2026-11-01

## 現象

`hub.exe`（および `src/scan.py`）の main ループは、構造検査で findings が無かったファイルにのみ `yr` を呼ぶ（`status != hit` が条件）。現行の `rules/structural.yar` の規則はすべて構造検査側の条件と重複しているため、YARA 単独で hit になるケースが存在しない。

実測（完成品フォルダ・fixture 4 件・`yr` 設定込み）: findings は `structural` の 3 件だけで、`yara` ルール行は 0。

## 影響

- 規則ファイルを実行エンジンの証明として呼ぶ経路は integration test で動いている（`run_yara` を直接呼ぶ）
- CLI 運用では YARA の追加検知がゼロ。規則を追加しても、構造検査が既に捕える条件なら同じ

## 対応案（未実施）

- YARA 側だけに意味のある条件（例: PDF の `/AA` に対する JS 実行内容、zip 内の `.url`/`.ini` 参照、magic と内容の非自明な不一致）を規則として追加し、構造検査側は最小化して役割を分担する
- または main ループで hit 判定前に YARA を走らせる（ただし hit 時の二重走査を防ぐ設計が必要）
