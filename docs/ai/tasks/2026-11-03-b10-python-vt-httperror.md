# B-10: Python 参照実装の VT が 404 で落ちる

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（静的）。`src/scan.py` の `vt_lookup`（`fetch=None` path）は `urllib.request.urlopen` を使い、404/429 で `HTTPError` を raise する。`process_file`（`src/scan.py`）は `vt_lookup` を try で囲まないため scan 全体で unhandled exception になる。test mock の tuple-return（status, dict）path では 404 が `unknown_to_vt` として扱われる（`process_file` の mock path は try で囲まれている）。live 確認は key がないため不可。対応: `vt_lookup` の urlopen を try で囲み、`HTTPError` を `unknown_to_vt`（または `(status, {})`）へ変換。合格条件: mock を使わない経路で 404 が exception にならない（scratch script で確認、テストは変更しない）。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
