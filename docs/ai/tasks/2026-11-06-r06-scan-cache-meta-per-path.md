# R-06: scan_cache の size/mtime が sha256 1 組のみで同一内容の別 path の skip が効かない

**実行固有ID**: assess-20261106-1056

**Status**: 完了
**Date**: 2026-11-06
**Next Action**: 完了（A1 path-key scan_meta 実装・合格条件テスト固定 PASS・CLI 実測 run2双方 hash 0）

## 意図 / 背景

A-12（`docs/ai/tasks/2026-11-03-a12-sha256-every-scan.md`）は scan で size/mtime 一致の unchanged を skip（hash 計算回避）する機構を追加した。skip 判定の meta（size・mtime）は `scan_cache` の sha256 1 組（PRIMARY KEY `sha256`、`schema.sql:31-40`）に置く。同一内容の 2 path では meta が 1 行しかなく、後登録の size/mtime が前登録を上書きするため、`scanSkipStatus`（`app/main.go:548-570`）が不一致となり skip が効かない。A-12 記録の「留意」（`2026-11-03-a12-sha256-every-scan.md:23`）で「安全側で再ハッシュに退化（正しさは維持）」と既知化していた点の、実測による有効性確定。

## 調査結果 / 変更内容

### 現在の実装と判断根拠

- `recordScanMeta`（`app/main.go:536-542`）は `ON CONFLICT(sha256) DO UPDATE SET size, mtime` で meta を sha256 1 組へ upsert。同一 sha256 の 2 path では後処理の meta が先行を常に上書きする。
- `scanSkipStatus`（`app/main.go:548-570`）は `scan_cache` の meta を `files` 経由で path に引き、実 stat の size/mtime と比較。不一致は再ハッシュ（安全側・正しさ維持）。
- 帰結は原文より悪い: 2 path を同一 scan 順で処理すると、1 つを再ハッシュした時点で meta がその path の値へ flip し、後続の path も不一致になる。steady state では**双方**が毎 scan 再ハッシュ（skip 完全喪失）。正しさ（findings 再 insert・scan_status 維持・gate 回数）は維持。

### 有効性: 有効（実測で確定）

原文の例（Galleries と acquisition）は機構として要修正。CLI scan は Galleries のみ走査（`app/main.go:1009`）、acquisition は `recordFile` 記録のみ（`app/acquisition.go:224`）で `recordScanMeta` を呼ばないため、acquisition path は meta を上書きせず `scanSkipStatus` にも入らない。実際のトリガーは processFile を通過する 2 path（例: 同一内容の file が Galleries 下 2 gallery folder に存在、download 書き込みの processFile 経由）。機構（sha 1 組 meta・上書き・path 比較）は有効。

### 実測（scratch・`docs/ai/tmp/assess-20261106-1056/`）

同一内容（sha256 `2ed017d4…c9f9a`・61 B）の 2 path（`Galleries\110001\dup.bin`・`Galleries\110002\dup.bin`）、mtime 差 87 s（1793966459 / 1793966546）。canonical build（scratch `hub.exe`・`--no-vt`・YARA off・scratch data-dir へ schema.sql コピー）。hash 計算回数 = 処理 file − skipped（skip 時 hash 0）。

- run1: `skipped=0`・hash 2（both 記録。meta = 後処理の dup2 の mtime）
- run2（mtime 一致 control）: `skipped=2`・hash 0 → skip 機構自体は単一 path 条件で有効（A-12 固定テストと整合）
- run3（dup2 mtime 変更直後）: `skipped=1`・hash 1。meta（1793966459）と一致する dup1 は skip、dup2 再ハッシュ → meta flip（1793966546）
- run4・run5（steady state）: `skipped=0`・hash 2（双方再ハッシュ）。scan_cache meta は 1 組（dup2 の mtime のまま）、files 両 path の scan_time が update（= hash 実測）

→ 合格条件（双方 hash 0）は現実装で**未充足**。実測で有効と確定。

### 対応の実現可能性と推奨修正

- 推奨 A1: path キーの meta 表を新設（例 `scan_meta(path PRIMARY KEY, size, mtime)`）。`recordScanMeta` を rel path キーの upsert へ、`scanSkipStatus` を path 直接参照へ変更。`scan_cache` は VT verdict 表として sha256 キー維持（`cacheLookup` 意味不变）。迁移は追加のみ（`CREATE TABLE IF NOT EXISTS` + `migrateSchema` 段階・schemaVersion 7→8・D-03 非破壊規約）で、既存 DB の pk 変更・行再構築は不要。既存 scan_cache meta 値は path に復元不能だが、修正後の 1 回目 scan は再ハッシュ（安全側）で機能上問題なし。
- 代替 A2: `files` へ `mtime` 列追加（非破壊 ALTER）し skip 判定を `f.size, f.mtime` へ。ただし `recordFile` の conflict-update が size を更新しない（`app/main.go:581-583`）ため size 陳腐化修正が必要。files 意味変更が広がるため A1 を推奨。
- 却下 B: `scan_cache` pk を `(sha256, path)` へ変更 = 表再構築（不可逆）。既存 VT verdict 行に path がなく meta を復元できず、D-03（定義置換のみ・既存行削除禁止）に反する。ユーザー承認なしの実施不可。

### 副作用・影響範囲

- 既存テストの固定: A-12 固定テストは単一 path の skip を固定し、本挙動（sha meta 上書き）はテストされていない（照合済み）。ただし `TestProcessFileSkipsUnchangedFileBySizeAndMtime` の「scan_cache record なし（行削除）→ 再ハッシュ」（`app/main_test.go:978-995`）は scan_cache を meta 源として固定しており、A1 実施時は同一テストの source 変更が必要（テスト変更=実施時の作業、現 assessment では変更しない）。`TestProcessFileRehashesOnSizeMismatchAndExternalRemoval`（`app/main_test.go:1039-1049`）は path 差の初回ハッシュのみで A1 後も整合。
- Python 参照実装は skip 分岐なし（`src/scan.py` に mtime/skip 機構なし）→ 影響なし。schema.sql 追加のみ。
- findings・VT gate・negative cache・scan_source 表現は変更なし（skip は gate 回数を増やさない性質を維持）。

推定規模: 小（schema.sql・migrateSchema・`recordScanMeta`・`scanSkipStatus`・固定テスト 1 件の更新）。

### 実装（2026-11-07・採用案 A1）

- 採用: 推奨 A1（path キー meta 表 `scan_meta(path PRIMARY KEY, size, mtime)` 新設）。根拠: 追加のみで D-03 非破壊迁移規約に収まる。`scan_cache` は VT verdict 表として sha256 キー維持。
- 変更点: schema.sql へ `scan_meta` 追加（新表・既存表 PK 変更なし）。`schemaVersion` 7→8・`migrateSchema` に `ensureScanMetaTable`（CREATE TABLE IF NOT EXISTS のみ）追加。`recordScanMeta` を path キーの scan_meta upsert へ、`scanSkipStatus` を path 直接参照（scan_meta JOIN files）へ変更。呼び出し側 `processFileWithYara` は rel path を渡す。
- 記録と実コードの差異（軽微・実装継続）: 記録の `src/schema.sql` 参照は正典構造記載由来。実ファイルはルート `schema.sql`（`TestDataFilesAtRoot`・`ensureSchema` の dataDir 参照先）。scan_cache 定義の行番号 31-40 は実ファイルと一致。
- 固定テスト: `TestProcessFileSkipsUnchangedFileBySizeAndMtime` の「record なし（行削除）」源を scan_cache → scan_meta（path キー）へ更新（記録済みの実施時作業）。R-06 合格条件固定テストを 1 件追加（同一 sha256 の 2 path・1 scan 後 2 scan で双方 hash 0）。
- `scan_test.go:622` のコメント（「Go 側は scan meta 行（verdict=''）を常に書く」）は本変更後 stale になるが、アサート（pass verdict 行なし）は有効なため他項目テストには触れない（scan meta 行は scan_meta へ移動）。

## Verification

### 実装検証（2026-11-07・試行 1 回で合格）

- `go vet .`（app）: clean。
- `go test -count=1 -timeout 300s .`（app）: ok 25.808s（全 PASS）。内訳確認（単独 -run）: `TestProcessFileSkipsPerPathForSameSha256AcrossTwoPaths`（R-06 合格条件固定・追加）PASS 0.04s・`TestProcessFileSkipsUnchangedFileBySizeAndMtime`（源更新 scan_meta へ）PASS 0.05s・`TestProcessFileRehashesOnSizeMismatchAndExternalRemoval` PASS・`TestMigrateSchemaAddsScanCacheSizeMtimeWithoutDeletingRows` PASS（scan_cache 列追加迁移不変）。
- 追加テストの合格条件固定: 同一 sha256 の 2 path（同一 bytes・mtime 差）→ scan 1 で hash 2・scan 2 で hash 0（双方 skip・skipped=2・findings 同一・scan_meta 2 行で mtime を path ごとに保持）。
- `python -m pytest tests/ -v`（root）: 56 passed・1 FAILED（12.09s）。失敗は現存既知 failure `test_canonical_build_freshness_rule_exists`（KI-20261106-04・AGENTS.md pointer 欠落）。R-06 と無関係（本項目で修復しない・AGENTS.md は write-protected）。pytest は schema.sql の scan_meta 追加（ensure_schema の executescript）で影響なし（Python 参照実装に skip 分岐なし）。
- CLI 実機確認（throwaway build `hub.exe`・scratch `docs/ai/tmp/assess-20261106-1056/implement-r06/`・`--no-vt`・YARA off（config.example.json yr null）・scratch data-dir へ schema.sql・config.example.json コピー・正典 DB・完成フォルダ未使用）: 同一内容 61 B の 2 path（`Galleries\110001\dup.cbz`・`Galleries\110002\dup.cbz`・mtime 差 90 s）。run1 `skipped=0`（hash 2）。run2 `skipped=2` → 双方 hash 計算 0（合格条件充足・修正前は steady state で skipped=0）。run3（dup1 mtime 変更のみ）`skipped=1`・rehash 1: dup2 の path 別 meta が保持され skip 維持（sha 1 組 meta の flip 影響がないことの実測）。run4 安定後 `skipped=2`。出力は run1→run4 の順で一致。脚本 `r06_check.ps1`・output 記録（scratch 内）。画面確認: UI 非関連のため該当なし。
- 正典 build（output/hitomi-hub/hub.exe）の再 build は本項目の合格条件に既定されていない（UI 非関連・Commit Checklist の UI 条非該当）。

### 調査時の検証（2026-11-06）

- `go vet .`: clean（app・2026-11-06）
- `go test -count=1 -timeout 300s .`（app）: ok 25.963s（全 PASS・既存テストが本挙動を固定していないことを suite pass で確認）
- `python -m pytest tests/ -v`（root）: 54 passed・1 FAILED（10.26s）
  - 失敗は `test_canonical_build_freshness_rule_exists`（AGENTS.md の search-viewer separation decision ポインタ欠落）。R-06 と無関係の現存失敗。未確認（本項目で修復しない）
- 実測: 上記「実測」節。run 時刻 21:01:28→21:01:29（run1 1.5 s）、run2〜run5 各 <1 s、check_meta.py 出力保存（scratch 内）。実 DB・完成フォルダ・既存テスト未変更。画面確認: UI 非関連のため該当なし（未確認ではなく該当なし）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
