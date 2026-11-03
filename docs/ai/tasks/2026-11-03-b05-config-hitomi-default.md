# B-05: hitomi キーのない config で use_live_gg が false

**Status**: 進行中
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`HitomiConfig.UnmarshalJSON`（`app/main.go`）は `hitomi` キーが存在し、`use_live_gg` が未指定のときだけ `UseLiveGG=true` を設定する。`loadConfig`（`app/main.go:649` 相当）は `cfg := Config{}` に `MinIntervalMS` 等の default を入れるが `UseLiveGG` の default がない。実測: `hitomi` キーなしの scratch config（`docs/ai/tmp/assess-20261103-1220/cfg_no_hitomi.json`, port 8732）で canonical exe を起動 → ログ `gg: snapshot fallback (live gg.js unavailable)`、search metrics に `gg` group が現れない（live gg 未使用）。canonical config（`hitomi.use_live_gg: true`）は live gg を使う。対応: `loadConfig` で `UseLiveGG` の default を true（または config キーの有無によらず明示 default）。合格条件: `hitomi` キーなし config で live gg が使われることを mock で確認。推定規模: 小。

## Verification

（記入）

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
