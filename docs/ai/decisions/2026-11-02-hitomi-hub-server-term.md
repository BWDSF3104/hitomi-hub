# hitomi-hub で「サーバー」という場合の正典解釈 (2026-11-02)

Status: active.

## 決定

`hitomi-hub` プロジェクトで「サーバー」と言う場合、基本的に `hub.exe` によって起動される hitomi-hub viewer server を指す。

- 正典 exe: `output/hitomi-hub/hub.exe`
- 正典 port: `8731`
- 確認対象: `hub.exe` process と `8731` listener。
- OS 上に残る VS Code、Kilo extension、Steam、Google Drive File Stream、MSI、system service などの unrelated loopback listener は、hitomi-hub server ではない。

## 運用

「他は起動していないことを確認」という依頼は、通常 `hub.exe` server の重複起動確認として扱う。 unrelated OS/app listener を hitomi-hub server と混同しない。
