# R-05: acquisition フォルダが title だけで決まり同名別言語 gallery で上書き衝突

**実行固有ID**: assess-20261106-1056

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: 設計承認済み（案 A: 作品名 + 作品番号、予約名・末尾点・末尾空白・長さ上限を処理、既存フォルダは再命名せず新規保存分から）。実施開始指示待ち。決定: docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md

## 意図 / 背景

- 指摘（原文）: acquisition の folder 名は title だけから決まり、gallery id は title 空時の fallback にすぎない。同名 title（言語違い）の 2 gallery が同一 folder に落ち、page file 名（`%03d.<format>`）が同じ形のため互いに上書きする。sanitize は禁止文字の `_` 置換のみで、Windows 予約名・末尾の `.`・末尾空白・255 文字超を処理しない。
- 承認済み前提（案 A・`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`）: 保存先フォルダ名を「作品名 + 作品番号」へ。予約名・末尾点・末尾空白・長さ上限を処理。既存フォルダは再命名せず、新規保存分から適用。実装未開始。
- 本記録は調査・実測のみ（コード変更なし）。変更したのは本タスク記録とスクラッチ（`docs/ai/tmp/assess-20261106-1056/`）のみ。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `acquisitionTitleName`（`app/acquisition.go:27-44`）: trim 後、control 文字と `/\:*?"<>|` を `_` に置換するのみ。末尾の `.`・末尾空白は文字列として保持（`strings.TrimSpace` は前後の空白だけ除去、末尾点はそのまま）。予約名・長さ上限の処理なし。gallery id は title 空または sanitize 結果が空のときの fallback（`app/acquisition.go:29-31`、`40-42`）。
- check（`app/acquisition.go:90`）と acquire（`app/acquisition.go:162`）は同一 `acquisitionTitleName`・同一 fallback（`strconv.Itoa(galleryID)`）で folder を指す（B-04 の統一は維持）。
- page file 名は `acquisitionPageName` の `%03d.<format>`（`app/acquisition.go:46-48`）。同名 title の別 gallery は同一 folder 内の同名 page file 経路になる。
- 上書きの発生条件（機構): acquire は `overwrite` が false のとき既存 page file を確認して confirm を返す（`app/acquisition.go:190-202`）。よって実際のバイト上書きは `overwrite: true` の要求で発生する。false では上書きしないが、その場合も check（folder 存在のみで confirm・`app/acquisition.go:97-104`）と acquire の folder confirm（`app/acquisition.go:169-181`）は「別 gallery が書いた folder」を自分の保存として confirm する（帰属誤報告）。
- 有効性: **確定（機構）**。同名 title → 同一 folder → 同名 page file の衝突はコードから確定。加えて実測で次の帰結を確定:
  - 末尾点・末尾空白は Windows がストリップし、`"foo."` と `"foo"` が同一物理 folder に衝突（同名衝突の別の形。実測 §測定 2・3）。
  - 予約名 folder（CON・PRN 等）は作成失敗（winerror=267）。`NUL` だけは mkdir が「成功」して実フォルダにならない（Go 実測: `os.Stat` は nil で返す → check が confirm を誤報告、`os.MkdirAll` は nil、`WriteFile` 失敗）。
  - folder 名は 255 ちょうどまで作成可、260 で失敗（winerror=123）。255 超は作れない（指摘どおり、境界は 255 ちょうど OK / 260 NG）。

### 測定（スクラッチ `docs/ai/tmp/assess-20261106-1056/`）

- スクリプト: `measure_folder_names.py`（Python 実測）、`r05_probe.go`（Go＝本番言語・正典 `os` パッケージでの挙動）。出力: `measure_folder_names.out.txt`、`r05_probe.out.txt`。正典フォルダ・既存データ・既存テストには触れていない。
- 結果要約（実測・閉じた論点）:
  1. 予約名: CON・PRN・AUX・COM1・COM9・LPT1・LPT9・`CON.txt` は folder 作成失敗。`NUL` は成功扱いになるが実ディレクトリでない（Go: stat isdir=false、配下 WriteFile 失敗、check は confirm 誤報告）。
  2. 末尾 `.`・末尾空白: 作成は成功するが実名からストリップされ、ストリップ後の同名（`title`・`title2`）と衝突（183 既存）。つまり「処理しない」のは単なる無効名だけでなく同名衝突の源。
  3. 長さ: 255 ちょうど成功、260 失敗（winerror=123）。full path が MAX_PATH を超える環境でも 255 名は成功（Go・Python 一致）。
  4. 同名 title 衝突（1 gallery 1 folder 相当の形状・スクラッチ内シミュレーション）: 同一 folder 同一 page file 名で後書きが前書きを上書き（final content = 後書き bytes）。実データに同名衝突の実例があるかは**未確認**（実例查询は実施せず、シミュレーションと区別）。
- 画面確認: 該当なし（UI 項目ではない）。保存経路の命名のみ。

### 対応の実現可能性と推奨修正方法（案 A の実装形・推奨）

- 実現可能。`acquisitionTitleName` を gallery id を受け取る形へ拡張し、check・acquire が同一関数を共有する構造（B-04 不変）はそのまま維持。推奨する変換順:
  1. sanitize（禁止文字 `_` 置換・既存挙動維持）
  2. 末尾の `.`・末尾空白の除去（明示。Windows の暗黙ストリップとの不一致を防ぐ）
  3. Windows 予約名の回避（case-insensitive 完全一致 CON・PRN・AUX・NUL・COM1-9・LPT1-9 を `_` 接頭等へ。`NUL` の「成功して実フォルダにならない」形もこれで回避）
  4. 長さ上限（推奨: sanitize 後の title 部を上限截断。folder 名 = `title部 + "_" + gallery_id`。folder 名 255 は実測で安全域（260 NG）、ただし full path の MAX_PATH と page file 名（`.%03d<format>`）の余地を確保するため title 部 200 前後を推奨。最終値は実装時確定）
  5. gallery id 接尾（案 A「作品名 + 作品番号」。gallery id は DB 内で一意なので新保存分の衝突は構造的に解消）
- 既存テスト固定への影響: `TestViewerAcquisitionCheckUsesSanitizedTitleFolderMatchingAcquire`（`app/viewer_test.go:3500-3502`）の fixture `what__file` は新命名で `what__file_<gallery_id>` へ更新が必要（実装時にテスト更新・追加）。`TestViewerAcquisitionCheckReturnsRunStartFolderStatus`（title `Kemo`）も同様に接尾が変わる。
- 代替案（不採用）: folder を gallery id のみにする案（衝突は消すが人間可読性を失う）。案 A 採用済みなので採用。

### 副作用・影響範囲

- **既存フォルダ非再命名（承認済み・不可逆変更回避）**: DB `files` 行の path（旧 folder 名）と実 folder は旧名のまま。新規保存分だけ新命名。
- 既存保存済み gallery への影響: 命名変更後、check・acquire は新命名（`title_<id>`）を指すため、旧名 folder で保存済みの gallery は check が `not_exists` を返す（表示上「未保存」に見える）。ファイル自体は `files` path 経由の local 表示（search card）で引き続き見つかる。既存と新規で命名が混在する。
- `NUL` 型の例外（check confirm 誤報告）は新命名（予約名回避）で解消。実データに旧 `NUL` folder が存在し得ない（実フォルダにならない）ため既存データの是正は不要。
- B-04 の「check と acquire が同一 sanitize 名を指す」不変は保つ（同一関数共有）。check 応答の `title` 表示は新命名になる（ユーザーに見える意味の変更＝案 A の承認範囲内）。
- 上書き自体は `overwrite: true` 要求でのみ起きる（`app/acquisition.go:170-181`、`190-202` の gate）。同名衝突のリスクは「同名 title の gallery が存在する場合」依存（実例未確認）。
- 正典テストルール（`docs/ai/rules/confirmation-test-rule.md`）とプロジェクト pre-work check（viewer/download 経路）は実装時に適用（canonical build・実機確認）。本タスクでは実装しない。

### supersede 関係・採用文書

- 矛盾: `docs/ai/decisions/2026-11-03-remaining-item-recommendations-approved.md` B-04 条と `docs/ai/decisions/2026-11-04-nozomi-dataset-name-space-form-correction.md` スコープ限定条（「`acquisitionTitleName` は変更しない」）と、2026-11-06 決定（案 A で変更）が相反。
- 処理: 日付の新しい `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（案 A）を採用。古い「変更しない」は check 側統一（B-04・実施済み commit `8fd5cae`）の範囲で有効だった記録として保持され、acquire 側命名の変更点は 2026-11-06 決定で supersede。R-05 の既存読み取り記録（`docs/ai/tasks/2026-11-06-claude-review-20261106-findings.md` R-05 欄）とは照合済み（確度「確定・衝突は実データ依存」と一致。上書き条件の overwrite gate 细分と予約名/末尾点の実測追記が本記録の加算）。

## Verification

- `go vet . && go test -count=1 -timeout 300s .`（app・変更なしの確認実行）: vet clean、`ok hitomi-hub/app 25.836s`（57 tests 相当・不合格なし）。
- `python -m pytest tests/ -v`: 54 passed / 1 failed・10.25s。失敗は `test_canonical_build_freshness_rule_exists`（AGENTS.md の pointer 状態）。R-05 と無関係（本項目はコード未変更）。観察として記録のみ、触れていない。
- 実測（スクラッチ内・正典データ非触）: Python 0.05s、Go probe 0.00s。結果は §測定 とスクラッチ出力ファイルに記録。
- 実機確認（canonical build・viewer `/api/image` 確認）は該当なし（本タスクは読み取り・スクラッチ実測のみ。実装時ルール上の必須確認は実装側で実施）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
