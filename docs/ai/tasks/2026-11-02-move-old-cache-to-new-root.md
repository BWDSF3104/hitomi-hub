# Move old cache to new distribution runtime root

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: old canonical cache moved to output runtime cache; new cache preserved; canonical server restarted

## 意図 / 背景

Canonical distribution startup root が `output\hitomi-hub` になったため、旧 canonical cache を新しい distribution runtime cache へ移動する。既存 new cache を失わないため、move 前に新 cache を scratch backup へ退避し、旧 cache を新しい location に置いた。

## 調査結果 / 変更内容

Old cache source:

```text
S:\Documents\Kilo_Commons\hitomi-hub\scratch\live\cache
```

New distribution cache destination:

```text
S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\cache
```

Existing new cache was preserved at:

```text
S:\Documents\Kilo_Commons\hitomi-hub\scratch\backup\output-cache-before-old-cache-move
```

Move actions:

1. Canonical server stopped.
2. Existing `output/hitomi-hub/cache` moved to `scratch/backup/output-cache-before-old-cache-move`.
3. `scratch/live/cache` moved to `output/hitomi-hub/cache`.
4. Canonical server restarted with distribution startup root:
   `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --root S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`

## Verification

Before move:

- `scratch/live/cache`: 253 files
- `output/hitomi-hub/cache`: 13 files

After move:

- `output/hitomi-hub/cache`: 253 files
- `scratch/backup/output-cache-before-old-cache-move`: 13 files

Canonical process after restart:

- process: `bgp_197fca661001eNrAINzA6ljfxl`
- URL: `http://127.0.0.1:8731`
- runtime root: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
