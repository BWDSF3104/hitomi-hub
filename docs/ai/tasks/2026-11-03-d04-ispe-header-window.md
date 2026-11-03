# D-04: IsPE が先頭 64 バイトしか見ない

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

（記入）

## 調査結果 / 変更内容

有効（実測）。`src/scan.py` の `is_pe`/`scan_file` は先頭 64 bytes しか読まない（`readHead(path, 64)`）。`e_lfanew = 0x80` の MZ ファイル（PE signature を 0x80 に置く）で `mz_without_pe_header` が hit（`Galleries\\102\\b.bin`）。対応: `e_lfanew` を readHead（4 bytes）で读取し、signature offset まで seek。合格条件: `e_lfanew >= 64` の fixture（scratch script）で `mz_without_pe_header` が出ない。推定規模: 小。

### 変更内容（実施）

- `src/scan.py` `is_pe(path, head)`: `e_lfanew` を head（0x3C–0x40 の 4 bytes）から読み、`e_lfanew + 4 <= len(head)`（=64、窓内）なら head 内 slice で判定（従来経路維持）。窓外（`e_lfanew >= 64`）は `path` を seek して signature offset で 4 bytes 読み `PE\x00\x00` を判定。seek 失敗（OSError）は False。`e_lfanew <= 0` は False のまま。`len(head) < 0x40`（64 bytes 未満の readHead）は e_lfanew が読めないので False（真の MZ-only として hit 側）。EOF 超え offset は seek 後 read が空になり False（巨大値での全読みなし）。
- `scan_file`: `is_pe(path, head)` へ変更（path を渡す）。`detect_magic`・YARA 規則・viewer・Go は変更なし。
- 実測で `Galleries\102\b.bin` は現マシンに存在しなかったため、合格条件 fixture はスクラッチスクリプト `docs/ai/tmp/assess-20261103-1220/d04_fixture_check.py` で生成し実測。
- テスト固定（`tests/test_scan.py`）: `make_pe_stub` fixture 追加 + 4 テスト（0x80 有効 PE は non-hit、0x38 窓内 regression、0x80 に signature なし stub は hit、真の MZ-only は hit）。

### YARA 基準確認（規則は変更しない）

規則名は `MZ_Header_At_Offset0`（`mz_without_pe_header` という規則は存在しない）。条件は `$mz at 0` のみで、PE 判定は「stays in scan.py」と明記（規則 description）。修正後の fixture 実測（`docs/ai/tmp/assess-20261103-1220/d04_yara_check.py`）: f1（有効 PE・e_lfanew=0x80）で YARA hit `[MZ_Header_At_Offset0]`、f3（stub）でも hit。つまり YARA は「MZ ヘッダの有無」、Python は「PE signature の実在」を判定する互补関係で、今回の修正による基準の食い違いは発生しない（規則側の修正は不要）。なお process_file では YARA は SCAN_TARGETS 拡張子のみ走査（MZ-at-0 規則が scan target に hit するケースは MZ スプープされた zip/pdf 名ファイルに限定される）。

## Verification

- 修正前実測（スクラッチ `d04_fixture_check.py`、2026-11-03 09:01）: f1（`e_lfanew=0x80`、PE signature at 0x80）→ `status=hit` / `findings=['mz_without_pe_header']`（バグ再現）。
- 修正後実測（同一スクリプト、2026-11-03 09:02）:
  - f1 `e_lfanew=0x80`（有効 PE）: `findings=[]` / `status=not_evaluated` → `mz_without_pe_header` 非 hit（合格条件満たす）
  - f2 `e_lfanew=0x38`（窓内 PE）: `findings=[]` / `not_evaluated`（既存挙動維持）
  - f3 `e_lfanew=0x80`・signature なし stub: `hit` / `mz_without_pe_header`（検知維持）
  - f4 `e_lfanew=0`（真の MZ-only）: `hit` / `mz_without_pe_header`（既存挙動維持）
- `python -m pytest tests/ -v`（ルート、2026-11-03 09:18 再実行、3.35s）: **33 passed**（`test_canonical_build_freshness.py` 2 件含む・合格。UI 未変更につき exe 再 build 不要、既存 exe が現行 ui.html を埋込み済みで合格）。中断（09:01）はテスト実行中の割り込みであり、再実行で完走。
- Go 変更なし（`app` 検証追加不要）。YARA fixture テスト（`test_run_yara_rules_on_fixture`）含め既存テスト全通過。
- 画面確認: 本項目は UI 変更ではないため不要（ルール上）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
