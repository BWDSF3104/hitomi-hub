# B-08: zip の .. 判定が部分一致で誤検知

**Status**: 完了
**Date**: 2026-11-03
**Backlog**: assess-20261103-1220
**Next Action**: 引き継ぎ完了。実施開始指示待ち（実行計画順・サブエージェント1項目ずつ）

## 意図 / 背景

zip entry 名の `..` 判定が部分一致のため、正当な名前（`Vol...01.webp`）で path_traversal 誤検知し file status hit になる（hit は export から除外される）。判定を path segment 単位へ是正し、Go / Python 参照実装 / YARA の基準を揃える。

## 調査結果 / 変更内容

有効（実測）。`app/scan.go:132`（`containsTraversal(name)`）と `rules/structural.yar` の `contains ".."` は `..` を部分一致で hit する。実測: zip entry 名 `Vol...01.webp`（正当）で finding `path_traversal:Vol...01.webp`、file status `hit`（hit は export から除外される）。対応: path separator を跨ぐ `..` セグメントのみ判定（例 `strings.HasPrefix`/segment 解析）、YARA rule も segment 化。合格条件: `Vol...01.webp` で hit しない、`../evil.exe` で hit する（既存テストは `../` のみ）。推定規模: 小。

実施（2026-11-03・実行固有ID assess-20261103-1220）:

- 基準（3実装共通）: entry 名を separator（`/` と `\`）で分割し、segment が `..` と完全一致する場合のみ hit。部分一致（`Vol...01.webp`、`..evil.webp`）は hit しない。leading `/`、`\`、drive prefix 判定は変更なし。
- `app/scan.go`: `containsDotDotSegment(name)` を追加（`strings.FieldsFunc` で `/ \|` 分割）し、`ScanZipEntries` の部分一致 `strings.Contains(name, "..")` を置換。
- `src/scan.py`: `has_dot_dot_segment(name)` を追加（`re.split(r"[/\\]", name)`）し、`scan_zip` の部分一致 `".." in name` を置換（同款の部分一致判定が存在するため同一基準へ揃えた）。
- `rules/structural.yar` の `Zip_Path_Traversal`: `contains ".."` を `matches /(^|[\/\\])\.\.([\/\\]|$)/` へ変更（YARA-X では `for any entry in zip.entries : ( ... )` の括弧が必要。`entry.file_path matches $seg` 形式は構文エラー、インライン regex は有効と実測確認）。

食い違い（実装前に確認）: 記録は `app/scan.go:132` を `containsTraversal(name)` と記載するが、実際の scan.go の判定はインラインで、`containsTraversal` は `app/export.go:174`（export 名のガード）にある。承認範囲は `app/scan.go` のため scan.go 側のみ segment 化し、export.go の部分一致判定は変更していない（export 名は `%03d_%s.%s` 生成で `..` segment になり得ないが、基準は旧来的な部分一致のまま）。

テスト（合格条件固定）:
- `app/scan_test.go`: `TestZipDotDotIsPathSegmentOnly`（9 entry 名 + status 2 件）
- `app/yara_test.go`: `Vol...01.webp` で `Zip_Path_Traversal` 非 hit の追加アサート
- `tests/test_scan.py`: `test_zip_dotdot_is_path_segment_only`（status/findings + helper 8 名）、`test_run_yara_rules_on_fixture` に `Vol...01.webp` 非 hit

## Verification

コマンドと実測（2026-11-03）:

1. `go vet . && go test -count=1 -timeout 300s .`（app）: `ok hitomi-hub/app 36.919s`。個別: `TestZipDotDotIsPathSegmentOnly` PASS 0.02s、`TestRunYaraExecutionPath`（tools/yr.exe yara-x-cli 1.22.1 存在・skip なし）PASS 0.14s。
2. `python -m pytest tests/ -v`（root）: `26 passed in 3.33s`（`tests/test_canonical_build_freshness.py` 2 件 PASS 含む。UI 未変更のため exe 再 build 不要、canonical exe は現行 ui.html を埋込み）。
3. 実 YARA 実行（`tools\yr.exe scan rules\structural.yar docs\ai\tmp\assess-20261103-1220\fixtures --output-format ndjson`）: hit した entry 名 = `../evil.exe`（Zip_Path_Traversal + Zip_Executable_Entry）、`a/../b.webp`、`a\..\b.webp`、`foo/..`、`..`。hit しなかった entry 名 = `Vol...01.webp`、`..evil.webp`、`normal/01.webp`。

合格条件の実測:

| entry 名 | Go findings / status | Python status | YARA rules |
|---|---|---|---|
| `Vol...01.webp` | findings なし / `pass` | `pass`（findings []) | hit なし |
| `../evil.exe` | `path_traversal:../evil.exe`（+`executable_entry`）/ `hit` | `hit`（`path_traversal:../evil.exe`） | `Zip_Path_Traversal` hit |

3実装の判定基準は segment 完全一致で一致（上表＋テスト表の同一 entry 名集合で確認）。

未変更・判断要: `app/export.go:174` の部分一致 `containsTraversal`（上記）。UI 変更なし、画面確認は該当なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
