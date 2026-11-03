# B-05: hitomi キーのない config で use_live_gg が false

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 実装・検証完了（単独コミット・push 済み。検証記録は Verification）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`HitomiConfig.UnmarshalJSON`（`app/main.go`）は `hitomi` キーが存在し、`use_live_gg` が未指定のときだけ `UseLiveGG=true` を設定する。`loadConfig`（`app/main.go:649` 相当）は `cfg := Config{}` に `MinIntervalMS` 等の default を入れるが `UseLiveGG` の default がない。実測: `hitomi` キーなしの scratch config（`docs/ai/tmp/assess-20261103-1220/cfg_no_hitomi.json`, port 8732）で canonical exe を起動 → ログ `gg: snapshot fallback (live gg.js unavailable)`、search metrics に `gg` group が現れない（live gg 未使用）。canonical config（`hitomi.use_live_gg: true`）は live gg を使う。対応: `loadConfig` で `UseLiveGG` の default を true（または config キーの有無によらず明示 default）。合格条件: `hitomi` キーなし config で live gg が使われることを mock で確認。推定規模: 小。

### 修正方法（実装前に確定・承認範囲内）

- `loadConfig`（`app/main.go:158`）の初期化を `cfg := Config{Limits: defaultLimits()}` → `cfg := Config{Limits: defaultLimits(), Hitomi: defaultHitomiConfig()}` へ変更。`hitomi` キーが無い JSON では `HitomiConfig.UnmarshalJSON` が呼ばれないため、この初期化 default（`UseLiveGG: true`）が効く。キーあり時は `UnmarshalJSON`（`app/hitomi.go:169-196`）が全フィールドを raw から上書きするため既存意味（明示 false は false、省略時 true）不変。他のフィールドは loadConfig の per-field default 補完と同一結果で影響なし。
- `HitomiConfig.UnmarshalJSON` は変更しない。viewer・`imageProxy` 順序・永続 cache・shard URL 組み立て・TN・UI full 判定・config の他の default・download・YARA は変更しない。snapshot fallback 経路（`EnsureGG` の `UseLiveGG=false` 分岐と fetch 失敗時 `snapshotGG()`）は維持。
- 固定テスト（`app/main_test.go` に追加）:
  1. `hitomi` キーなし config で `loadConfig` が `UseLiveGG=true` を返すこと
  2. その config の `Client` + mock fetch（gg.js fixture）で `LoadGG` の source が `live`、fetch URL が `gg.js`、metrics の `gg` group に calls=1 が出ることを mock で確認
  3. `hitomi.use_live_gg: false` 明示 config は `UseLiveGG=false` のまま、mock で `LoadGG` source が `snapshot`（fetch 呼出し 0 回）＝ snapshot fallback を引き続き使う

読んだ decision（作業前チェック・フル画像ガード関連）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

## Verification

読んだ decision（作業前チェック）: `docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`、`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`、`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`。

### 固定テスト（mock）

- `app` で `go vet . && go test -count=1 -timeout 300s .` → `ok hitomi-hub/app 36.861s`（修正確定状態。実行 2 回目）
- 新規 `TestLoadConfigWithoutHitomiKeyUsesLiveGGViaMock`（`app/main_test.go`）:
  - `hitomi` キーなし config（`{"viewer":{"port":8732}}`）で `loadConfig` が `UseLiveGG=true` を返す
  - その config の `Client` + mock fetch（fixture gg.js）で `LoadGG` source = `live`、fetch URL が gg.js、calls=1、metricsSnapshot の `gg` group に calls=1
  - `hitomi.use_live_gg: false` 明示 config は `UseLiveGG=false` のまま、mock で `LoadGG` source = `snapshot`（fetch 呼出し 0 回）、bucket は `snapshotGG()` と一致（既存意味・fallback 経路維持）
- 修正前（`main.go` を一時的に `d010178` 相当へ戻して）同テスト実行 → `main_test.go:442 config without hitomi key should default UseLiveGG true: got false` で FAIL（テストが合格条件を固定している証拠）。復元後 PASS。

### pytest

- ルートで `python -m pytest tests/ -v` → 33 passed（3.06s）。`tests/test_canonical_build_freshness.py` 含む（合格。UI 未変更、exe は実機確認のため修正込みで再 build 済み・一致）。

### 実機確認（canonical build・scratch dataDir）

- 実機確認は canonical 永続先を汚さないよう scratch root/dataDir（`docs/ai/tmp/assess-20261103-1220/scratch/`、config は `hitomi` キーなし・port 8732）で実施。canonical server（`output/hitomi-hub/hub.exe`、修正込み再 build 済み・既定 8731）は稼働済み。
- CLI download（実在 gallery 4301441、`-limit 2 -no-vt`）: `attempted=2` と `written=2` 一致。stderr に `gg: snapshot fallback` 警告なし（live gg 使用）。経過 2.42s。
- scratch viewer（8732）`/api/search?term=cat` の metrics に `gg` group: `calls=1, network_ms=444`（live gg.js 取得の客観的証拠）。経過 9.10s。
- viewer `/api/image/35fd0c2ab0521c4baabb55d58e3d0ac076ac5e11fa979874d15e127e33592e5f?format=webp` → `200` / `x-cache: miss` / 139238 bytes（`no_full` でない、1.30s）。`/api/image/137a33922ba807c8b53b88c02f317122076271fbeac388e1f18dfa6417cbe1cf?format=webp` → `200` / `x-cache: miss` / 155314 bytes。
- 実機確認の書き込みは scratch 内（`scratch/cache/images`・`scratch/Galleries`）のみ。canonical server は停止→再 build した exe で `http://127.0.0.1:8731` に復旧済み（稼働中）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
