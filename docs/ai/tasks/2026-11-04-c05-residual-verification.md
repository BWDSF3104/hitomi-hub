# C-05残項目: 正典DB operational 確認＋B-06 optional live 200 baseline

**Status**: 進行中（正典DB確認完了・B-06 live baseline 未確認）
**Date**: 2026-11-04
**Backlog**: assess-20261103-1220
**Next Action**: 正典 EICAR hash 275a021b… への GET /api/v3/files/<hash> 1 回の承認待ち（200 が返れば key 有効性＋200 形状基線を確定）

## 意図 / 背景

`docs/ai/known-issues/2026-11-04-c05-non-target-caches-unbounded.md` の残項目（実施承認 `docs/ai/decisions/2026-11-04-remaining-residual-approval-split.md`）を解決可能な範囲で処理する:

1. 正典 `hitomi.db` の `ensureSchema` 実確認（`user_version` 4→5・`scan_cache` の `size`/`mtime` 列・既存行削除禁止）。
2. B-06 の optional live 200 baseline（code 変更なし・external API 1 回のみ・key 記録禁止・4 req/min）。
3. `versions`/`versionAt`/`versionForceAt` の bounded 判定（code 変更なし・記録のみ）。

code 変更は本項目で行わない（実装承認済みの `indexTopCache`（`ec71af4`）・`noFullCache`（`b1ef252`）はすでに単独 commit 済み）。

## 調査結果 / 変更内容

### 1. 正典 DB operational 確認 — 確認済み（読み取り専用・server 起動なし）

scratch: `docs/ai/tmp/assess-20261103-1220/c05_canonical_db_check.py`（`sqlite3` read-only URI `mode=ro`・DB 非変更）。

- `PRAGMA user_version` = **5**（4→5 の揃えは完了済み。第10/11/12実行で「未確認運用点」としていた点が解消）
- `PRAGMA table_info(scan_cache)` = `sha256, verdict, engine, threat_name, detail, cached_at, size, mtime` → `size`/`mtime` 列あり（A-12 の非破坏迁移どおり）
- `scan_cache` rows = **0**（既存行削除なし。`size`/`mtime` 入力済み行 0 は単に scan 済み行がないため）
- `files` rows = 6（read-only 照会のみ）

→ 判定: 正典 DB はすでに v5 相当で揃っている。この残項目は**確認済み**。

### 2. `versions`/`versionAt`/`versionForceAt` — 実質 bounded（code 変更なし）

- key は `field` 名ぶん（`app/searchindex.go:159-208`）。上限宣言・eviction はない。
- 本番 call site は `"galleries"` のみ: `app/searchhtml.go:178`（plain term 分岐）と `app/searchindex.go:413`（phrase token 降下）。→ 実運用で key 集合は 1。
- `indexDir(field)`（`app/searchindex.go:138-149`）は `default: "tagindex"` で任意 field を受け入れるため、将来 field 呼び出しを増やすと key 集合が grow しうる（现状の bounded は call site に依存、宣言ではない）。
- 判定: 記録のみで締（global AGENTS.md「非必須問題は known-issues へ記録」条）。防御 cap 追加は承認なしでは行わない。

### 3. B-06 optional live 200 baseline — 未確認（1 回呼出しで HTTP 404）

scratch: `docs/ai/tmp/assess-20261103-1220/b06_live_baseline_20261104.py`（key は `scratch/virustotal-api.key` から env へ・key 文字列は記録・出力・commit へ書かない・hash-only・upload なし）。

- hash `27545216921b2559534379243d19446661534481345149624164814776710798`（当時は EICAR 正典 hash と誤認識。2026-11-04 の local 計算で**正典 EICAR の SHA256 ではない**と判明 → 404 は「file not found」側の説明）
- 結果: `http_status=404`・`stats` 空・`results_entries=0`
- 呼出しは 1 回のみ（承認上限）。key 有効性は本次第で**未確認**（404 は key 無効・権限・応答形状のいずれでも起こりうる）。追加呼出しなし。
- code 変更なし。B-06 の分岐修正（`7369066`）は mock テストで固定済み（`TestVTEmptyResultsWithStatsTotalIsNotEvaluated`・`TestProcessFileVTEmptyResultsDoesNotStorePass`）で、live 基線は optional 条件。

→ 判定: B-06 live 200 baseline は**未確認のまま**。

追加実測（ユーザー承認 2026-11-04・追加 1 回・`docs/ai/tmp/assess-20261103-1220/b06_live_baseline_call2.py`・key 文字列非記録）:
- `GET https://www.virustotal.com/api/v3/files?limit=1` → **HTTP 405 Method Not Allowed**（public API の list endpoint は不可）。key 有効性はこの呼出しでは確定できず・`data` なし。
- local 証拠（network なし・`docs/ai/tmp/assess-20261103-1220/b06_eicar_hash_check.py`）: key 長 64（VT public API key の format と一致・key 内容自体は記録しない）。正典 EICAR（68 bytes・trailing newline なし）SHA256 = `275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f`。1 回目で使用した hash と**不一致**（trailing newline variante は `131f95c51cc819465fa1797f6ccacf9d494aaaff46fa3eac73ae63ffbdfd8267`）。
- 推測（確認ではない）: key が無効なら VT は 403 を返す運用なので、1 回目の 404 は key 認証通過後の not-found である可能性が高い。
- 追加 1 回の承認があれば `GET /api/v3/files/275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f` を実行し、200 が返れば key 有効性と 200 形状基線を同時に確定できる。key あり＋空 results の 200（B-06 の `not_evaluated` 分岐のlive 実証）は選択できないため mock テスト固定と区別して記録する。

## Verification

- `python docs/ai/tmp/assess-20261103-1220/c05_canonical_db_check.py` → `user_version = 5`・size/mtime 列あり・rows 0（read-only・DB 非変更）。
- `python docs/ai/tmp/assess-20261103-1220/b06_live_baseline_20261104.py` → `http_status=404`・`requests_made=1`・key 記録なし（4 req/min 内・upload なし）。
- code 変更なし → `go vet .` / `go test` の再実行は該当なし（本項目は code 非変更）。
- `python docs/ai/tmp/assess-20261103-1220/b06_live_baseline_call2.py` → `http_status=405`・calls_made=1（4 req/min 内・upload なし・key 記録なし）。
- `python docs/ai/tmp/assess-20261103-1220/b06_eicar_hash_check.py` → 正典 EICAR SHA256 `275a021b…` と使用 hash の不一致を確認（network なし・key 内容非表示・長 64 のみ表示）。
- `grep -R "x-apikey\|VIRUSTOTAL_API_KEY" docs/ai` で key 文字列の記録不在を確認（scratch スクリプトは env 参照のみ）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
