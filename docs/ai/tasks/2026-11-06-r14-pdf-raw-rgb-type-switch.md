# R-14: rawRGB が pixel ごと img.At で遅く、alpha を捨てて透過が黒になる

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 完了（型 switch・条件付き SMask・DCTDecode 分岐実装。テスト 3 件追加・既存 PDF 3 テストと A-11 gate 通過。commit 予定: app/pdf.go・app/export_test.go・decision 2026-11-07-r14）。pytest の test_canonical_build_freshness_rule_exists は既存 failure（KI-20261106-04・本項目と無関係）

## 意図 / 背景

`app/pdf.go:42-61` の `rawRGB` は decoder 出力を 1 pixel ずつ `img.At(x, y).RGBA()` で読み、`r, g, b, _` のうち alpha を捨てて raw RGB を書き出している。指摘は三点:

1. `img.At` 経由は interface 呼び出しと換算が pixel ごとに走り遅い（型 switch か `draw.Draw` でまとめられる）
2. alpha を捨てるため透過部分が黒く出る
3. Flate の raw RGB 保存は肥大化（JPEG は DCTDecode でそのまま embed できる）

memory（全 page 保持）は A-11 で解消済み（`docs/ai/tasks/2026-11-03-a11-pdf-raw-rgb-memory.md`）。本項目は速度・alpha・format 分岐の残存点。既存の読み取り記録は `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` R-14 欄（速度の実測値なし）。

## 調査結果 / 変更内容

### 現在の実装（読み取り）

- `rawRGB(img, buf)`: `buf` を page 間で再利用し、pixel ごとに `img.At(x, y).RGBA()` → `byte(r>>8)` 等。alpha は `_` で破棄（`app/pdf.go:53`）。
- `exportPDF` は page ごとの decode→rawRGB→zlib→file 直接書き出し（streaming）。image object は `/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode`（`app/pdf.go:203`）で、`/SMask` は書かない。
- `decodeStored` は decoder が持つ形式のみ（webp・png・jpeg）。AVIF は pure-Go decoder が無く skip（`app/pdf.go:22-38`）。

### 有効性: 確定（速度・alpha・肥大化の三点とも実測で確認）

実測はスクラッチ `docs/ai/tmp/assess-20261106-1056/` で行い、本番コード・実完成フォルダ・実 DB には書き込んでいない。

### 1. 速度（実測・3746x2525 = 9,458,650 pixel / raw RGB 28,375,950 B）

decoder 実際の返却型とそのままでの換算（1 run）:

| decoder 返却型 | 現行 `img.At` | 型 switch（pixel 直接） | `draw.Draw`+alpha strip |
|---|---|---|---|
| `*image.NRGBA`（png alpha・webp lossless） | 152.20 ms | 23.06 ms | 48.00 ms |
| `*image.NRGBA`（alpha 200） | 165.00 ms | 22.43 ms | 49.64 ms |
| `*image.RGBA` | 143.00 ms | 17.00 ms | 19.46 ms |
| `*image.YCbCr`（jpeg・webp lossy 4:2:0） | 219.93 ms | 93.06 ms | 57.06 ms |
| `*image.YCbCr`（manual 4:2:0） | 280.04 ms | 158.06 ms | 118.08 ms |
| `*image.NYCbCrA`（webp lossy alpha あり） | 298.01 ms | 180.00 ms | 199.80 ms |

- 現行は 1 run あたり 55.9〜63.1 MB を割当。fast path は 27.06 MB（= rawRGB 出力バッファ分のみ。pixel ごとの interface 割当が消える）。
- 1 page 全工程（decode→rawRGB→zlib）の実測: decode 73.51 ms + rawRGB（fast path）16.98 ms + zlib 23.00 ms = 113.49 ms。現行 `img.At` の rawRGB が 142〜150 ms なので、page あたり 130〜160 ms（約 60%）が速度改善幅。1300 page gallery では体感 320 s → 147 s（目安）。
- 型 switch の YCbCr は `COffset(x, y)` を pixel ごとに使う形が必須。行頭オフセット＋`+x` の直線計算は 4:2:0 で chroma 添字が倍速で進み、範囲外（panic）かつ換算誤差 maxdelta=255。subsample 正しい計算は `draw.Draw`（`internal/imageutil.DrawYCbCr`）が既に行っており、byte 一致（diff=0）。
- 現行の `img.At` は NRGBA 系で unpremultiply（alpha で乗算）、`*image.RGBA` では stored 値をそのまま返す。この非対称が下の alpha 問題と同一根拠。

### 2. alpha（実測・PDF content stream で黒化を示した）

pypdf はこの環境に未インストール（`python -m pip show pypdf` = not found）。追加指示どおり content stream 解析（`zlib` 展開）で示した。fixture 8x8（`docs/ai/tmp/assess-20261106-1056/fixture_alpha.png`）、source stored RGBA:

| pixel | source RGBA | 現行（alpha 捨て）stream | alpha 保持（`/SMask`）stream |
|---|---|---|---|
| (0,1) fully transparent | (40,70,80,0) | **(0,0,0) black=True** | RGB (40,70,80) + alpha 0 |
| (0,2) semi-transparent | (40,80,80,128) | (20,40,40)（50% 減光） | RGB (40,80,80) + alpha 128 |
| (1,3) opaque | (50,90,85,255) | (50,90,85) | (50,90,85) + alpha 255 |

白背景で描画したときの見え方（viewer 表示相当）: current = (0,0,0) / (20,40,40) / (50,90,85)、alpha 保持 = (254,254,254) / (147,167,167) / (50,90,85)。

- 黒化は「alpha を捨てる」だけでは説明不能。現行は `At().RGBA()` の premultiply を通すため、alpha=0 の pixel は stored RGB が非零でも 0 に寄る（上の (0,1)）。実 webp/PNG decoder は透過 pixel を 0 で持つことが多いので、結果はほぼ黒。
- alpha 保持案（DeviceRGB + `/SMask` DeviceGray Flate）は構造条件を保ったまま両 PDF で xref offset 実バイト一致・`/Length` = stream 実バイト・`endstream` 存在を確認済み。SMask は追加 object（fixture 1 page で +77 B、`/Length` と object 番号が増える）。

### 3. Flate raw RGB の肥大化（実測）

photographic 相当 fixture（3746x2525・滑らか勾配＋ノイズ・alpha なし）:

- raw RGB 28,375,950 B → FlateDecode stream 10,130,788 B
- PNG（Flate best）12,800,249 B
- JPEG（DCTDecode で embed 可能）490,998 B → Flate raw RGB は JPEG の **20.63 倍**
- worst case（乱噪声 page）: raw 28,375,950 B → Flate 28,378,123 B（圧縮 gain ゼロ）

DCTDecode 分岐が効くのは保存済みの `.jpg` entry のみ。既定 format の webp（nozomi shard URL）は DCT stream を持たず、decode 経由が必須。AVIF は decoder が無く skip 継続。

### 4. A-11 合格条件との照合

- `TestExportPDFStreamedPeakMemoryFitsSinglePageWorkingSet`（`app/export_test.go:242-`）の gate は page 1 分の working set（decoder image 4/3P + rawRGB P + zlib 結果上限 P + zlib writer internal）＋ header/metadata 1MB ＋ noise margin 8MB（gate 10,719,104 B・実測 peakDelta 3.1MB）。型 switch 案は rawRGB バッファへ直接書くので gate の項目構成は変わらない。
- `draw.Draw` 案は dst RGBA（4/3P）を另 に確保するため、gate の理論上界に項目が増える（gate fixture P=360,000 → +480,000 B）。gate 自体の再導出または合格条件の明示が必要。
- `cfg.Limits.MaxEntries`（`app/pdf.go:167`）と skipped セマンティクス、byte 形式、page 辞書 `>>` 構文、decodable 0 件で file 作成前 error（lazy create）は型 switch 案・SMask 案のどちらでも非触で維持できる。
- `pypdf` extractable 条件は既存テストに不在（`tests/` に pypdf 参照なし。既存の PDF 合格条件は `app/export_test.go` の byte マーカー・`/Count`・`/MediaBox`・xref 実バイト照合）。上記 content stream 解析で同等条件（xref offset 実バイト照合・`/Length` 一致・endstream）を代わりに確認した。

### 5. 正典参照の矛盾（記録）

追加指示が挙げる「`YCbCr` 130ms 改善の記録（`docs/ai/decisions/` 内）」は存在しない。`docs/ai/` を `YCbCr` と `130ms` で検索し、ヒットは以下のみ:

- `docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` R-14 欄（読み取りのみ・実測なし）
- `docs/ai/tasks/2026-11-06-r14-pdf-raw-rgb-type-switch.md`（本記録）
- `docs/ai/current-task.tsv` の本項目行

Flate/DCTDecode の既存分岐も decision に記録がない（`README.md:46`・`docs/ai/usage-memo.md:69`・`docs/ai/tasks/2026-11-01-nozomi-title-search-index.md` に「1 page 1 image・FlateDecode raw RGB」とあるのみ）。古い状態記述（「DCTDecode で embed すべき」とされた 2026-11-02 review findings の指摘）は未実施の指摘として残っている。採用した根拠は本タスクの実測値（速度・サイズ・alpha）で、130ms 数値は使っていない。

### 実施（2026-11-08・assess-20261106-1056）

追加指示の承認（alpha 保持 `/SMask`・format 分岐 DCTDecode・型 switch）に基づき `app/pdf.go` を実装。テスト 3 件追加（`app/export_test.go`）。

- 速度: `rawRGB` を型 switch 化（採用案どおり `draw.Draw` ではなく型 switch → A-11 gate の項目構成不変）。
  concrete 型（`*image.NRGBA`・`*image.RGBA`・`*image.YCbCr`・`*image.NYCbCrA`）は buffer へ直接書く。
  YCbCr 系は `YOffset`/`COffset` を pixel ごとに使う（subsample 正しい添字）。換算は `image/color` の
  `ycbcrToRGB` と同一算式（byte 一致）。既知 ratio 以外は generic path（`At` 換算）へ。
  `bounds.Min` が原点でない画像も generic path。
- alpha: fast path は stored 値を raw RGB へ書き、stored alpha を別 buffer（`abuf`）へ書き出す。
  micro 決定: `/SMask` object は page 内に非 opaque alpha が 1 つでもある page だけ書く
  （opaque のみの page は abuf を確保せず byte 形式が従来と同一 → A-11 gate fixture（opaque）の
  項目構成と既存 PDF テストの byte が不変）。alpha buffer は alpha page 分のみ（+P）。
  generic path（concrete 型以外）は従来動作（premultiply・alpha なし）維持。
- format 分岐: `.jpg`/`.jpeg` entry は decode せず保存済みバイトを `/Filter /DCTDecode` stream へ
  embed。size は JPEG header の最初の SOF marker から読む（decode なし）。magic 不正・SOF 不在は
  `undecodable` skip 維持。missing/undecodable/exceeded の skipped セマンティクス・MaxEntries・
  lazy create・page 順・byte 形式は非触。object 番号は page base（3+i*3）で連続、SMask を書く
  page のみ +1 ずれ（xref は object 番号順の map）。
- 正典矛盾（記録）: 追加指示が「正典として採用」と言う `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` に R-14 の記載はない（R-11・R-15・R-20・R-05・R-08・R-13 のみ）。
  採用した根拠: 追加指示そのもののユーザー承認（alpha 保持・format 分岐）＋本記録の推奨案＋実測。
  実装 micro 決定（条件付き SMask・SOF header による size）は `docs/ai/decisions/2026-11-07-r14-type-switch-smask-dct-implementation.md` に記録。

### 推奨（複数案中の採用案）

1. 速度: `rawRGB` を型 switch で rawRGB バッファへ直接書く。YCbCr 系は subsample ratio を尊重する添字計算（`draw.Draw` ではなく `YOffset`/`COffset` を pixel ごとに使う）、または `draw.Draw`（byte 一致）だが memory gate の再導出が必要。A-11 gate を壊さない意味で **型 switch を推奨**（NRGBA・RGBA・YCbCr・NYCbCrA の 4 型で byte 一致・diff=0 を確認済み）。
2. alpha: `At().RGBA()` の premultiply をやめ、stored 値を使い `/SMask`（DeviceGray・Flate）で alpha を separate stream として書く。黒化をなくす唯一の形。単色背景へ合成する案（white over）は「透過は白」になる表示変化なので、UX 変化として承認が要る。
3. format 分岐: `.jpg` entry は decode せず `DCTDecode` で stream embed（20.6x の縮小）。webp/png は decode+Flate 維持。分岐は既存の skipped セマンティクス（undecodable）と `pypdf`/xref 条件を壊さない形で object 番号と `/Length` のみ変更。

### 副作用・影響範囲

- alpha 保持は PDF 表示の意味を変える（透過が黒でなくなる）。ユーザーに見える出力変化 → 承認が必要。既存テストは pixel 値を固定していない（`TestExportPDFWritesPagesAndSkipsUndecodable` は byte マーカーのみ）のでテスト変更は必要だが、意味変更の承認と分離すべき。
- DCTDecode 分岐は `decodeStored` を bypass する。`ScanPDFKeys`・magic 照合（R-16 関連）には影響しない（対象ファイルは gallery 保存画像）。出力サイズが page 単位で大きく下がる。
- `draw.Draw` 案は working set に RGBA dst を追加（A-11 gate 再導出）。型 switch 案は追加なし。
- 既存の page 順・entry 名・xref 物理順（`/Pages` 末尾）は不変。UI は非触（本項目は UI 項目でない → 画面確認 該当なし・未確認）。

### 推定規模

小〜中。`app/pdf.go` 内 `rawRGB` の型 switch 化（数十行）＋ alpha/SMask 分岐（object 番号と `/Length` の扱い）＋ format 分岐（JPEG 分岐）。テスト追加 3 件（pixel 値一致・SMask 構造・JPEG DCT embed）。既存 PDF テスト 3 件と A-11 gate は維持。

## Verification

### 実装後再測（2026-11-08・実装代码に対する 1 run 値）

スクラッチフォルダは 7 日規則で消えていたため `docs/ai/tmp/assess-20261106-1056/` を再作成（`bench.go`＋`go.mod`・stdlib のみ・本番コード・実完成フォルダ・実 DB へ書き込みなし）し、同じ fixture 定義（3746x2525・勾配＋ノイズ）で再測。

- 型 switch（実装後）vs `At` 経由（1 run・同 fixture）:
  - `*image.NRGBA`（opaque）: At 170.57 ms / fast 26.00 ms
  - `*image.NRGBA`（alpha 200）: At 156.56 ms / fast 27.00 ms
  - `*image.RGBA`: At 143.51 ms / fast 22.00 ms
  - `*image.YCbCr`（4:2:0）: At 283.15 ms / fast 138.00 ms
  - `*image.NYCbCrA`（alpha）: At 295.00 ms / fast 135.99 ms
- 割当: `At` 経由 58.6〜66.2 MB（= 55.9〜63.1 MiB の記録範囲）→ fast path 28.38 MB（= rawRGB 出力バッファ P 分のみ・27.06 MiB）。pixel ごとの interface 割当が消えることを再確認。
- 1 page 全工程（png fixture・decode→rawRGB→zlib・1 run）: fast path 906.99 ms（decode 284.52 + rawRGB 26.00 + zlib 596.46）vs At path 1016.60 ms → page あたり 109.6 ms 改善（この fixture・zlib 支配）。rawRGB 単体の改善幅（型別）: 121.5〜159.0 ms/page（基準 130〜160 ms/page と YCbCr 145.2・NYCbCrA 159.0・NRGBA 144.6 で一致、RGBA 121.5 は同一数量級・1 run 揺れ）。1300 page gallery 換算（rawRGB 分のみ・この fixture 1 run の ms 値）: 221.7 s（At path）→ 33.8 s（fast path）。
- size cmp（photographic fixture・jpeg q90）: Flate raw RGB 24,348,480 B vs JPEG(DCT) 1,170,655 B = 20.80x（記録 20.63x 相当）。noise page worst case: raw 28,375,950 B → Flate 28,378,123 B（記録と一致・gain なし）。
- 実装テスト（追加 3 件）:
  - `TestExportPDFRawRGBFastPathPixelParity` — fast path と `At()` 換算の byte 一致（opaque・6 ratio・odd size 37x23）。alpha fixture の stored 値一致（(0,1)=(40,70,80,0) が黒化しない、At() 参照では 0 に寄ることを両方向で確認）。通過。
  - `TestExportPDFWritesSMaskForNonOpaqueAlpha` — content stream 展開で pixel (0,1) が stored RGB (40,70,80)＋alpha 0、(0,2) が (40,80,80)+128、(1,3) が (50,90,85)+255。`/SMask` object（DeviceGray・Flate）存在・`/Length` = stream 実バイト・`endstream` 存在・xref 各 entry が実バイト上の object を指す（xref size 7 = object 0..6）。通過。
  - `TestExportPDFEmbedsJpegAsDCTDecode` — valid JPEG（5x7）が `/Filter /DCTDecode`・stream bytes = 保存済みファイルバイト一致・`/Length` 一致・`endstream` 存在。invalid `.jpg` は `undecodable` skip（pages 1・skipped 1）。通過。
- `app` `go vet . && go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 24.463s`（全通過）。既存 PDF 3 テスト（opaque fixture → byte 形式・object 番号不変なので SMask 非発火）と A-11 gate（`TestExportPDFStreamedPeakMemoryFitsSinglePageWorkingSet`）通過。fast path は opaque page で追加割当なし（gate 項目構成不変）。
- ルート `python -m pytest tests/ -v` → 56 passed / 1 failed（12.43s・collect 57）。不合格は開始時点からの既存 failure `test_canonical_build_freshness_rule_exists`（KI-20261106-04・本項目と無関係）。collect 57（記録時の 55）は pytest 側の collect 増で本項目の変更ではない。`test_canonical_executable_embeds_current_pages` PASSED（UI 非触）。
- UI 画面確認: 該当なし（本項目は UI 項目でない）。Drive 保存なし。
- 修正・再検証の試行（実装 1 回。不合格はテスト側 2 試行・実装バグの検出なし）:
  1. 新テスト失敗: 8x8 fixture の pixel index を x 優先で計算（正 j = y*w + x）＋ SMask テストで fixture file 書き出し漏れ（missing skip → error）。原因はテスト側。修正 → 該当テスト通過。
  2. 新テスト失敗: xref 解析が object 0 の `f` entry（20 B）を object 1 と読んで offset 0。原因はテスト側。修正（f line skip）→ 通過。以後全通過。

## 完了確認

- Commit: `65f51e3`（push 済み・base `bb6efba`）。変更: `app/pdf.go`・`app/export_test.go`・decision `2026-11-07-r14-type-switch-smask-dct-implementation.md`・本記録・`current-task.tsv`（参照行削除）。
- 段階3 持ち帰り: なし。alpha 保持（表示意味変化）と DCTDecode 分岐は追加指示で承認済み。micro 決定（条件付き SMask・SOF header で size・generic path の従来動作維持）は既存テスト・gate を壊さない範囲の選択で、正典 decision（2026-11-07 新規）に記録。
- 正典矛盾: 追加指示が挙げる decision ファイルの R-14 記載は不存在（採用根拠: 追加指示のユーザー承認＋本記録の推奨案＋実測）。記録済み。
- 既存 PDF 3 テストは opaque fixture のため SMask 非発火・byte 同一で通過（意味変更の承認範囲と分離確認）。

### 実測（スクラッチ `docs/ai/tmp/assess-20261106-1056/`・本番コード・実完成フォルダ・実 DB へ書き込みなし）:
  - `go run ./bench`（fixture `fixture_opaque.png`・`fixture_opaque.jpg`・`fixture_alpha.png`・`fixture_alpha.jpg`）→ 上記速度表。p50 ではなく 1 run 値。
  - `go run ./alphapdf` → `current_no_alpha.pdf`（alpha 捨て）と `alpha_kept_smask.pdf`（SMask）を scratch へ生成。
  - `python parse_pdf.py` → content stream 展開で現行 PDF の (0,1) が `(0,0,0)`（black=True）を示し、alpha 保持 PDF は RGB (40,70,80) + alpha 0。両 PDF で xref offset 実バイト一致・`/Length` 一致・`endstream` 存在。
  - `go run ./sizecmp` → Flate raw RGB 10,130,788 B / JPEG 490,998 B（20.63x）、noise page で gain なし。
- pypdf は未インストール（`pip show pypdf` = not found）。追加指示の代替（content stream 解析）で確認。
- `app` `go vet . && go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 26.322s`（全通過。ソースは変更していないので既存合格条件はそのまま成立）。
- ルート `python -m pytest tests/ -v` → 54 passed / 1 failed（11.33s）。不合格は `tests/test_canonical_build_freshness.py::test_canonical_build_freshness_rule_exists`（AGENTS.md が `docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md` へのポインタを持たない）。本項目・本セッションの変更ではなく開始時点からの既存不合格（R-14 と無関係）。正典 rules の R-14 合格条件（streaming・max entries・pypdf extractable 相当）には影響しない。
- UI 画面確認: 該当なし（本項目は UI 項目でない）。Drive 保存なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
