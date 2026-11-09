# 2026-11-09 — disk cache cap は件数・既定 4000（R-15 決定点1 を supersede）

**Status**: 決定（ユーザー 2026-11-09 00:11 UTC「件数かつ4000」）。実装未着手（実装開始指示待ち）
**タスク記録**: `docs/ai/tasks/2026-11-09-settings-value-centralization.md`

## 決定

- 単位: **件数**（DB 行 `COUNT(1)`）。byte 計算を入れない。
- 既定値: `defaultCacheDiskMaxEntries` 8000 → **4000**（`app/main.go:157`）。

## 維持（既存実装の形どおり）

- kind 単位の cap（`kind IN ('full','thumbs')`、`app/main.go:525`）。件数判定は DB 行だけで、filesystem scan をしない（`app/main.go:521`、`app/viewer.go:598-611`）。
- 捨てる順は `created_at ASC`（作成順）。現在閲覧中の gallery の項目は捨てない（`app/viewer.go:618`）。
- TN（`thumbs`）も同一 cap で prune される（`recordAndPruneCacheEntry` → `pruneCacheEntries`、`app/viewer.go:652-659`）。TN byte を full actual pixels として数えない規則（`app/viewer.go:331`）は非接触。

## 未確定（4000 の適用範囲）

1. 4000 を per kind（full 4000 + thumbs 4000 = 最大 8000 行）にするか、合計 4000（kind 按分）にするか。既存実装は per kind。per kind のままなら full 層が memory cap（`viewer.max_object_urls` 4000）と同数になり、memory と disk の full 層の件数が揃う。
2. last access 記録（R-15 決定点 2）: mtime は読書で更新されず LU 順にならない。作成順（`created_at`）を LRU の順として認めるか、明示的な last access 列を置くか。単位決定だけでは解決しない。
3. 設定化するかどうか（設定化不可の型2 の判定）: 件数確定により「件数で設定化できる」が、保存済み优先判定と結合するため allowlist に入れるかは別途。

## 実装時に更新が必要

- `app/main_test.go:1584-1585`（"approved disk cap must be 8000 per kind"）→ 4000 へ re-anchor。
- `app/viewer.go:598` のコメント（既定 8000 件）→ 4000。
- `config.example.json` は設定化する決断をした場合のみ key を追加（現状 key なし）。

## 容量の算術（推定・実測ではない）

- full 1 件あたり平均 150–210 KB（完成フォルダの実ファイル 172,426 B・210,786 B、acquisition avif 129–136 KB のサンプルから）。4000 件 ≒ 600–840 MB。gallery 1 件 60 ページとすると ≒ 66 gallery。
- 8000 → 4000 の半減は、現在閲覧中 gallery 以外の古い full cache が早く捨てられることを意味する。再表示で remote 再取得（gate 消費・R-07）が増える。R-15 の「実測後に決定」の性質は残る（実測は未実施）。

## supersede する決定

- `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` の R-15 決定点 1「disk は byte 推奨」の点のみ supersede。同 decision の memory 側 4000 件、保存済み优先（memory 側のみ）、disk の gallery 単位古い順、通知 UI を置かない、TN を原寸画像の保存として数えない方針は維持。
