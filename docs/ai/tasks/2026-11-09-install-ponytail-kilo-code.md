# タスク：Kilo Code に Ponytail プラグインを導入する

**Status**: 完了
**Date**: 2026-11-09
**Origin**: remote received
**Source**: 2026-11-09-install-ponytail-kilo-code.md
**Received**: 2026-11-09

受領プロジェクト：Common_Operations

ユーザーのローカルPCで使っている Kilo Code のグローバル設定ファイル `C:\Users\fujim\.config\kilo\kilo.jsonc` に、Ponytail（https://github.com/DietrichGebert/ponytail ）のプラグインを導入してください。目的は、Kilo Code のエージェントに「既存コード・標準ライブラリ・プラットフォーム機能を先に当たる最小実装」のルールセットを毎ターン適用し、過剰実装を抑える効果を試すことです。変更対象は kilo.jsonc の `plugin` キーだけで、他の設定・コマンド・スキルは現状のまま使います。

## 前提（公式ドキュメントで確認済みの事実）

- Ponytail 公式の INSTALL.md および docs/agent-portability.md によると、Kilo Code は OpenCode 系のプラグインをそのまま実行でき、グローバル導入は `~/.config/kilo/kilo.jsonc` のトップレベルに `{ "plugin": ["@dietrichgebert/ponytail"] }` を加える形です。これによりルールセットが毎ターン注入され、`/ponytail` のレベル切替（lite／full／ultra／off）と `/ponytail-review` 等のコマンドが使えるようになります。Kilo CLI 7.8.3 で検証済みとされ、VS Code 拡張とはコアを共有すると公式に書かれています。
- Ponytail のデフォルトレベルは full で、追加の設定ファイルは不要です。本タスクではデフォルトのまま使います（`PONYTAIL_DEFAULT_MODE` 環境変数や `%APPDATA%\ponytail\config.json` は作成・変更しません）。
- プラグイン本体（npm パッケージ `@dietrichgebert/ponytail`）の取得は、次回 Kilo Code の新規セッション開始時にプラグイン機構が行います。本タスクでの実セッション起動は範囲外です。
- Drive には kilo.jsonc を対象とする先行タスク `2026-11-09-kilo-strata-sampling-settings.md` が発行済みです。本タスクは、受領時点の kilo.jsonc 現物を正として進めてください。サンプリング値が入っているかどうかに関わらず、既存の記述はすべて保持し、本タスクでは `plugin` キーの追加／マージだけを行います（サンプリング値の設定は本タスクの範囲外です）。

## 進め方

### 1. 保全

1. 受領したプロジェクトで、報告ファイルを作成する前に現状態を確認する。作業ツリーに変更がある場合は、そのファイルだけを明示的にステージして保全コミットする。他のセッション由来と見られる変更はそのまま残し、ファイル名の一覧を報告に書く。変更が無ければ、その旨を報告に書く。
2. kilo.jsonc のバックアップを同じフォルダに `kilo.jsonc.bak-20261109-ponytail` としてコピーする。

### 2. 現物と公式仕様の確認

1. kilo.jsonc の現物を読み、トップレベルのキー一覧と、既存の `plugin` キーの有無・内容を確認する。あわせて `$schema` の参照先を確認する。
2. Ponytail 公式の INSTALL.md（Kilo Code の項）と docs/agent-portability.md（Kilo Code の行）で、キー名が `plugin`（単数形）であることと、パッケージ名が `@dietrichgebert/ponytail` であることを確認する。
3. npm レジストリで `@dietrichgebert/ponytail` が実在することを読み取り専用で確認し、最新のバージョン番号を控える（例：`npm view @dietrichgebert/ponytail version`）。ネットワーク等の理由で確認できない場合は、その旨を報告に書いて手順3へ進む。
4. Kilo Code のバージョンが読み取り専用の方法（CLI の `--version` や拡張の表示等）で分かる場合は控える。分からない場合はその旨を報告に書く。

### 3. 編集

1. kilo.jsonc のトップレベルに `plugin` キーを用意し、`"@dietrichgebert/ponytail"` を要素として含める。既存の `plugin` キーがあって他のプラグインが登録されている場合は、既存の要素をすべて保持したまま末尾へ追加する。既に同じ要素が登録されている場合は、重複追加はせず、その旨を報告に書く（この場合は編集自体が不要だったものとして扱う）。
2. 編集は該当箇所への最小限の追記にとどめる。kilo.jsonc はコメントを含む JSONC のため、ファイル全体の再整形やコメントの削除は行わず、既存の記述と書式を保ったまま編集する。他のキー（provider、model、variants 等）には触れない。
3. 編集はファイルへの書き込みのみとし、実行中の Kilo Code／VS Code の再起動や再読み込みは行わない。設定は次に開く新規セッションから効く前提で、その旨を報告に書く。

### 4. 検証

1. 編集後の kilo.jsonc を読み直し、`plugin` キーの配置（トップレベル）と要素の綴りが手順2の公式仕様どおりであることを確認する。
2. JSONC として構文が正しいことを、コメントを除去したうえでのパース等、ファイルレベルの方法で確認する。
3. バックアップ `kilo.jsonc.bak-20261109-ponytail` との差分を取り、変更が `plugin` キーの追加／マージのみであることを確認する。

### 5. 差分の確認

- 変更対象ファイルの差分が必要分のみで、内容が正しいことを確認する。
- 受領したプロジェクト側の変更は報告ファイルの追加のみであることを、git status と差分で確認する。

## 報告

- 報告先：受領したプロジェクトの `reports/`
- 報告ファイル名：`2026-11-09-install-ponytail-kilo-code-report.md`
- 報告に含めるもの：
  - 完了した件数／総数と、すべて完了したかどうかの明示。総数は次の5件とする：(1) 保全（プロジェクト側の保全と kilo.jsonc のバックアップ）、(2) 現物と公式仕様の確認（既存 `plugin` キーの有無、キー名・パッケージ名の確定、npm 上の実在と最新バージョン、Kilo Code のバージョン）、(3) 編集、(4) 検証（読み直しと JSONC パースと差分）、(5) プロジェクト側の差分確認
  - 確認した公式ドキュメントの出典（INSTALL.md と docs/agent-portability.md の Kilo Code に関する記述）
  - 編集前後の `plugin` キーの内容（編集前にキーが無かった場合はその旨、既に登録済みだった場合はその旨）
  - npm で確認した `@dietrichgebert/ponytail` の最新バージョン（確認できなかった場合はその旨と理由）
  - Kilo Code のバージョン（確認できた場合）
  - バックアップのパスと、保全コミットのハッシュ（保全が不要だった場合はその旨）
  - 設定が効き始めるタイミング（次回新規セッションから。プラグイン本体の取得もその時に行われる）であることと、効き始めの目印（新規セッションで `/ponytail` と送ると現在のレベルが返ること）を1行で書く
  - 秘密情報（APIキー、トークン類）の値は含めない
