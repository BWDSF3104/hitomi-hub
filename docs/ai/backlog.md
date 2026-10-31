# Backlog

## P1

- #1 単体静的バイナリ化（Go + `modernc.org/sqlite`＝pure-Go・CGO なし）。dev toolchain の導入承認が必要。FTS5 を設計に入れるなら pure-Go ドライバの対応確認が前提
- #2 既読・お気に入りの書き込み口（view 時に `read_state` 更新、最終ページ index を保存）
- #3 検索層（hitomi の非公式 endpoint。呼出し間隔・再試行方針を decisions に記録）
- #4 `yr`（YARA-X）導入時の規則実行検証。digest 照合後、規則単位の検証を追加

## P2

- #5 CBZ/PDF エクスポート（DL 経路ができてから）
- #6 UI は 1 ページのローカル Web（ブラウザ既存。native GUI は CGO・webview 依存で移植性を壊すため不可）
- #7 デバイス間同期（単体 exe 運用では後回し）
