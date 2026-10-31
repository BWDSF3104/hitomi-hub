# 外部 API レート制限

| 先 | 制限 | 運用 |
|----|------|------|
| VirusTotal public API | 4 req/min・500 req/day | hash-only 照会のみ（`GET /files/{sha256}` は内容を送信しない）。SHA256→verdict を `scan_cache` に永続化し、再ダウンロードはキャッシュヒットで 0 呼出し。アップロードは既定無効。テストは mock |
| hitomi.la | 公式 API なし（nozomi・Cloudflare 後） | 呼出し間隔 1000 ms（`hitomi.min_interval_ms`）・retries 3・backoff 2000 ms×試行（429/5xx のみ）。方針は `decisions/2026-11-01-hitomi-client-endpoints.md`。テストは mock（`Client.fetch` 注入） |

テストは外部 API を実際に呼ばない。
