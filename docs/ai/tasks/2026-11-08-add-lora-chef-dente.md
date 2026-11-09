# Civitai LoRA 1件のダウンロード配置とワイルドカード登録（シェフ・ダンテ）

**Status**: 完了
**Date**: 2026-11-08
**Origin**: remote received
**Source**: 2026-11-08-add-lora-chef-dente.md
**Received**: 2026-11-09

受領プロジェクト：Common_Operations

## 作業内容

Civitai から LoRA 1件をダウンロードし、指定のファイル名で配置する。あわせてワイルドカードファイルへ登録行を1行追加する。

### 対象LoRA

- キャラ：シェフ・ダンテ（Chef Dente／ヨクバリス、ぽこ あ ポケモン）
- ダウンロードURL：https://civitai.com/api/download/models/3183682
- 配置ファイル名：`ChefDenteIL.safetensors`
- 配置先：`G:\OhterApp\stable-diffusion-webui-reForge\models\Lora`（直下配置）

### ダウンロード方法

- Civitai APIキーは環境変数 `$env:CIVITAI_API_TOKEN` を `Authorization: Bearer` ヘッダーで使う。
- キーの値の表示・出力・報告への記載はしない。詳細ログ（curl -v 等）は付けない。
- 変数が未設定・空の場合は、その旨を報告して止まる。

### ワイルドカード登録

追加先ファイル：`furryAllnoobai_FemaleOnly.txt`（雌用）
追加する行（このまま1行追加する）：

```
chef dente, CHEF_DENTE, [[squirrel, semi-anthro, fat, plump, stumpy limbs, short stature, brown fur, two-tone fur, buck tooth, orange nose, fluffy tail, big tail]] <lora:ChefDenteIL:1:1:lbw=CHARA_MEGA_PRECISION>
```

- ワイルドカードのクローンは `G:\OhterApp\stable-diffusion-webui-reForge\extensions\sd-dynamic-prompts\wildcards` で、このフォルダ自体が独自の git 管理（親の extensions\sd-dynamic-prompts は拡張機能の公式クローンで、その中に入れ子になっている）。git 操作は wildcards フォルダの中でそのフォルダ自身の管理として完結させる。親フォルダ側の git 操作と入れ子関係の変更は範囲外。

## 共通の進め方

1. 変更前に現状態を git コミットして保全する（対象ファイルだけを明示的にステージし、他の未コミット差分を巻き込まない）。
2. 変更後は、対象ファイルの差分が必要分のみで内容が正しいことを確認する。
3. コミット／プッシュは自身の判断で実行してよい。

## 報告

- 報告先：受領したプロジェクトの `reports/`、ファイル名は `2026-11-08-add-lora-chef-dente-report.md`。
- 報告には「完了した件数／総数」と「すべて完了したかどうか」を表示する。
