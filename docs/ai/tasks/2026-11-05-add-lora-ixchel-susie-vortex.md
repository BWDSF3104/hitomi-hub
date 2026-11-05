# タスク：キャラLoRA 5件のダウンロード配置とワイルドカード登録（2026-11-05、11-06追加分を含む）

**Status**: 完了
**Date**: 2026-11-05
**Origin**: remote received
**Source**: 2026-11-05-add-lora-ixchel-susie-vortex.md
**Received**: 2026-11-06

ユーザーが採用を決めた Illustrious 用キャラ LoRA 5件について、(1) reForge の LoRA フォルダへダウンロード配置、(2) sd-wildcards への行追加、までを行うタスクです。生成確認は範囲外です（ファイル配置と検証までで完了扱い）。

Civitai のお気に入り登録は Muse 側で完了済みです（対象モデルはすべて登録済み）。Civitai 側の操作は不要です。

## 1. ダウンロードと配置

配置先：ユーザーの reForge（`G:\OhterApp\stable-diffusion-webui-reForge`）の `models\Lora`。既存の配置慣例に合わせ、迷う場合は直下に置いてください。

| # | キャラ | ダウンロードURL | 保存ファイル名 | 期待サイズ |
|---|---|---|---|---|
| 1 | Ixchel（イクシェル） | https://civitai.com/api/download/models/1508038 | `IVITS.safetensors` | 約48.6MB |
| 2 | Susie（DELTARUNE） | https://civitai.com/api/download/models/2396023 | `sus13_b1gd4d_ILL.safetensors` | 約217.9MB |
| 3 | Vortex（Helluva Boss） | https://civitai.com/api/download/models/3449205 | `Vortex_sdxl_3448936_epoch_10.safetensors` | 約109.1MB |
| 4 | Maid Marian（ディズニー版ロビン・フッド） | https://civitai.com/api/download/models/2581313 | `Marian_v18as98.safetensors` | 約218.4MB |
| 5 | Sekhmet（パルワールド） | https://civitai.com/api/download/models/3061187 | `Sekhmet_Palworld_IL.safetensors` | 約217.9MB |

- 保存ファイル名は上記の名前で統一してください（ワイルドカードの `<lora:...>` はこのファイル名で参照します）。
- リンク直ダウンロードで失敗する場合は、ユーザーから提供済みの Civitai API キーを使ってください。
- 配置後に各ファイルの実サイズを記録し、期待サイズと大きく違わないことと、safetensors としてヘッダが読めることを確認してください。

## 2. ワイルドカードへの行追加

対象はローカルの sd-wildcards クローン `G:\OhterApp\stable-diffusion-webui-reForge\extensions\sd-dynamic-prompts\wildcards` です。lbw は汎用プリセット `CHARA_MEGA_PRECISION` を使うのがユーザーの指定です。重みは既存のキャラ行の慣例どおり `1:1` です。

雌用 `furryAllnoobai_FemaleOnly.txt` の末尾に4行追加：

```
ixchel, IVITS, [[velociraptor, feathered dinosaur, green scales, feather crest, long tail, claws]] <lora:IVITS:1:1:lbw=CHARA_MEGA_PRECISION>
susie_\(deltarune\), SUSI3_B1Gd4d, [[anthro, scalie, purple scales, hair over eyes, sharp teeth]] <lora:sus13_b1gd4d_ILL:1:1:lbw=CHARA_MEGA_PRECISION>
maid marian, MarianC, [[fox, orange fur, brown eyes, brown nose, dress]] <lora:Marian_v18as98:1:1:lbw=CHARA_MEGA_PRECISION>
sekhmet_\(palworld\), Sekhmet_p4l, [[cat, egyptian, pal_\(species\)]] <lora:Sekhmet_Palworld_IL:1:1:lbw=CHARA_MEGA_PRECISION>
```

雄用（汎用） `furryAllnoobaiFavorite.txt` の末尾に2行追加：

```
vortex_\(helluva_boss\), V0rt3xHB, [[hellhound, canine, grey fur, red eyes, muscular]] <lora:Vortex_sdxl_3448936_epoch_10:1:1:lbw=CHARA_MEGA_PRECISION>
sekhmet_\(palworld\), Sekhmet_p4l, [[cat, egyptian, pal_\(species\)]] <lora:Sekhmet_Palworld_IL:1:1:lbw=CHARA_MEGA_PRECISION>
```

セクメトは多数個体がいる種族キャラで雌限定ではないため、雌用と汎用（雄用）の両ファイルに登録します。

## 作業手順の必須事項

- 変更を始める前に、sd-wildcards の現状態を git コミットして保全してください（対象ファイルだけを明示的にステージすること）。
- 行追加の後は、コミット前に `git diff` を確認し、差分が必要な行の追加のみになっていて、内容が正しいことを確認してください。
- 確認後に sd-wildcards リポジトリの通常運用に沿ってコミットとプッシュをしてください。

## 3. 報告

報告先：GitHub `BWDSF3104/kilo-commons-operations` の `reports/2026-11-05-add-lora-ixchel-susie-vortex-report.md`

報告内容：
- 導入完了した件数／総数（例：5/5）と、すべて完了したかどうかの明示
- 5ファイルのダウンロード結果（保存先パスと実サイズ、検証結果）
- 追加した行（ファイル名と行内容）と、差分確認（git diff が必要分のみで内容が正しいこと）の結果
- 事前コミットと追加後コミットのハッシュ
- 生成確認は範囲外のため未実施である旨
