# settingsAcquisitionDirValue の検証がゆるい

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: 案A 実装（衛生）・実測合格（委譲 assess-20261109-1320）

## 意図 / 背景

実行固有ID: assess-20261109-1320（項目 #5・調査のみ。コード変更なし）

原文の主張: `settingsAcquisitionDirValue`（app/viewer.go:758）の検証が `raw` 基準で行われ `clean` と対応がずれる。`a//b` の扱いが曖昧、空セグメント・`\`/`/` 混在値が config に残り得る。UI 関連項目として settings 画面の実表示と Google Drive スクリーンショットを確認した。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

検証順（app/viewer.go:758-790）:
1. `raw == ""` → 既定 `acquisition`
2. raw 基準検査: 先頭 `/`・`\`、`drivePrefix`、`containsDotDotSegment(raw)`
3. `clean := filepath.Clean(filepath.FromSlash(raw))`。`clean == "."`・`filepath.IsAbs(clean)` を拒否
4. raw 文字検査: 制御文字・DEL・`: * ? " < > |` に加えて `/` と `\` を拒否（「invalid path characters」）
5. raw 基準セグメント検査: 空セグメント・`.`・Windows 予約名を拒否
6. `withinRoot` ガード。保存値は `clean`

### 実測で確定した挙動（served UI・fresh isolated instance・network POST 実測）

- 区切り文字を含む入力はすべて 4 の文字検査で拒否される。`a//b` → 400 `contains invalid path characters`（決定論的拒否。「曖昧」ではないが、実際の理由は「multi-segment 未支持」であり、エラー文は誤解を招く）
- 受理される入力は常に単一セグメント。受理経路では `clean == raw` で必ず一致（raw/clean ずれは実害なし）
- 空セグメントエラー `must not be empty`（5）は UI 入力から到達不能な死分岐: `a//b` は 4 を先 hit、`.` は `clean == "."`（3）を先 hit（実測: `.` → `must be a root-relative folder name`）。セグメント検査で実働するのは予約名検査のみ
- 混在値保存: `a/b`・`a\b` とも拒否 → settings UI write 経路で `\`/`/` 混在値が config に残ることは実測でなし（read-back 確認: scratch config.json 保存値は区切り文字なし）
- 読み戻し・folder 解決: 保存直後に `acquisitionRoot = filepath.Join(root, clean)` を再計算し、`safeAcquisitionPath` が acquisitionRoot 内書き込みを保つ。正典「完成フォルダ内に閉じる」条と矛盾なし（`cacheClearRootGuard` の完成フォルダ不削除保护も維持、app/cacheclear.go:64-76）

実測テーブル（POST /api/settings、served UI と同一検証関数）:
- `a//b` → `invalid path characters`（誤解を招く文）
- `.` → `must be a root-relative folder name`（空セグメント文ではない）
- `CON` → `segment must not be a Windows reserved name`（唯一実働するセグメント検査）
- `a/b`・`a\b`・`a\`（末尾区切り）→ `invalid path characters`
- `a\..\b` → `must be a root-relative folder name`（dotdot は raw 基準・文字検査より前）
- `C:\escape`・`S:\escape` → `must be a root-relative folder name`
- `""` → 200・保存値 = 既定 `acquisition`
- `Vol...01`（部分 `..`）→ 200・保存（scan 基準 `containsDotDotSegment` と一致）

### 有効性

部分有効。
- 有効な点: 検査基準が raw 基準（dotdot・drivePrefix）と clean 基準（`.`・IsAbs）で混在、空セグメントエラーは死分岐、`a//b` のエラー文が誤解を招く。指摘のとおり「対応がずれる」構造はある。
- 実挙動として無効な点: 曖昧さはなく決定論的拒否。混在値が config に残る主張は UI write 経路で実測否定。

### 対応の実現可能性と推奨修正方法

- 推奨（案 A）: settings.html UI note（「root 内の folder name」）どおり「単一 folder name のみ」を明示設計として確定。区切り入り入力を「multi-segment は未支持」と明示エラーへ変更し、死分岐（空セグメントエラー）を削除。実挙動（拒否）は変えず、説明文とデッドコードの整理のみ。実現性: 高・小規模。
- 代替（案 B）: nested folder name を許可するなら、文字検査から `/`・`\` を除外し、セグメント検査を clean 基準へ統一、保存時に区切り様式を正典形式へ正規化。UI note と download 時単一セグメント基準の変更が必要でスコープが大きい。
- 推奨は案 A。設計意味（単一 folder name のみ）の確定は UI note と実挙動（multi-segment 受理経路なし）で既におり、案 A は意味変更ではないため判断保留としない。

### 副作用・影響範囲

- エラー文変更は既存テストの assert 文字列に影響: `TestSettingsPostRejectsAcquisitionDirOutsideRoot`（app/viewer_test.go:5192-5211）が `invalid path characters` と `must be a root-relative folder name` を anchor。settings-status 表示は API error 文字列の直結。
- 関連境界（スコープ外・記録のみ）: `loadConfig`（app/main.go:195）は `acquisition_dir` を検証しない。config.json 直接編集で `..\x` を置けば acquisitionRoot が root 外へ脱出し「完成フォルダ内に閉じる」条に反しうる。UI write 経路では発生しない（実測で拒否）。
- 隣接未確認（実測なし・有効性判断に影響なし）: 末尾点 `foo.` は受理・保存されるが、Windows の folder 作成時正規化で末尾点が除去され保存値と実 folder 名がずれる可能性。未実測。

### 作業前チェック（確認テスト系正典該当分）

読込確認済み（ファイル名明記）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`・`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。本項目は full 画像取得順序・TN display source・shard URL format・永続先・gg を変更せず、矛盾なし（viewer 内の settings 検証関数の調査・実装のみ）。

### 実装（案A・衛生・実挙動変更なし）

委譲 assess-20261109-1320 で実施（ユーザー同意済み案A）。承認範囲: `app/viewer.go`（関数は `app/main.go` ではなく `app/viewer.go` に存在。委譲文の `app/main.go` は記録・実コードの食い違いとして留意。機能・スコープは同一）の `settingsAcquisitionDirValue` 内のみ。

- 死分支削除: `segment == "" || segment == "."`（空セグメントエラー）を削除。区切り文字検査（文字検査）が文字検査より前に完了するため UI write 経路から到達不能（``.` は `clean == "."` を先 hit・空セグメントは区切り文字検査を先 hit）。セグメント検査は Windows 予約名検査のみ become（単一 folder name 確定后、segment = raw）。
- エラー文明示: 区切り文字 `/`・`\` を文字検査から独立分岐へ移し、単一 folder name 設計どおり `acquisition_dir must be a single folder name (path separators are not supported)` へ明示。制御文字・`: * ? " < > |` 用の `contains invalid path characters` は維持。
- 校验语义不変: raw 基準の先頭区切り/drivePrefix/`containsDotDotSegment`・`clean == "."`/`IsAbs` 拒否・保存値 `clean`・`withinRoot` escape guard・400 挙動すべて変更なし。拒否集合（決定論）は変更前と同一。

テスト（`app/viewer_test.go`・承認範囲の settings validation anchor 更新のみ）:
- `TestSettingsPostRejectsAcquisitionDirOutsideRoot` の control ケース: 旧 anchor 入力は JSON で制御文字ではなくリテラル `\`（混合区切り）だったため案A 後新明示分岐に移る。anchor を保持するため入力を真の制御文字（JSON `\u0000`）へ修正。`invalid path characters` anchor は単一 folder name 内制御文字で到達可能（回帰テストで pin）。混合区切り値は新 anchor 側で pin。
- 追加回帰テスト `TestSettingsAcquisitionDirValueDeterministicRejection`（8 checks・mock・外部 API なし・既存 harness 流儀）: `a//b`・`a/b`・`a\b`・`a\`（末尾区切り）→ 新明示文、`.` → `must be a root-relative folder name`（死分支削除確認）、`CON` → 予約名、制御文字 → `invalid path characters`、単一 folder name `collect` の受理無変更。

## Verification

- `python -m pytest tests/ -v`（プロジェクトルート）: `60 passed in 16.32s`（`tests/test_canonical_build_freshness.py` 含む。正典 exe 現行 app bytes 一致）
- `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 31.433s`。`TestSettingsPostWritesAcquisitionDirAndRecomputesAcquisitionRoot`（保存直後の acquisitionRoot 再計算・config read-back）合格
- 実装後の正典検証: `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 35.876s`。`python -m pytest tests/ -v`: `65 passed in 27.77s`（freshness 含む・実装後正典 exe fresh build 一致）
- Served 画面確認（実装後・合格）: 正典 exe を現行 app から fresh build（`go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .`）、fresh isolated instance 起動（`hub.exe view --port 8731 --data-dir --root "docs\ai\tmp\n20-settings-acquisition-dir-verify"`・Playwright browser tool・isolated origin `n20-settings-acquisition-dir-20261109`・clean context）。window size 実測 inner 1666x932 / outer 1679x1025（人工 resize なし）。被写体 full page・同一 window size・同一取得方法。
  - 注記（帰属）: 初回試行は default context tab（inner 1666x876・shared origin）で撮影 → 不合格扱い。isolated origin tab（inner 1666x932）+ clean instance で全 4 枚を再取得し、Drive 保存分は再取得分。cold hygiene 3 条件（fresh build・fresh instance・isolated origin）満たす。
  - 操作1 `a//b` 保存 → POST /api/settings 400・settings-status = `acquisition_dir must be a single folder name (path separators are not supported)`（明示化。拒否・保存なし挙動は変更前と同一）: `n20-settings-error-before-20261109.png`（status 空）→ `n20-settings-error-after-20261109.png`（エラー表示）
  - 操作2 `collect` 入力保存 → POST 200・settings-status = `Saved`・read-back config.json `"collect"`（区切り文字なし）: `n20-settings-valid-save-before-20261109.png`（status 空）→ `n20-settings-valid-save-after-20261109.png`（Saved）
- Drive 保存先: `I:\マイドライブ\kilo\2026-11-09-n20-settings-acquisition-dir-validation\`（実装後 4 png + `n20-settings-acquisition-dir-planA-confirmation-2026-11-09.txt`。調査時の既存 4 png + txt は同一フォルダに残存）。local copy（`scratch\n20-screen` png・txt）は `kilo-safe-remove` でゴミ箱、scratch instance フォルダは `kilo-safe-rmdir` で削除。ブラウザは browser tool の終了操作で閉じ、閉じた時点で工程完了（browser 本体プロセス残存 0）
- download 時の folder 解決の実機ダウンロードは未実行（未確認。既存 Go テスト合格とコード読解で確認）

## 判断記録

- 案A（ユーザー同意済み 2026-11-09）を採用。根拠: 実測で単一 folder name のみ受理（multi-segment 受理経路なし）＋ settings.html UI note「root 内の folder name」。意味変更なし。段階3持ち帰りなし（エラー判定動作の変更なし・正典ルール変更なし・不可逆データ変更なし・外部サービス無関係・承認範囲内）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
