# Playwright tool output dir の掃除常任承認（2026-11-10）

- ユーザー決定（2026-11-10）: tool output dir `C:\Users\fujim\AppData\Local\Temp\kilo-playwright-mcp\` は Drive 保存後、正典 `kilo-recycle-external <path> -Approved` でゴミ箱へ送る。このフォルダ限りの `-Approved` は常任承認に基づき実行のたびにユーザー再確認しない（正典 tool の gate・`-Approved` 引数自体は有効のまま）。
- global AGENTS（正典）に条として反映。個別の decision file 参照は global 条に置かない（ユーザー指示 2026-11-10）。
- tool の規定動作は変更不可（filename 省略時・実運用で相対 filename 指定時も tool server の output dir へ書かれる）。操作側 lever は相対 filename の root 指定のみ。
- 実測根拠（2026-11-10 現測・掃除前）: 1,036 件（yml 798・log 205・png 28・txt 5）。そのうちこのセッションで削除済み 15 件（n15 4・n06 2・n10 9・`kilo-recycle-external -Approved` 実行確認・commit `73175ce` に反映）。
- 実行確認（2026-11-10）: `docs/ai/tmp/n25-temp-cleanup.ps1`（正典 tool を per-file 呼び出）・ok=1036 fail=0。出力ファイルは未保存（stdout のみ）。
- 影響: 各 task 記録の「project root 外残件（削除せず記録）」条・「追加承認要」条は本条により以後の執行で置換（原文は歴史的記録として保持・この entry 参照）。global AGENTS commit `8b2e94e`（push 済み・実測確認）。
