# 2026-11-07: R-14 実装採用設計（型 switch・条件付き SMask・DCTDecode 分岐）

**Status**: 実装完了（R-14・assess-20261106-1056）
**Date**: 2026-11-07

## 前提（正典矛盾の記録）

追加指示は `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` の R-14 記載を正典として採用するよう指示したが、その decision ファイルに R-14 の項目はない（R-11・R-15・R-20・R-05・R-08・R-13 のみ）。採用した根拠は (1) 追加指示そのもののユーザー承認（alpha 保持 `/SMask`・format 分岐 DCTDecode・型 switch）、(2) `docs/ai/tasks/2026-11-06-r14-pdf-raw-rgb-type-switch.md` の推奨案＋実測。古い「130ms 改善の記録（docs/ai/decisions/ 内）」は存在せず（本タスク記録 §5 参照）、130ms 数値は根拠に使っていない。

## 決定（承認済み範囲内の micro 決定）

1. 速度: `rawRGB` を型 switch で rawRGB バッファへ直接書き出す（推奨案採用・`draw.Draw` 案は A-11 gate 再導出が必要なので不採用）。concrete 型 `*image.NRGBA`・`*image.RGBA`・`*image.YCbCr`・`*image.NYCbCrA`。YCbCr 系は `YOffset`/`COffset` を pixel ごとに使う（subsample 正しい添字・直線計算禁止）。換算算式は `image/color` の `ycbcrToRGB` と同一（byte 一致をテストで確認）。既知 ratio 以外・bounds.Min 非原点・concrete 型以外は generic path（`At` 経由 premultiply）へ。
2. alpha: stored 値を raw RGB へ書き、stored alpha を別 buffer へ書き出す（premultiply 廃止）。`/SMask`（DeviceGray・Flate）追加 object は page 内に非 opaque alpha が 1 pixel でもある page だけ書く。条件付きにした理由: opaque のみの page は byte 形式が従来と完全に同一（既存 PDF テスト 3 件・A-11 gate の項目構成を壊さない。gate fixture は opaque）。alpha buffer は alpha page 分のみ（+P）。alpha 保持の表示意味変化（透過が黒でなくなる）はユーザー承認済み（追加指示 2026-11-07）。
3. format 分岐: `.jpg`/`.jpeg` entry は decode せず保存済みバイトを `/Filter /DCTDecode` stream へ embed（Flate raw RGB の約 20.6x）。page 辞書の `/MediaBox` に必要な size は JPEG header の最初の SOF marker から読む（decode なし）。magic 不正・SOF 不在は `undecodable` skip として既存セマンティクス維持。webp・png は decode+Flate 維持。AVIF は pure-Go decoder なしで skip 継続（非触）。
4. object 番号: page base（3+i*3）で連続。SMask を書く page のみ +1 ずれ、xref は object 番号順の map。`/Length` は stream 実バイト。byte 形式（`N 0 obj`・`stream`/`endstream`・xref 10桁・trailer）・page 辞書 `>>` 構文・skipped セマンティクス・`cfg.Limits.MaxEntries`・lazy create・page 順は不変。

## 影響

- 単色背景への合成案（white over）は採用しない（表示変化のため承認要・SMask 案で代替済み）。
- `ScanPDFKeys`・magic 照合（R-16 関連）は非触。UI 非触（本項目は UI 項目でない）。
- 実測はタスク記録 `docs/ai/tasks/2026-11-06-r14-pdf-raw-rgb-type-switch.md` の実装後再測欄（速度表・割当・pypdf 代替の content stream 解析）。
