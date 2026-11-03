# B-10: Python 参照実装の VT が 404 で落ちる

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

`fetch=None`（mock なし・実 HTTP）経路で VT が 404/429 を raise すると、scan loop（`main()` の `process_file` 呼び出しは try なし）が unhandled exception で scan 全体が止まる。mock path は `(status, dict)` 返しで意味が確定済みなので、live path の exception を mock path と同じ意味（`unknown_to_vt` 等）へ変換する。

## 調査結果 / 変更内容

有効（静的）。`src/scan.py` の `vt_lookup`（`fetch=None` path）は `urllib.request.urlopen` を使い、404/429 で `HTTPError` を raise する。`process_file`（`src/scan.py`）は `vt_lookup` を try で囲まないため scan 全体で unhandled exception になる。test mock の tuple-return（status, dict）path では 404 が `unknown_to_vt` として扱われる（`process_file` の mock path は try で囲まれている）。live 確認は key がないため不可。対応: `vt_lookup` の urlopen を try で囲み、`HTTPError` を `unknown_to_vt`（または `(status, {})`）へ変換。合格条件: mock を使わない経路で 404 が exception にならない（scratch script で確認、テストは変更しない）。推定規模: 小。

実施（2026-11-03・実行固有ID assess-20261103-1220）:
- `src/scan.py`: `import urllib.error` 追加。`vt_lookup` の `fetch=None` path の `urlopen` を try で囲み、`except urllib.error.HTTPError` → `result = ({}, exc.code)`（既存 tuple 規約 `(payload, http_status)` に一致。最初に `(exc.code, {})` と書くと `http_status` が `{}` に化けることを live 実測で確認し修正）。変換後の `http_status` マッピング（401→auth_error / 429→rate_limited / 404→unknown_to_vt / その他→not_evaluated）と `unknown_to_vt` の `cache_store`（negative cache）は共有 code path 経由で意味不変。request が実発生するため `state` の window/total 計上も従来どおり。
- テスト変更なし。既存 mock path 404 → `unknown_to_vt` 意味の固定テスト `test_vt_404_unknown_to_vt_path_and_negative_cache_unchanged` は変更なしで通過。
- VT cache TTL: `SCAN_CACHE_TTL_SECONDS` 30 日（正典 30 日。「60 日」表記は stale）と `NEGATIVE_SCAN_CACHE_TTL_SECONDS` 2 時間（`unknown_to_vt` に適用）は定数・適用ロジックともに変更なし。scratch で negative TTL 超過 miss / positive TTL 保持を実測。

## 調査結果 / 変更内容

有効（静的）。`src/scan.py` の `vt_lookup`（`fetch=None` path）は `urllib.request.urlopen` を使い、404/429 で `HTTPError` を raise する。`process_file`（`src/scan.py`）は `vt_lookup` を try で囲まないため scan 全体で unhandled exception になる。test mock の tuple-return（status, dict）path では 404 が `unknown_to_vt` として扱われる（`process_file` の mock path は try で囲まれている）。live 確認は key がないため不可。対応: `vt_lookup` の urlopen を try で囲み、`HTTPError` を `unknown_to_vt`（または `(status, {})`）へ変換。合格条件: mock を使わない経路で 404 が exception にならない（scratch script で確認、テストは変更しない）。推定規模: 小。

## Verification

- pytest（ルート）: `python -m pytest tests/ -v` → 38 passed in 2.91s（`tests/test_canonical_build_freshness.py` 含む。UI/exe 未変更）
- live（fetch・mock なし、hash-only 1 回、key は env 経由）: 実 VT 照会 1 回で 404 → exception にならず `vt_lookup` が結果を返した（state total=1）。run1 で tuple 規約逆順バグを検出 → `({}, exc.code)` へ修正
- scratch 実測 `docs/ai/tmp/assess-20261103-1220/b10_verify.py`（fetch=None path・mock なし、実 `urlopen`＋ローカル HTTP サーバのURLリダイレクトで HTTPError を実発生）:
  - 404 → `unknown_to_vt`（finding `virustotal unknown_to_vt`、cache 行 `unknown_to_vt`、scan_status pass）
  - 429 → `api_error:rate_limited`、500 → `api_error:not_evaluated`（scan loop＝`main()` と同形の try なし呼び出しで crash なし、summary `{"pass": 3}`、state total=3）
  - TTL 意味不変確認: 定数 30 日/2 時間、negative TTL 超過は miss、positive pass は 3 時間経過でも保持
- Go 対応付け（判断点・変更せず）: `app/vt.go` の fetch は status code を返す（exception 経路なし）で 404/429/401 マッピングは Python と一致。Python の `fetch=None` は非 HTTPError（`URLError`・socket timeout）では依然 raise し得る（本項目スコープ外・未解決）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
