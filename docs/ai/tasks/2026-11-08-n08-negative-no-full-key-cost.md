# N-08: negativeNoFullKey が呼出ごとに case 値をソート・連結（约 12KB）

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: `negativeNoFullKey` が呼ぶたびに約 2000 件の case 値をソートして連結している。image と TN の各リクエストで `canUse`、`imageFetchKey`、`clear`、`remember` と複数回走る。key は約 12KB の文字列になり、`noFullCache` が最大 1024 件で約 12MB を保持する。GG の parse 時に fingerprint（ハッシュ）を一度だけ計算して使えば解消する。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-08 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（確度: 確定・コード照合済み）

- `negativeNoFullKey`（`app/hitomi.go:297-317`）: 呼出ごとに `gg.CaseSet` をスライス化 → `sort.Ints` → `hash|format|gg.Source|gg.Bucket|case値` を連結。key は 1 回あたり O(n log n) ソート + O(n) 連結。
- 呼出経路（call site 照合）: `imageFetchKey`（`app/hitomi.go:319-321`・`app/viewer.go:1029`）、`tnFetchKey`（`app/hitomi.go:325-327`・`app/viewer.go:991`）、`canUse`（`app/hitomi.go:348-349`・`app/viewer.go:1088`）、`clear`（`app/hitomi.go:368-369`・`app/viewer.go:1087,1096,1115`）、`remember`（`app/hitomi.go:380-381`・`app/viewer.go:1125,1133`）。
- 1 リクエストあたりの呼出数（実測 3 -run・call site 列挙）: TN=1、image negative hit=1、image miss→success=3（canUse+imageFetchKey+clear）、image 404=3（canUse+imageFetchKey+remember）、404+gg 変化 retry 最大=4（imageFetchKey 2 回込み）。`clear` と `remember` は同一リクエスト内で排他（成功時 clear・404 時 remember）なので、項目原文の「4 関数すべてが每回走る」は機構は正しいが 1 リクエスト典型 3 回・最大 4 回。

### 有効性: 有効（実測で確定・数値に補正あり）

スクラッチ実測（`docs/ai/tmp/assess-20261108-1332/r08/`・`go test -overlay`・外部 API 呼出しなし・server 起動なし・正典コード非触）:

- case 数: snapshot `ggCaseSet` n=**2112**（原文「约 2000」は live 窓の実測範囲 1984–2112（`decisions/2026-11-01-ggset-generation-and-drift.md`）と一致・妥当）
- key 長: 実測 **10,070 bytes**（64+format+source+bucket+case 連結）。原文「約 12KB」は **約 20% 過大**（同じオーダー）
- memory: `noFullCache` cap 1024 件の key bytes 合計 **10,311,680 bytes = 9.83 MiB**（+ string header 16B×1024・map bucket 概算で合計 ~9.9 MiB）。原文「約 12MB」も約 20% 過大
- CPU: 現行 **164–178 µs/呼出**（3-run）。typical image request（3 呼出）= **491–533 µs/request**（1000 request で 491–533 ms）
- fingerprint 案（parse 時 1 回 SHA-256・hash 前に sort 済みで順非依存・hash 入力 7,868 bytes）: 1 回 **17.6–515 µs**（cold 515 µs・warm 17.6 µs）、key 生成 **144–146 ns/呼出**（key 長 155 bytes・cap 1024 で key bytes 0.151 MiB → memory 9.83 MiB → 0.151 MiB）。1000 image request 比較: **533 ms → 0.45 ms（約 1,100–1,185x 削減）**。lookup+build（1000・cap 1024 到達後）: 現行 162–167 ms → fp 1.0 ms

### hit 判定意味の等価性・衝突

- hit 判定意味（同一 case 集合 → 同一 key）は fingerprint で維持: hash 前に sort 済みで map 反復順非依存（実測で順序不同→同一 fp を確認）。別 case 集合 → 別 key も実測確認。
- 現行 key は単射（case 集合の完全なエンコード）。fingerprint は単射でなくなり、理論上の誤 hit は hash 衝突のみ: 全 SHA-256（128-bit×2）なら 1024 entry 内衝突確率 ~2⁻¹²⁸ 量で実務上無視。64-bit 截断（xxhash64 系）なら 1024 entry で ~5.5×10⁻¹⁴（非ゼロ）。推奨は全 SHA-256。
- 既存テスト照合（変更なし）: `hitomi_test.go:133,176,190`（key を oracle として使用・byte 列固定なし）、`viewer_test.go:814`（`imageFetchKey == negativeNoFullKey` の同一性固定、817 format 分離、820 gg 分離）、`viewer_test.go:4435`（tnFetchKey を oracle 使用）、`viewer_test.go:622`（negative hit→x-cache `no_full`）。いずれも key の byte 列・連結形式を固定しておらず、key 生成関数自身の意味（同一 case 集合→同一 key・format/gg 分離）を固定するテストのみ。fingerprint 化でも全テスト通過（単射性→衝突確率の差のみ）。

### 正典 condition 照合（破らない）

- `noFullCache` bounded（C-05・`known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`・Glob で実在確認済み）: cap 1024 **entry 数**・stale-only・active protected の意味。fingerprint は entry 数・clean 経路（`cleanStaleNoFullCacheLocked`）を一切変えない。bounded 条件は entry 数ベースなので byte 縮小は条件に抵触せず（memory は 9.83 MiB→0.151 MiB に改善）。
- gg live 既定・snapshot fallback・bucket/switch 約 1 時間回転: fingerprint は parse 時 1 回（snapshot は precompute・sync.Once 相当）で窓 rotation ごとに再計算。source 意味非触。
- x-cache（miss/cache/local）判定・no_full 意味・TN display fallback 禁止・TN バイトを full cache に保存しない・full actual pixels 意味: hit 判定経路の byte 列が変わるだけで、`writeNoFull`・x-cache 出力・TN path 意味は不变。
- UI 項目でないため画面確認は不要（表示源・x-cache 判定を壊さない案のみ）。画面キャプチャ・Drive 保存なし。

### 正典ルール変更該当性の分離

- key 構成 `hash|format|gg.Source|gg.Bucket|sorted CaseSet` の記載はタスク記録 `docs/ai/tasks/2026-11-03-c08-no-full-storm.md`（正典 decision・AGENTS.md condition ではない）。decision（上記 2 件）と AGENTS.md condition が固定するのは**意味**（hit 判定・x-cache・TN・full 経路・bounded entry 数）であり、key の byte 表現ではない。
- したがって fingerprint 化は decision/AGENTS.md の正典ルール変更には該当しない（c08 タスク記録の記載更新は実装時に必要）。ただし「key 構成変更」そのものは実施ゲート（実装開始指示）を要る。衝突による完全性保証を要求する場合は key 構成を変えない代替案（下記 3）が承認不要の安全案。

### 推奨（複数案・最も適切を推奨）

1. **推奨 A（fingerprint・実測で 1000x 削減・memory 9.83 MiB→0.151 MiB）**: `GG` に `Fingerprint string` フィールド追加（`parseGG` で 1 回計算・snapshotGG は precompute）。key = `hash|format|source|bucket|sha256hex(sorted case values)`。単射性ではなく衝突確率（全 SHA-256・実務上無視）で hit 判定を担保。テスト変更不要（oracle テストのみ）。実装規模: 小（S・約 30–50 行）。`sameGG`・gen script（`tools/gen_ggset.py`）は非触で可（snapshot fp は実行時 precompute）。
2. 案 B（memoize・key 構成不変・CPU のみ改善・単射性維持）: GG インスタンス単位で sorted 連結文字列を 1 回だけ生成（`parseGG` 時・snapshot は precompute）。key は byte 列同一、テスト・意味完全不変。ただし memory 9.83 MiB は残る（C-05 condition 上は問題なし）。
3. 推奨の適用条件: memory 縮小も目的なら A、単射保証（key の意味を byte 列レベルで不変としたい）なら B。B は正典ルール変更・承認分離が不要。

### 副作用・影響範囲

- `imageFetchKey`/`tnFetchKey`（fetch dedup group key）が同一 builder 共有なので、negative cache hit 判定と join 判定は同一 key 変化で整合（group key 短縮で group map memory も同比例縮小）。TN 意味（variant suffix 追加）は維持。
- `x-cache` 出力文字列（miss/cache/no_full）・gate lane・singleflight・generation guard・full 経路順序は非触。
- 観測性: key の人間可読性（デバッグ・テスト failure メッセージの key 文字列）が 12KB 文字列から hash に変わる（ diagnostique の可読性低下のみ・機能影響なし）。

## Verification

- `go vet .`（app）: 合格（出力なし）。経過: 約 1–3 s
- `go test -count=1 -timeout 300s .`（app・正典コード変更なし・通常 suite）: `ok hitomi-hub/app 37.925s`（全通過）。スクラッチ measurement（`-overlay` 注入・外部 API 呼出しなし）: 3-run PASS（0.5–0.6 s/-run）
- `python -m pytest tests/ -v`: **59 passed / 1 failed**（12.35 s）。failure は既知 `test_canonical_build_freshness_rule_exists`（非接触・本項目と無関係・記録どおり）
- UI 項目でないため画面確認・screenshot・Drive 保存なし（不要として記録）
- fingeprint 案の hit 判定意味・正典 condition（bounded entry 数・x-cache・TN・live gg）非触は上記「正典 condition 照合」で確認済み。実装前の実機確認（`/api/image/<hash>?format=webp` の x-cache・実在 gallery download）は実装項目（承認後）の合格条件として残す（本項目は調査のみ・実装していない）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（親の `task_manager` 側）
- （該当なし）解決した `known-issues/` ファイルなし — C-05 参照ファイルは archive 実在確認済み（Glob 根拠）。本項目は known-issues 新規作成なし
- （該当なし）`backlog.md` の項目なし（N-08 は claude review findings 記録の項目）
- 実装実施時は `docs/ai/tasks/2026-11-03-c08-no-full-storm.md` の key 構成記載（sorted CaseSet）が key 構成変更の記載更新対象（推奨 A 採用時）
