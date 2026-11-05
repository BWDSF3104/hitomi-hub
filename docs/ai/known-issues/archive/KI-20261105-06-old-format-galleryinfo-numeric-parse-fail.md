# 旧形式 live galleryinfo の数値フィールドで Go parse 失敗（card API error）

**Status**: 未解決（記録のみ・修復は別承認）

## 症状

- runtime 確認中、page 3（旧 gallery）の card API がすべて error を返した:
  - `json: cannot unmarshal number into Go struct field rawGallery.id of type string`（id が number の旧形式）
  - `json: cannot unmarshal number into Go struct field rawGallery.tags.0.female of type string`（tags 値が number）
- 実測 id: `296403`（tags 形式）・`293004`・`241311`・`238019`・`236142`（id 形式）。page 2 の `2299308` は string 形式で正常。
- card は error 本文を描画し、badge（表示済み）は map が補完されないため表示されない。`#lists-refresh`・`renderReadBadges()` 呼出しでも badge 空（map 補完経路が error card に反映されない）。

## jump 機能への影響

- hit 判定は `read_state`（DB）のみで成功（viewed 判定は galleryinfo fetch 不要）。移動・highlight は正常動作（`4-out-of-range-after.png` の top 93px）。
- badge 表示だけこの parse 失敗の影響で出ない。機能の合格条件（移動・highlight・status 行）には影響なし。

## 次の対応

- `parseGallery`（`app/hitomi.go`・`app/galleryinfocache.go`）の raw struct を number/string 両対応（flexible decode）にするか、live 応答形式バリエーションを test anchor に追加する。修复には別の実装承認が必要。テスト方針は外部 API 呼出し可（`docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md`）。
