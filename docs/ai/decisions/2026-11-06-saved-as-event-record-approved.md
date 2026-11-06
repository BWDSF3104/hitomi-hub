# saved の算出源を event 記録（sticky）へ：設計判断の承認（2026-11-06）

**Status**: 承認済み（ユーザー 2026-11-06 09:14・選択 A）
**Date**: 2026-11-06
**対象タスク**: `docs/ai/tasks/2026-11-05-saved-status-definition.md`（実行固有ID `assess-20261106-0326`）
**実装開始指示**: implement-items 起動（2026-11-06 09:04）＋ A 承認（2026-11-06 09:14）

## 確定した方針（保留点の解決）

- **saved は event 記録（sticky）基準**。完成後にファイルが削除された場合、saved は true を保持する（state 再計算で false へ戻さない）。
- 算出源: `read_state` へ `saved_at TEXT` を追加（非破壊迁移・`schemaVersion` 6→7、既存行削除なし）。列追加のみ（`ensureScanCacheSizeMtime` パターン・D-03 非破壊規約）。
- 書き込み時点是 app 自身の download 完了（全 entry を `gallery_dir` へコピーし終わった時点）。アプリ外部の手動コピーは event にならない（現况表示の `local N/M` には反映される）。
- card API（`searchResult`）の `saved` と firststatus の saved 判定は event 列を使用する。firststatus の saved pre-filter（`files` の `EXISTS` record 基準）も event 源へ切り替え、candidate（record）と flag（event）の分岐を解消する。
- 表示の二重化を許容: badge = 完成 history（event）、meta 側 `local N/M` = disk 現况。この食い違いは本方針の意図された挙動。

## 正典の関係（supersede 範囲）

- `docs/ai/decisions/2026-11-05-search-result-state-badges-approved.md` 第3項のうち、**算出源のみ** supersede（「判定は card API の既存 `entryLocalFormats` ループで計算（追加 I/O なし）」）。
- 同項の定義（指定フォルダ内に全 entry の local ファイルがあること）と部分保存の扱い（badge に出さず meta `local N/M`）は有効のまま。
- `docs/ai/decisions/2026-11-05-saved-status-definition-approved.md` の定義（時点基準）は有効。本記録はその保留点（算出源の恒久的な決め方）を A で閉じるもの。
- `AGENTS.md`「絶対に壊してはいけない条件」（フル画像経路・TN display policy・永続 cache・shard URL format）は無変更。

## 実測で確定済みで採用した事実

- `read_state` に saved の event 列はなかった（`schema.sql:60-68` は `viewed`/`viewed_at` のみ）。
- 現行実装は disk 現况の逐再計算（`app/viewer.go:640-663`、`fileLocal`/`os.Stat` `app/viewer.go:706-718`、`:913-920`、`app/firststatus.go:99-116`）で、完成後削除により saved が 1→0→1 と戻る（スクラッチフィクスチャ実測・port 8741・実データ無変更）。
