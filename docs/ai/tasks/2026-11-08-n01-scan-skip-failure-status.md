# N-01: scan skip が失敗状態（VT api_error・YARA error）を固定する

**実行固有ID**: assess-20261108-1332

**Status**: 進行中
**Date**: 2026-11-08
**Next Action**: 調査完了・実施未。推奨案（scan_meta.retry_needed 列追加・D-03 非破壊）単独実施可。実施指示待ち

## 意図 / 背景

- 外部指摘（Claude）2026-11-08 の N-01。原文: scan の skip（`scanSkipStatus`）は size・mtime 一致で保存済み status をそのまま返す。VT が `rate_limited`・`no_key` で失敗した行は status `pass`・`scan_source` `structural` のままなので次回以降 VT に問い合わせない。YARA 失敗は `not_evaluated` + `structural+yara_error` で記録されるが skip 判定は `scan_source` を見ない。B-07 で「retry 用表現」と定めた意味が失われる。
- 原文の確度は読み取り（未検証）。本子の実測で機構確定へ更新。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- Go `processFileWithYara`（`app/main.go:902-905`）: `scanSkipStatus` が true なら hash 計算（`hashFunc`）・YARA gate・VT gate・findings 書込み（`recordScanFile`）を一切走らさず、`files` の保存済み scan_status を返して `summary["skipped"]` を加算。
- `scanSkipStatus`（`app/main.go:651-673`）の除外条件は meta NULL・size/mtime 不一致・`external_removal` のみ。`scan_source`（YARA 失敗マーキング）・findings の `api_error:*`（VT 失敗）を参照しない。
- 失敗行の保存値: VT api_error（`no_key`・`rate_limited` 等）は status を変えない（構造 pass → `pass`・source `structural`。B-06/B-10 表現）。YARA 失敗は B-07 表現（`not_evaluated` + `structural+yara_error`）で findings 行ではなく `scan_source` のみ（実測で findings 0 を確認）。両者とも meta 一致で skip され、以降再評価されない。

### 実測（Go・CLI・外部 VT API 非呼出・scratch `docs/ai/tmp/assess-20261108-1332/`）

- シナリオ A（VT enabled・api_key 空・api_key_env 不在 → `no_key`。YARA 未設定。fixture cbz・115 B）:
  - run1: summary `{"pass":1,"skipped":0}`。files 行 status `pass`・source `structural`・scan_time `2026-11-08T04:41:29Z`。findings 1 行 `api_error:no_key`（run1 の gate 1 回）。
  - run2（約 1.5 秒後）: summary `{"pass":0,"skipped":1}`（skipped 加算は skip 分岐のみ）。status `pass`・source `structural`・scan_time `04:41:29Z` のまま（非更新）。findings 1 行のまま。hash 計算 0 回（skip 分岐が `hashFunc` より前。VT gate 呼出 0 回。VT 呼出は run1 の gate 1 回・fetch は no_key で HTTP 前。run2 は skip で gate 非到達）。→ skip 発火・失敗固定を実測。
- シナリオ B（`yr`=存在しない binary・VT 無効・同一 fixture）:
  - run1: summary `{"not_evaluated":1,"skipped":0}`。files 行 `not_evaluated` + `structural+yara_error`・scan_time `04:41:30Z`。
  - run2: summary `{"skipped":1}`。status・source・scan_time のまま。`scan_source` が yara_error を含む行も skip される（skip 判定が `scan_source` を見ない）→ 原文どおりを CLI 実測で確定。

### 実測（Python 参照実装・`src/scan.py`・mock fetch/runner・外部 API 非呼出）

- Python 側には size/mtime skip 機構が存在しない（A-12 は Go のみ）。`scan_meta` 0 行。
- Group A（VT enabled・fetch mock 429 呼出回数 1/回）: run1 hash 1・fetch 1 → status `pass`・source `structural`・finding `api_error:rate_limited`。run2: hash +1・fetch +1・scan_time 更新 → 再評価が発生（失敗固定なし）。
- Group B（runner mock 失敗）: run1 hash 1・runner 1 → `not_evaluated` + `structural+yara_error`。run2: hash +1・runner +1・scan_time 更新 → 再評価。
- Go と Python の差異（同一挙動ではない）: Go は失敗行を skip で固定、Python は全件毎回再評価（失敗固定なし・全件再ハッシュ）。指摘の挙動は Go 実装固有。実測数値は上（呼び出し回数・status 列・scan_time 変化）。

### 有効性

有効（機構確定・実測済み）。原文の指摘は Go 実装で成立。追加の同根影響: `unknown_to_vt`（negative cache TTL 2 h）で記録した行も skip で固定され、unchanged では TTL が到達しない（再評価経路が skip の前にあるため）。

### 対応の実現可能性と推奨修正方法

- 案1「記録を残す」（列追加・マーキングのみ・skip 条件不変): 失敗は可視化するが skip 発火のままなので項目を閉じない。マーキングを skip 判定が参照する時点で案2 と同一になる → 単独案として不適。
- 案2（推奨）「skip 条件へ失敗行を含める」: `scan_meta` へ `retry_needed INTEGER NOT NULL DEFAULT 0`（追加のみ・D-03 非破壊迁移）を新設し、評価時に YARA 失敗・VT api_error（`rate_limited`・`no_key`・`auth_error`・`network_error`・`daily_quota_exhausted`）で 1 を立て、成功評価で 0 へ戻す。`scanSkipStatus` は「meta 一致かつ `retry_needed = 0`」のみ skip（YARA 側は `scan_source` の `structural+yara_error` 判定でも可。VT 側は B-07 表現上 scan_source に失敗マーキングがないため、列か表現変更が必要）。
- 任意強化（推奨に含めない): `last_attempt` 列での backoff・回数列（失敗行再評価が VT 4 req/min・500/day を毎 scan 消費する問題の抑止）。

### 副作用・影響範囲・代替案

- 再評価コスト: 失敗行は scan ごとに sha256 再計算（1 回/行/回）＋ YARA runner 再呼出（ext が scan target の行）＋ VT gate 再呼出（予算内）。失敗行が N 件の場合、VT 有効スキャンで毎 scan 最大 N 回の gate 消費（4 req/min・500/day）。失敗率が高いと日次予算を継続消費。案2 の任意強化で抑止可。
- findings は insert-only（D-03）維持: 再評価で `api_error` の再記録（unique key `(path, sha256, rule, detail)` で同一 key は dedup）や later hit の新行が増える可能性。既存行の削除・自動 dedup なし（D-03 正典と整合）。`resolveStaleFindings` は sha 変化時のみ（skip 解除で sha 一致なら影響なし）。
- A-12 固定テスト `TestProcessFileSkipsUnchangedFileBySizeAndMtime`（run2 hash 0・VT 1 回目のみ）は成功評価前提で現行合格条件を維持。案2 の実装時は失敗行が skip されない固定テストの新設が必要（既存テスト変更なし・実装時に承認要）。R-06（`scan_meta` path キー）の正典と整合（同じ表の列追加のみ）。
- 代替案（承認分離): VT 失敗を `scan_source`（例 `structural+vt_error`）で表現して skip 判定源にする案。表示・`hub list` は scan_status/scan_source を出さない（UI HTML に scan 表示なし。`hub list` は unread/favorites/queries・`app/main.go:1355`）。ただし正典表現変更（B-07 条・findings `api_error` 表現の extension）なので、この案は承認を要る側として分離。
- UX: 表示ルールは変わらない。ただし skip 解除により保存値が将来変化し得る（`pass`→`hit`・`not_evaluated`→`pass` 等）。これは B-07「retry 用表現」の意図の実現（データ更新）で、UI 意味の変更ではなく承認不要として扱える。実 DB の既存行は次回 scan（meta 一致行の再評価開始）で自然に変化・強制更新はしない。
- Python 側: 失敗固定は起きない（skip 機構なし）ため修正不要。A-12 の全件再ハッシュ差（Python 参照実装漂移）は別条（B-07 parity 記録で既知）。

### 正典照合（参照先まで辿った結果）

- A-12（`docs/ai/tasks/2026-11-03-a12-sha256-every-scan.md`）: skip の意味は hash 計算回避・gate 回数を増やさない。B-07 decision（`docs/ai/decisions/2026-11-03-implementation-judgment-approval.md`・`docs/ai/decisions/2026-11-03-judgment-waiting-reduction-policy.md`）: `structural+yara_error` を「retry 用表現」として維持と確定。両正典の status 記述の矛盾はなし。行為（skip が失敗行の再評価を禁止）が B-07 の retry 意図と矛盾 → 案2（skip 条件に失敗参照）が B-07 の意図の回復であり、decision 変更なしで実装可。
- D-03（`docs/ai/decisions/2026-11-03-d03-findings-unique-index-migration.md`）: insert-only・非破壊迁移（追加のみ）維持。案2 の列追加・findings 参照の単独案はこれと整合。
- 画面確認: UI 項目でない（scan 表示が画面にない）→ 該当なし。キャプチャ保存なし。

推定規模: 小（schema.sql・`ensureSchema`（列追加）・`scanSkipStatus`・`processFileWithYara`・固定テスト新設 1〜2 件。Python 側変更なし）。

## Verification

- Go CLI 実測（2026-11-08・scratch・外部 API 非呼出・run2 前に約 1.5 s 间隔): 上記実測の summary・status 列・scan_time。fixture・config・DB 状態は `docs/ai/tmp/assess-20261108-1332/`（`go_root_a`・`go_root_b`・`go_cfg_vt.json`・`go_cfg_yara.json`・読取 script `n01_read.py`）。
- Python 実測: `python docs\ai\tmp\assess-20261108-1332\n01_probe.py`（mock fetch・mock runner。呼び出し回数・scan_time 変化を JSON 出力。外部 API 呼出なし）。
- `python -m pytest tests/ -v`（ルート・17.25 s）: 59 passed / 1 failed。failed は既存 known-issue `test_canonical_build_freshness_rule_exists`（`docs/ai/known-issues/KI-20261106-04-canonical-build-freshness-pointer-missing.md`・AGENTS.md pointer 条・本項目非接触・本項目の不合格ではない）。
- `go vet . && go test -count=1 -timeout 300s .`（app・24.423 s・2026-11-08）: vet clean・ok（Go 側変更案があるため実施。本項目はソース無変更）。
- 既存テスト変更なし・本番コードディレクトリへの書込みなし・実 DB・完成フォルダ非変更。スクラッチのみ。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
