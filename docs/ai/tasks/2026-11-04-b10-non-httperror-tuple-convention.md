# B-10残件: 非 HTTPError（URLError・timeout）の tuple 規約拡大

**Status**: 完了
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 実装済み・テスト固定・commit（未確認: 実 HTTP 実測は任意方針により未実施）

## 意図 / 背景

B-10（close `a303a4e`）の残件。`vt_lookup` の `fetch=None` path で非 HTTPError（`URLError`・timeout）は依然 raise し得る。推奨承認（`decisions/2026-11-04-recommendations-approved-execution.md`）でスコープ拡大：非 HTTPError を tuple 規約 `(payload, status)`（`api_error:network_error`）へ変換し crash しない。Python 参照実装のみ・テスト固定・TTL（30日/2時間）意味不変。

## 調査結果 / 変更内容

実施（2026-11-04・実行固有ID assess-20261103-1220・第11実行③・単独 commit）:

- `src/scan.py`: `import socket` 追加。`vt_lookup` の `fetch=None` path で B-10 の `except urllib.error.HTTPError`（変更なし）の後に `except (urllib.error.URLError, socket.timeout)` を追加 → `result = ({}, 0)`（tuple 規約 `(payload, http_status)`。`http_status = 0` = network 層・HTTP 応答なし）。`http_status != 200` マッピングへ `elif http_status == 0: status = "network_error"` を追加 → `process_file` の既存 `api_error:*` 規約で finding `api_error:network_error`。HTTPError は URLError の subclass なので分岐順（HTTPError 先）で B-10 意味を維持。
- 意味維持: 401→`auth_error` / 429→`rate_limited` / 404→`unknown_to_vt` 不変。negative cache 条件は `status == "unknown_to_vt"` のまま（`network_error` は権威ある「unknown」ではないため negative cache に入れない）。window/total 計上は従来どおり（request は実発生）。TTL（`SCAN_CACHE_TTL_SECONDS` 30日 / `NEGATIVE_SCAN_CACHE_TTL_SECONDS` 2時間）・rate limit（4 req/min・500 req/day）・gate（1000ms＝Go 側 config）非触。
- `tests/test_scan.py`: テスト5件追加（mock injection・実ネットワーク障害なし）。

## Verification

- pytest（ルート）: 変更前 baseline 46 passed in 9.50s → 変更後 51 passed in 9.10s（`tests/test_canonical_build_freshness.py` 含む）。commit 前再実行: 51 passed in 7.99s。回帰: 既存 HTTPError・negative cache・TTL テスト全通過（46 passed 無損）。Go 検証該当なし（Go 非触）。実機 UI 確認該当なし。
- 固定テスト名（単独実行 5 passed in 0.50s）:
  - `test_vt_urlerror_on_fetch_none_path_converts_to_network_error` — mock 実測: `URLError` injection → `(payload, 0)` → `status = "network_error"`・`http_status = 0`・`detail = "network_error"`・state total=1・window 1・negative cache 行なし。crash なし。
  - `test_vt_socket_timeout_on_fetch_none_path_converts_to_network_error` — mock 実測: `with` 块内 read での `socket.timeout` injection → `network_error`（crash なし）。
  - `test_process_file_network_error_uses_api_error_convention_without_negative_cache` — mock 実測: finding `("virustotal", "api_error:network_error")`・scan_status pass・`scan_cache` 行数 0。
  - `test_scan_loop_without_try_survives_network_error` — `main()` 同形 loop（try なし）で例外発生なし・summary `{"pass": 1}`・state total=1。
  - `test_vt_httperror_branch_on_fetch_none_path_unchanged` — 回帰固定（B-10 分岐）: 404→`unknown_to_vt`＋negative cache 行、429→`api_error:rate_limited`、500→`api_error:not_evaluated`、401→`auth_error`、state total=4。
- 実 HTTP 実測: 未実施（合格条件では任意。key 使用時は gate 1000ms 既定維持）。未確認。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
