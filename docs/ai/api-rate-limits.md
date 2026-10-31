# 外部 API レート制限

| 先 | 制限 | 運用 |
|----|------|------|
| VirusTotal public API | 4 req/min・500 req/day | hash-only 照会のみ（`GET /files/{sha256}` は内容を送信しない）。SHA256→verdict を `scan_cache` に永続化し、再ダウンロードはキャッシュヒットで 0 呼出し。アップロードは既定無効。テストは mock |
| hitomi.la | 公式 API なし（非公式 endpoint・Cloudflare） | 検索層実装時に呼出し間隔と再試行方針を decisions に記録 |

テストは外部 API を実際に呼ばない。
