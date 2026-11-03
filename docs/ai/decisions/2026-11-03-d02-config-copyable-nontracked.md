# D-02 config path決定（非追跡config.json・fallback・copy simple）

D-02の正典方針は「実keyを置くpathは追跡下ではなく、他の自端末へのコピーが簡単」。

- `config.example.json` はテンプレートとして追跡維持し、`api_key: ""` のまま。
- 既定config pathは `<data-dir>/config.json`。このファイルは `.gitignore` で非追跡。
- `config.json` 不在時のみ `config.example.json` へfallback。既存のkey-less example運用を壊さない。
- `--config` が明示された場合はそのpathを優先。
- keyは `config.json`、または env `VIRUSTOTAL_API_KEY` に置く。key文字列をcommit、task記録、decisions、known-issues、テスト、報告へ書かない。
- `secrets/` も非追跡。

実装: `.gitignore` に `config.json` / `secrets/`、`app/main.go` の `resolveConfigPath(dataDir, explicit)` で `config.json` 優先、missing時 `config.example.json` fallback。
