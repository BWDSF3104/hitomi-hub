# 外部 API レート制限

| 先 | 制限 | 運用 |
|----|------|------|
| VirusTotal public API | 4 req/min・500 req/day | hash-only 照会のみ（`GET /files/{sha256}` は内容を送信しない）。SHA256→verdict を `scan_cache` に永続化し、再ダウンロードはキャッシュヒットで 0 呼出し。アップロードは既定無効。テストは mock |
| hitomi.la | 公式 API なし（nozomi・Cloudflare 後）。**server 側の minimum interval は観測されず**（2026-11-05 実測: 100 ms 間隔・6 並行で 429 なし・rate 関連ヘッダなし・ltn 経路 `server: nginx`）。値はプロジェクト自己方針 | 本行は数値を再掲しない（正典単一出力元: `app/hitomi.go` の `defaultHitomiConfig()`。`bg_grace_ms` は `const defaultBgGraceMS`）。config key は `hitomi.min_interval_ms` / `hitomi.image_min_interval_ms`。方針（現値・retries・backoff・block signal と `Retry-After`）は decision `docs/ai/decisions/2026-11-05-rate-values-and-concurrency-change-approved.md` へ参照（数値は同 decision と正典単一出力元へ）。テストは mock（`Client.fetch` 注入）。実測記録: `tasks/2026-11-05-assess-20261105-1010-rate-limit-verification.md` |

テストは外部 API を実際に呼ぶことを許可する（`hitomi-hub` のプロジェクト例外。決定: `docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`）。既存の mock テストは可能な限り mock のまま維持し、実呼出しを使うテストはレート制限と API 方針を守る。
