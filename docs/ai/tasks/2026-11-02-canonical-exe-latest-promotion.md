# canonical hub.exe latest promotion

**Status**: 完了
**Date**: 2026-11-02
**Next Action**: stop canonical server, update hub.exe to latest verified binary, restart distribution startup

## 意図 / 背景

正典配布用 exe `output\hitomi-hub\hub.exe` を latest verified binary として更新し、distribution startup command で正典サーバーを再起動する。

## 調査結果 / 変更内容

- Latest verified binary was `hub.cp22.exe`.
- Canonical `hub.exe` was not latest before this task.
- Canonical process was stopped to allow `hub.exe` update.
- `hub.cp22.exe` was promoted to `hub.exe`.
- Canonical server was restarted with distribution startup root `output\hitomi-hub`.
- Startup command:
  `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --root S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`
- No `--data-dir` was specified because default data dir is exe folder, which is distribution folder.

## Verification

- `go test .`: pass.
- `go vet .`: pass.
- SHA-256 after promotion:
  - `hub.exe`
  - `hub.cp22.exe`
  - `7DB2AD39F9FA6B06D7C9E0B914685E48E47534FB1D0F994C2D14CB0720BBE084`
- Canonical process after restart:
  - process: `bgp_197f92c38001b2Yxy40P823zf9`
  - URL: `http://127.0.0.1:8731`
  - runtime root: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
