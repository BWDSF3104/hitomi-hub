# 2026-11-03 残項目（実施順22以降）推奨のユーザー承認

`assess-20261103-1220` 残項目の判定推奨をユーザーが承認（2026-11-03 18:33）。承認は方針確定であり、実施開始指示ではない。実施は実行計画の承認範囲と合格条件、正典 `task_manager.py` 運用に従う。

## 項目別の承認内容

- **A-11**: page ごとの encode + sequential 書き出し（streaming）方式採用。上限方式は不採用。合格条件 peak memory = page 1 分 + header。
- **A-04/D-09/D-10**: hitomi 正典準拠確認後に判定、確認後は hitomi 挙動へ寄せる。確認前にアプリ挙動を変えない（`2026-11-03-implementation-judgment-approval` と整合）。D-10 は hitomi index key 形式確認後、space 型なら normalize 方針固定、`_` 型なら現状維持。
- **A-06**: `select { case <-ctx.Done(): return; case <-time.After(d): }` へ（判定不要・方針確定済み）。
- **A-09**: stream + 条件付き応答（304/206・ETag・Range）。UI full 判定（no-store fetch・x-cache・generation guard）に影響しないことを確認する。full actual pixels 判定には触れない。
- **A-13**: `bytes.Contains` へ変更（判定不要、既存テスト通過で合格）。
- **C-10**: test 専用 gate interval inject 案採用。本番 gate interval 1000ms は不変（`2026-11-03-implementation-judgment-approval` と整合）。
- **D-08**: production 未使用コードは削除。テスト参照の関数はテストファイルへ移動。`pageSource` は使用中（誤り）なので変更なし。`app/ui.html:1458` の no-op 文を削除。
- **D-06**: docs 側を faithful parse 基準へ修正（承認済み方針と整合）。README 具体行は実物確認後に修正。
- **D-07**: exe 不在時は test を skip。exe がある環境は現状維持。test 内 build は不採用（CI 実行時間と freshness 検証の趣旨にずれる）。
- **A-12**: `scan_cache(sha256,size,mtime)` で unchanged を skip。mtime 不一致は再ハッシュの安全側。scan_status 意味は維持。低優先。
- **B-04**: check 側を `acquisitionTitleName`（sanitize 名）へ揃える。フォルダ実体が sanitize 名で存在する正。低優先。
- **B-06 残件**: key あり + 空 `last_analysis_results` + positives 0 の 200 応答で Go 側を `not_evaluated` へ揃える小修正を実施（単独 commit）。承認済み方針（empty results + positives 0 基準）の実施。
- **B-07 残件①**: Python 参照実装（`src/scan.py` process_file）の YARA error 処理を Go 表現（構造 pass の `scan_status` は `not_evaluated` 維持、YARA 失敗は `scan_source` の `structural+yara_error`）へ揃える。残件②（YARA error ファイルが VT gate に入る副作用）は現状許容（判断済み）。
- **manual full-image retry**: 再現条件（実際 UI `errorDone > 0` entry）が確認できるまで保留継続。低優先（判断済み）。
- **P2 #8（AVIF の pdf embed）**: pure-Go decoder は追加しない（移植性方針）。AVIF は PDF embed の対象外と記録に明示し、embed 可能形式（png/jpeg 再エンコード）のみ対応。
- **P2 #9（`bookmark_queries.notify`）**: 用途を先に定義。用途不明なら列削除か「意図的 placeholder」として記録に明記して閉じる。

## 実施順（推奨として承認）

1. 判定不要項目: A-13 → A-06 → C-10 → D-07 → D-06
2. 次: A-11 → A-09
3. 判断を要する項目（A-04/D-09/D-10 の hitomi 準拠確認、B-06/B-07 parity、P2 #8/#9）は実施開始指示後に順次

各項目は 1 サブエージェント・単独検証・単独 commit（実行計画の方式と同一）。実施開始はユーザーの明示的な指示後。
