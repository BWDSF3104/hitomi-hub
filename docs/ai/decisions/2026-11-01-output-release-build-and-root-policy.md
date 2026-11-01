# output 配布フォルダ・canonical exe・runtime root の決定

- 決定日: 2026-11-01
- 対象: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`

## 決定

1. `output\hitomi-hub\hub.exe` は正典配布用 exe として常に最新化する。
   - `hub.cpXX.exe` は latest verified build の検証用 checkpoint binary として扱える。
   - ただし正典 exe の地位は `hub.exe` のまま。
   - 有意な更新版、タグ、GitHub release build 時には、latest verified build を `hub.exe` として更新する。
   - `hub.exe` が latest でない状態を残さない。

2. 有意な更新版としてタグを設定するときは、`output` 内の必要ファイルのみを release build として GitHub へアップする。
   - release build folder: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`
   - release build に含めるファイル:
     - `hub.exe`
     - `config.example.json`
     - `schema.sql`
     - `rules\structural.yar`
     - `使い方メモ.md`
     - `run.cmd`（単体 distribution startup script）
     - `LICENSE`（GitHub release に必要な場合）
   - release build に含めないもの:
     - `hub.cpXX.exe`
     - `hub.exe~`, backup files
     - `*.db`
     - `cache/`
     - `Galleries/`
     - `scratch/`
     - screenshots, tmp files, runtime artifacts

3. runtime root は distribution と分離できるが、distribution として運用する場合は root を distribution 内、または明示指定した runtime folder に限定する。
   - `--data-dir` は exe が読む config・schema・rules などの data file 置き場。既定では exe と同じ folder。
   - `--root` は runtime workspace。ここに `hitomi.db`, `Galleries/`, `cache/images/` が生成される。
   - standalone distribution で使う場合の既定:
     - `hub.exe --root .`
     - または `hub.exe --root <distribution内 runtime folder>`
   - `--root` が repo/project scratch や distribution 外にある場合は、その folder は runtime data folder であって distribution folder ではない。開発検証用 root としては許容するが、単体配布フォルダとして運用する場合は修正が必要。

4. 正典サーバー更新規則:
   - `output\hitomi-hub\hub.exe` を更新するときは、正典プロセスを停止し、latest verified build を canonical `hub.exe` として更新し、distribution startup command で再起動してよい。
   - distribution startup script は `run.cmd` とする。
   - `run.cmd` の内容は:
     - `pushd "%~dp0"`
     - `hub.exe view`
     - `pause`
   - これにより script のある folder、すなわち distribution folder を cwd/runtime root として扱う。
   - command-line で直接起動する場合は:
     - `hub.exe --root .`
     - または `hub.exe --root <distribution内 runtime folder>`
   - 開発 screen verification で cache-less runtime root を使う場合、その root は distribution startup と区別し、記録上 dev runtime root として扱う。

## 現在の状態

- `output\hitomi-hub\hub.exe` は latest verified binary と哈希が一致しない。
- latest verified binary は `hub.cp22.exe`。
- 正典サーバーは現在 canonical `hub.exe` から起動しており、正典 exe を更新するには正典プロセスの停止が必要。
- 本記録では exe 更新やサーバー再起動は実施していない。

## 修正判定

- `hub.exe` を latest verified binary として更新する修正が必要。
- distribution startup command の `--root` を distribution 内または明示 runtime folder にする修正が必要。
- 現在の `S:\Documents\Kilo_Commons\Common_Operations\scratch\hitomi-usage-fixed-ongoing` root は cache-less verification 用 dev runtime root であり、単体配布フォルダとしては distribution root として不正。開発では許容、配布としては修正対象。
