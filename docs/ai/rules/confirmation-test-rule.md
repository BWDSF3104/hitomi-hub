# 本プロジェクトルール: 確認テスト系正典（単一ルール）

**Date**: 2026-11-09
**Status**: active（ユーザー指示 2026-11-06 08:13 UTC / 2026-11-09 ユーザー指示でブラウザ開閉ルールを追記）
**Scope**: hitomi-hub

テスト系規則は本ファイルに集約する。AGENTS.md の Rules ポインタは本ファイル1本のみ。
**Supersede 対象（内容は本ファイルへ移動）**：`docs/ai/rules/normal-screenshot-rule.md`、`docs/ai/rules/screen-confirmation-size-rule.md`、`docs/ai/rules/cold-confirmation-hygiene-rule.md`、`docs/ai/rules/before-after-screenshot-and-drive-cleanup-rule.md`。歴史的記録（task・decision・known-issues）内の旧ファイル名参照は書き換えない。

## 1. Canonical build / served confirmation

- UI 確認は freshly rebuilt canonical `hub.exe` が served する UI/network 応答に対して行う。`app/*.html` の source string テストだけでは不合格。
- `python -m pytest tests/ -v` が通ること（`tests/test_canonical_build_freshness.py` を含む）。
- UI 変更時は canonical server を stop → canonical build → 同じ exe から server を起動して served 画面を確認する。
- 正典 build command と server command の記録: `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`（AGENTS.md ポインタ化の記録: `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`）。

## 2. 画面確認サイズ

- 固定目標サイズ（`1280x900` など）を合格条件にしない。基準は実際に表示している window size。
- fresh target で測った actual window size と content viewport を記録する。人工 resize で特定サイズへ合わせて合格判定しない（resize は bug 証明にのみ使用可）。
- その window size で要素が切り取られず、明示された固定寸法契約（例 `--search-card-height: 130px`）が保たれることを確認する。固定寸法を個別に要求する場合は rule・decision・task に明示する。

## 3. Cold 確認 hygiene

cold state を claim する UI/network 実測は、測定時点で次の3条件を満たすこと。

1. **Canonical build freshness**: 現行 `app` からの fresh canonical `hub.exe`（freshness test 合格）起動 server で測定
2. **Fresh server instance**: in-memory `infoCache` は server インスタンスごとに閉じる。同一 instance が他の tab・run に応答済みなら cold claims 無効（warm として記録、または server 再起動で再測定）
3. **Isolated origin**: 測定 tab は state のない isolated origin（clean profile・localStorage clear）。共有 localStorage・stale tab と同一 origin を共有しない

既知の汚染源: `restoreSearchState()` の起動時 fetch（`app/search.html`）が測定前に server instance の `infoCache` に membership を作りうる。再発時は network request 全件 + `cacheDistribution` + root disk cache を突合し、どの条件が破れていたか記録する。

## 4. スクリーンショットの被写体と取得方法

- 通常スクリーンショットの被写体は Playwright browser tool で表示したブラウザ画面の画像。
- 確認スクリプトは browser executable を launch しない。Playwright browser tool の target を直接使用する。
- 通常確認で OS foreground 化は不要。`kilo-screen-capture.ps1` は Playwright browser 画面画像で確認できない OS-level exception（フルスクリーン状態・window state・viewport 外・複数 monitor）のためだけ使う exception script。その場合 `ExceptionReason` に「Playwright browser screenshot で確認できない理由」を記録し、画像は例外的 OS full-screen 確認として区別する。
- Playwright の出力 root 制限により、まず `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\`（または project `scratch/`）へ保存し、PowerShell `Copy-Item` で Drive へコピーする。
- ブラウザは、開いたセッションが責任を持って閉じる。画面確認のため Playwright browser tool でブラウザを開いた場合は、スクリーンショットの Drive 保存とタスクへの記録が終わった時点で、他の工程へ進む前に browser tool の終了操作で閉じること。その後に不合格・未解決・判断の持ち帰り等のどの経路で終了する場合でも、結果を返す時点でブラウザは閉じていること。閉じられない場合は、その旨と理由を結果に明記する。
- 委譲元の統括セッションは、委譲のたびに、結果へ「ブラウザを閉じた（または開いていない）」旨が含まれていることと、Playwright のブラウザ本体プロセス（コマンドラインに専用プロファイル／一時フォルダを含むもの。MCP サーバー本体は除く）の残存が 0 であることを機械的に確認する。残存がある場合は、その識別条件に一致するブラウザ本体プロセスだけを終了させ、終了させた件数を記録する。
- 委譲を使わない単独実行でも同じ規律とする。タスク終了時にブラウザを閉じ、閉じた事実をタスク記録または報告に残す。

## 5. 操作前後（before / after）

- 操作（click・key・toggle・form 入力）で表示・状態表現が変わる項目は、操作前と操作後のスクリーンショットを両方残す。
- 前後で同じ window size、同じ被写体範囲（同一 element または同一 full page）、同じ取得方法を使う。
- ファイル名に `before` / `after` を含める（例 `manual-jp-card-4308198-before-20261106.png`、`manual-jp-card-4308198-after-20261106.png`）。
- 確認結果には操作内容と「before → after で何が変わったか」を記録する。
- 操作前後で状態が変わらない項目は前後1組不要（1枚でよい）。

## 6. Google Drive 保存と local cleanup

- 保存先: ユーザーやり取りフォルダ `I:\マイドライブ\kilo`。フォルダはタスク別 `I:\マイドライブ\kilo\YYYY-MM-DD-<task-slug>\` を優先（`docs/ai/decisions/2026-11-02-google-drive-task-specific-screenshot-folder.md`）。date folder 条（`docs/ai/decisions/2026-11-02-google-drive-confirmation-storage-procedure.md`）は同日矛盾時に旧記録として扱い、本条の適用範囲は future tasks。
- 各確認結果は画像（before / after を含む）と確認手順 text（`<confirmation-topic>-confirmation-YYYY-MM-DD.txt`）を同じフォルダへ置く。text は日本語で、日付・ファイル名・目的・操作手順・合格条件・実測値・合否・不合格条件・コード変更・検証コマンドと結果を書く。
- Drive へ保存した後は、ローカルのスクリーンショットコピー（Playwright output、project `scratch/`）は正典 `kilo-safe-remove` でゴミ箱へ送る。`Remove-Item`・`del`・`Remove-Item -Recurse` は使わない。Drive 側は残す。
- git repository に screenshot binary を保存しない。個別タスク記録の Verification に Drive 保存先・ファイル名（before / after）、local copy をゴミ箱へ送った事実を記録する。

## 判定

- 状態変化を伴う確認で before が欠けている場合は未検証扱い（合格にしない）
- 前後で window size・被写体範囲・取得方法が違う場合は比較証拠として不合格（同一条件で再取得）
- 画面確認ができない項目は未検証として扱い、合格にしない
