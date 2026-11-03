# A-15: isLoopbackHost が [::1]:port を弾く

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`app/viewer.go:122-135`（`isLoopbackHost`）は `strings.Cut(host, ":")` で `[::1]:8731` を name=`[` と読む。実測: `POST /api/fav`（invalid body）+ `Host: [::1]:8731` → 403、`Host: localhost`（control）→ 400。したがって正当な loopback Host 形式で POST が拒否される。実運用は `[::1]:8731` で bind しないので影響は限定されるが、機能上の不具合として有効。対応: bracket 形式を剥がして name を比較。合格条件: `Host: [::1]:8731` で POST が same-origin 扱い（400/200）。推定規模: 小（テストは変更しない）。

実装前に確認: 記録の行番号 122-135 は現ファイルでは 133-153（同一関数・内容は記録どおりで機能上の食い違いなし。判定意味を変えないため実装を進めた）。

実施（2026-11-03・サブエージェント・assess-20261103-1220）:

- `isLoopbackHost` を `net.SplitHostPort` で bracket 形式を剥がし host name を比較する判定へ修正。port 区切りの解釈失敗（port なし）は name 全体（bracket があれば `[ ]` を剥ぐ）として扱う。既存意味の保持（回帰実測）: `localhost`・`127.0.0.1`・`::1`・`[::1]`・`localhost:8731`・`127.0.0.1:8731` → true（変更前と同一）、`evil.example`・`evil.example:80`・`localhost:`・`[::1]:`（port 空）→ false（変更前と同一）。修正で変わる入力のみ: bracket + port（例 `[::1]:8731`）→ false（誤）から true（正）。
- 合格条件固定テスト追加（既存テストの変更なし）: `app/viewer_test.go` `TestViewerPOSTSecurityBracketIPv6HostIsSameOrigin`。`Host: [::1]:8731` → 400（same-origin 扱い）、`Host: localhost` control → 400、`Host: evil.example` 回帰 → 403。
- same-origin / CORS / CSRF の他の判定、viewer の full 判定、imageProxy 順序、cache、config、UI（ui.html）、download、YARA は未変更。loopback 以外を same-origin 扱いにする変更なし。

## Verification

1. `app` で `go vet . && go test -count=1 -timeout 300s .`: vet クリーン、`ok hitomi-hub/app 37.525s`（初回 37.329s）。個別: 追加テスト PASS 0.03s・既存 control テスト `TestViewerPOSTSecurityAllowsLoopbackJSONAndRejectsCrossOriginText` PASS 0.04s（意味不変・回帰確認）。
2. ルート `python -m pytest tests/ -v`: 33 passed（3.16s、`tests/test_canonical_build_freshness.py` 含む。`app/ui.html` 未変更）。実 HTTP 実測のため修正込み exe を `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` で再 build（freshness テストは再 build 前後で PASS）。
3. 実 HTTP 実測（canonical server 既定 `http://127.0.0.1:8731`、再 build exe・`hub.exe view`・PID 12080）: スクリプト `docs/ai/tmp/assess-20261103-1220/live_check.py`（`POST /api/fav`・body `{"gallery_id":0}`）。実測 status code: `Host: [::1]:8731` → **400**（same-origin 扱い）、`Host: localhost`（control）→ **400**、`Host: evil.example` → **403**（回帰）。経過 0.33s。DB 書き込みなし（400 は handler 内の validation で、`gallery_id <= 0` のため DB 到達前）。実測後 server は停止（本タスク開始時点で canonical server は未稼働だったため、停止して開始時の状態へ復した）。
4. 既合格項目への影響: なし。`isLoopbackHost` の使用点は `sameOriginPost` のみ。B-08（export/scan）、D-03、D-04、B-01、B-02、B-03、C-08、A-07b の判定意味・テストに接しない（全 go test・pytest PASS で回帰確認）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
